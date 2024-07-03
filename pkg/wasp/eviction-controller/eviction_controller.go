package eviction_controller

import (
	"context"
	"encoding/json"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"kubevirt.io/wasp/pkg/client"
	"kubevirt.io/wasp/pkg/log"
	pod_evictor "kubevirt.io/wasp/pkg/wasp/pod-evictor"
	pod_filter "kubevirt.io/wasp/pkg/wasp/pod-filter"
	pod_ranker "kubevirt.io/wasp/pkg/wasp/pod-ranker"
	shortage_detector "kubevirt.io/wasp/pkg/wasp/shortage-detector"
	stats_collector "kubevirt.io/wasp/pkg/wasp/stats-collector"
	"time"
)

const (
	timeToWaitForCacheSync = 10 * time.Second
	WaspTaint              = "waspEvictionTaint"
)

type EvictionController struct {
	statsCollector   stats_collector.StatsCollector
	shortageDetector shortage_detector.ShortageDetector
	podRanker        pod_ranker.PodRanker
	podFilter        pod_filter.PodFilter
	podEvictor       pod_evictor.PodEvictor
	nodeName         string
	waspCli          client.WaspClient
	podInformer      cache.SharedIndexInformer
	nodeInformer     cache.SharedIndexInformer
	nodeLister       v1lister.NodeLister
	resyncPeriod     time.Duration
	stop             <-chan struct{}
}

func NewEvictionController(waspCli client.WaspClient,
	podInformer cache.SharedIndexInformer,
	nodeInformer cache.SharedIndexInformer,
	nodeName string,
	maxAverageSwapInPagesPerSecond float32,
	maxAverageSwapOutPagesPerSecond float32,
	maxMemoryOverCommitmentBytes resource.Quantity,
	AverageWindowSizeSeconds time.Duration,
	waspNs string,
	stop <-chan struct{}) *EvictionController {
	sc := stats_collector.NewStatsCollectorImpl()
	ctrl := &EvictionController{
		statsCollector:   sc,
		shortageDetector: shortage_detector.NewShortageDetectorImpl(sc, maxAverageSwapInPagesPerSecond, maxAverageSwapOutPagesPerSecond, maxMemoryOverCommitmentBytes.Value(), AverageWindowSizeSeconds),
		nodeName:         nodeName,
		waspCli:          waspCli,
		podEvictor:       pod_evictor.NewPodEvictorImpl(waspCli),
		podRanker:        pod_ranker.NewPodRankerImpl(),
		podFilter:        pod_filter.NewPodFilterImpl(waspNs),
		resyncPeriod:     metav1.Duration{Duration: 5 * time.Second}.Duration,
		podInformer:      podInformer,
		nodeInformer:     nodeInformer,
		nodeLister:       v1lister.NewNodeLister(nodeInformer.GetIndexer()),
		stop:             stop,
	}
	return ctrl
}

func (ctrl *EvictionController) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()
	log.Log.Infof("Starting ARQ controller")
	defer log.Log.Infof("Shutting ARQ Controller")

	go wait.Until(ctrl.handleMemorySwapEviction, ctrl.resyncPeriod, ctrl.stop)
	go wait.Until(ctrl.statsCollector.GatherStats, metav1.Duration{Duration: 1 * time.Second}.Duration, ctrl.stop)

	<-ctx.Done()
}

func (ctrl *EvictionController) gatherStatistics() (*v1.Node, error) {
	if !waitForSyncedStore(time.After(timeToWaitForCacheSync), ctrl.nodeInformer.HasSynced) {
		return nil, fmt.Errorf("nodes caches not synchronized")
	}
	return ctrl.nodeLister.Get(ctrl.nodeName)
}

func (ctrl *EvictionController) handleMemorySwapEviction() {
	shouldEvict := ctrl.shortageDetector.ShouldEvict()
	node, err := ctrl.getNode()
	if err != nil {
		log.Log.Infof(err.Error())
		return
	}
	evicting := nodeHasEvictionTaint(node)

	switch {
	case evicting && !shouldEvict:
		err := removeWaspEvictionTaint(ctrl.waspCli, node)
		if err != nil {
			log.Log.Infof(err.Error())
		}
	case !evicting && shouldEvict:
		err := addWaspEvictionTaint(ctrl.waspCli, node)
		if err != nil {
			log.Log.Infof(err.Error())
		}
		ctrl.statsCollector.FlushStats()
	}
	if !shouldEvict {
		return
	}
	pods, err := ctrl.listRunningPodsOnNode()
	if err != nil {
		log.Log.Infof(err.Error())
		return
	}

	rankedFilteredPods := ctrl.podRanker.RankPods(ctrl.podFilter.FilterPods(pods))

	if len(rankedFilteredPods) == 0 {
		log.Log.Infof("Wasp evictor doesn't have any pod to evict")
		return
	}
	for _, p := range rankedFilteredPods {
		log.Log.Infof("potenial pod: %v in ns: %v in node: %v", p.Name, p.Namespace, p.Spec.NodeName)
	}
	log.Log.Infof("will evict pod: %v  ns: %v", rankedFilteredPods[0].Name, rankedFilteredPods[0].Namespace)
	err = ctrl.podEvictor.EvictPod(&rankedFilteredPods[0])
	if err != nil {
		log.Log.Infof(err.Error())
	}
}

func nodeHasEvictionTaint(node *v1.Node) bool {
	// Check if the node has the specified taint
	for _, taint := range node.Spec.Taints {
		if taint.Key == WaspTaint && taint.Effect == v1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}

func addWaspEvictionTaint(waspCli client.WaspClient, node *v1.Node) error {
	taint := v1.Taint{
		Key:    WaspTaint,
		Effect: v1.TaintEffectNoSchedule,
	}

	taints := append(node.Spec.Taints, taint)

	taintsPatch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"taints": taints,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal taints patch: %v", err)
	}

	_, err = waspCli.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.StrategicMergePatchType, taintsPatch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node: %v", err)
	}

	return nil
}

func removeWaspEvictionTaint(waspCli client.WaspClient, node *v1.Node) error {
	var newTaints []v1.Taint
	for _, taint := range node.Spec.Taints {
		if taint.Key != WaspTaint {
			newTaints = append(newTaints, taint)
		}
	}

	taintsPatch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"taints": newTaints,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal taints patch: %v", err)
	}

	_, err = waspCli.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.StrategicMergePatchType, taintsPatch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch node: %v", err)
	}

	return nil
}

func waitForSyncedStore(timeout <-chan time.Time, informerSynced func() bool) bool {
	for !informerSynced() {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-timeout:
			return informerSynced()
		}
	}

	return true
}

func (ctrl *EvictionController) listRunningPodsOnNode() ([]v1.Pod, error) {
	handlerNodeSelector := fields.ParseSelectorOrDie("spec.nodeName=" + ctrl.nodeName)
	list, err := ctrl.waspCli.CoreV1().Pods(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		FieldSelector: handlerNodeSelector.String(),
	})
	if err != nil {
		return nil, err
	}
	var pods []v1.Pod

	for i := range list.Items {
		pod := list.Items[i]

		//  A pod with all containers terminated is not
		// considered alive
		allContainersTerminated := false
		if len(pod.Status.ContainerStatuses) > 0 {
			allContainersTerminated = true
			for _, status := range pod.Status.ContainerStatuses {
				if status.State.Terminated == nil {
					allContainersTerminated = false
					break
				}
			}
		}

		phase := pod.Status.Phase
		toAppendPod := !allContainersTerminated && phase != v1.PodFailed && phase != v1.PodSucceeded
		if toAppendPod {
			pods = append(pods, pod)
			continue
		}
	}
	return pods, nil
}

func (ctrl *EvictionController) getNode() (*v1.Node, error) {
	if !waitForSyncedStore(time.After(timeToWaitForCacheSync), ctrl.nodeInformer.HasSynced) {
		return nil, fmt.Errorf("nodes caches not synchronized")
	}
	return ctrl.nodeLister.Get(ctrl.nodeName)
}
