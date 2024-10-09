package eviction_controller

import (
	"context"
	"fmt"
	"github.com/openshift-virtualization/wasp-agent/pkg/client"
	"github.com/openshift-virtualization/wasp-agent/pkg/log"
	wasp_taints "github.com/openshift-virtualization/wasp-agent/pkg/taints"
	pod_evictor "github.com/openshift-virtualization/wasp-agent/pkg/wasp/pod-evictor"
	pod_filter "github.com/openshift-virtualization/wasp-agent/pkg/wasp/pod-filter"
	pod_ranker "github.com/openshift-virtualization/wasp-agent/pkg/wasp/pod-ranker"
	shortage_detector "github.com/openshift-virtualization/wasp-agent/pkg/wasp/shortage-detector"
	stats_collector "github.com/openshift-virtualization/wasp-agent/pkg/wasp/stats-collector"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"time"
)

const (
	timeToWaitForCacheSync = 10 * time.Second
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
	podStatsCollector stats_collector.PodStatsCollector,
	podInformer cache.SharedIndexInformer,
	nodeInformer cache.SharedIndexInformer,
	nodeName string,
	waspNs string,
	stop <-chan struct{},
	sd shortage_detector.ShortageDetector,
	sc stats_collector.StatsCollector) *EvictionController {

	ctrl := &EvictionController{
		statsCollector:   sc,
		shortageDetector: sd,
		nodeName:         nodeName,
		waspCli:          waspCli,
		podEvictor:       pod_evictor.NewPodEvictorImpl(waspCli),
		podRanker:        pod_ranker.NewPodRankerImpl(podStatsCollector),
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
	log.Log.Infof("Starting eviction controller")
	defer log.Log.Infof("Shutting eviction Controller")

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
	shouldEvict, err := ctrl.shortageDetector.ShouldEvict()
	if err != nil {
		log.Log.Infof(err.Error())
		return
	}
	node, err := ctrl.getNode()
	if err != nil {
		log.Log.Infof(err.Error())
		return
	}
	evicting := wasp_taints.NodeHasEvictionTaint(node)

	switch {
	case evicting && !shouldEvict:
		err := wasp_taints.RemoveWaspEvictionTaint(ctrl.waspCli, node)
		if err != nil {
			log.Log.Infof(err.Error())
		}
	case !evicting && shouldEvict:
		err := wasp_taints.AddWaspEvictionTaint(ctrl.waspCli, node)
		if err != nil {
			log.Log.Infof(err.Error())
		}
	}
	if !shouldEvict {
		return
	}
	pods, err := ctrl.listRunningPodsOnNode()
	if err != nil {
		log.Log.Infof(err.Error())
		return
	}
	filteredPods := ctrl.podFilter.FilterPods(pods)
	if len(filteredPods) == 0 {
		log.Log.Infof("Wasp evictor doesn't have any pod to evict")
		return
	}
	err = ctrl.podRanker.RankPods(filteredPods)
	if err != nil {
		log.Log.Infof(err.Error())
	}

	var evictionDetails string
	for _, p := range filteredPods {
		evictionDetails += fmt.Sprintf("pod: %v in ns: %v in node: %v\n", p.Name, p.Namespace, p.Spec.NodeName)
	}
	log.Log.Infof("Pods in eviction order:\n%vWill evict pod: %v in ns: %v", evictionDetails, filteredPods[0].Name, filteredPods[0].Namespace)
	err = ctrl.podEvictor.EvictPod(filteredPods[0])
	if err != nil {
		log.Log.Infof(err.Error())
		return
	}
	ctrl.statsCollector.FlushStats()
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

func (ctrl *EvictionController) listRunningPodsOnNode() ([]*v1.Pod, error) {
	handlerNodeSelector := fields.ParseSelectorOrDie("spec.nodeName=" + ctrl.nodeName)
	list, err := ctrl.waspCli.CoreV1().Pods(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		FieldSelector: handlerNodeSelector.String(),
	})
	if err != nil {
		return nil, err
	}
	var pods []*v1.Pod

	for i := range list.Items {
		pod := &list.Items[i]

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
