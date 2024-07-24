package limited_swap_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/openshift-virtualization/wasp-agent/pkg/client"
	"github.com/openshift-virtualization/wasp-agent/pkg/log"
	"github.com/shirou/gopsutil/mem"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	v1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	kubeapiqos "k8s.io/kubernetes/pkg/apis/core/v1/helper/qos"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type enqueueState string

const (
	Immediate      enqueueState = "Immediate"
	Forget         enqueueState = "Forget"
	BackOff        enqueueState = "BackOff"
	cgroupPathBase              = "/host/sys/fs/cgroup"
)

type LimitedSwapManager struct {
	podInformer    cache.SharedIndexInformer
	podLister      v1lister.PodLister
	podQueue       workqueue.RateLimitingInterface
	waspCli        client.WaspClient
	swapCapacity   uint64
	memoryCapacity uint64
	nodeName       string
	stop           <-chan struct{}
}

func NewLimitedSwapManager(waspCli client.WaspClient,
	podInformer cache.SharedIndexInformer,
	nodeName string,
	stop <-chan struct{},
) *LimitedSwapManager {
	swap, err := mem.SwapMemory()
	if err != nil {
		panic(fmt.Sprintf("Error fetching swap memory: %v", err.Error()))
	}
	virtualMem, err := mem.VirtualMemory()
	if err != nil {
		panic(fmt.Sprintf("Error fetching virtualMem memory: %v", err))
	}
	cgroupManager := LimitedSwapManager{
		podInformer:    podInformer,
		podLister:      v1lister.NewPodLister(podInformer.GetIndexer()),
		waspCli:        waspCli,
		nodeName:       nodeName,
		podQueue:       workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{Name: "pdo-queue-for-cgroup-manager"}),
		stop:           stop,
		swapCapacity:   swap.Total,
		memoryCapacity: virtualMem.Total,
	}

	_, err = cgroupManager.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: cgroupManager.updatePod,
		DeleteFunc: cgroupManager.createPod,
	})
	if err != nil {
		panic("something is wrong")
	}
	return &cgroupManager
}

func (lsm *LimitedSwapManager) updatePod(old, curr interface{}) {
	curPod := curr.(*v1.Pod)
	oldPod := old.(*v1.Pod)
	if oldPod.Spec.NodeName != lsm.nodeName && curPod.Spec.NodeName == lsm.nodeName {
		key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(curPod)
		if err != nil {
			log.Log.Errorf("LimitedSwapManager: %v", err)
			return
		}
		lsm.podQueue.Add(key)
	}

	return
}

func (lsm *LimitedSwapManager) createPod(podObj interface{}) {
	pod := podObj.(*v1.Pod)
	if pod.Spec.NodeName == lsm.nodeName {
		key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod)
		if err != nil {
			log.Log.Errorf("LimitedSwapManager: %v", err)
			return
		}
		lsm.podQueue.Add(key)
	}
	return
}
func (lsm *LimitedSwapManager) runWorker() {
	for lsm.Execute() {
	}
}

func (lsm *LimitedSwapManager) enqueueAllPods() {
	pods, err := lsm.podLister.List(labels.Everything())
	if err != nil {
		log.Log.Errorf(err.Error())
		return
	}
	for _, p := range pods {
		if p.Spec.NodeName == lsm.nodeName {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(p)
			if err != nil {
				log.Log.Errorf("LimitedSwapManager: %v", err)
				return
			}
			lsm.podQueue.Add(key)
		}
	}
}

func (lsm *LimitedSwapManager) Execute() bool {
	key, quit := lsm.podQueue.Get()
	if quit {
		return false
	}
	defer lsm.podQueue.Done(key)

	err, enqueueState := lsm.execute(key.(string))
	if err != nil {
		log.Log.Infof(fmt.Sprintf("RQController: Error with key: %v err: %v", key, err))
	}
	switch enqueueState {
	case BackOff:
		lsm.podQueue.AddRateLimited(key)
	case Forget:
		lsm.podQueue.Forget(key)
	case Immediate:
		lsm.podQueue.Add(key)
	}

	return true
}

func (lsm *LimitedSwapManager) Run(threadiness int) {
	defer utilruntime.HandleCrash()
	log.Log.Infof("Starting LimitedSwapManager")
	defer log.Log.Infof("Shutting down LimitedSwapManager")
	defer lsm.podQueue.ShutDown()

	for i := 0; i < threadiness; i++ {
		go wait.Until(lsm.runWorker, time.Second, lsm.stop)
		go wait.Until(lsm.enqueueAllPods, metav1.Duration{Duration: 20 * time.Second}.Duration, lsm.stop)
	}

	<-lsm.stop
}

func (lsm *LimitedSwapManager) execute(key string) (error, enqueueState) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	pod, err := lsm.podLister.Pods(namespace).Get(name)
	if kapierrors.IsNotFound(err) {
		return nil, Forget
	} else if err != nil {
		log.Log.Errorf(err.Error())
		return err, BackOff
	}
	excludedPrefixesForNamespaces := []string{"openshift", "kube-system"}
	podQos := kubeapiqos.GetPodQOS(pod)
	setAllContainersSwapToZero := podQos != v1.PodQOSBurstable || kubelettypes.IsCriticalPod(pod) || hasExcludedPrefix(pod.Namespace, excludedPrefixesForNamespaces)

	for _, container := range append(pod.Spec.Containers, pod.Spec.InitContainers...) {
		containerState, exist := getContainerState(pod, container)
		if !exist || containerState.Waiting != nil || containerState.Running == nil {
			lsm.podQueue.AddRateLimited(key)
			continue
		} else if containerState.Terminated != nil {
			continue
		}

		containerUID, err := getContainerUID(pod, container)
		if err != nil {
			lsm.podQueue.AddRateLimited(key)
			continue
		}

		dirPath, err := getContainerCgroupPath(containerUID)
		if err != nil {
			log.Log.Errorf(err.Error())
			lsm.podQueue.AddRateLimited(key)
		}
		containerDoesNotRequestMemory := container.Resources.Requests.Memory().IsZero() && container.Resources.Limits.Memory().IsZero()
		memoryRequestEqualsToLimit := container.Resources.Requests.Memory().Cmp(*container.Resources.Limits.Memory()) == 0
		if containerDoesNotRequestMemory || memoryRequestEqualsToLimit || setAllContainersSwapToZero {
			err := setSwapLimit(dirPath, 0)
			if err != nil {
				log.Log.Infof("LimitSwapManager: couldn't set swap limit: %v", err.Error())
				lsm.podQueue.AddRateLimited(key)
			}
			continue
		}
		containerMemoryRequest := container.Resources.Requests.Memory()
		swapLimit := calcSwapForBurstablePods(containerMemoryRequest.Value(), int64(lsm.memoryCapacity), int64(lsm.swapCapacity))
		err = setSwapLimit(dirPath, swapLimit)
		if err != nil {
			log.Log.Infof("LimitSwapManager: couldn't set swap limit: %v", err.Error())
			lsm.podQueue.AddRateLimited(key)
			continue
		}
	}

	return nil, Forget
}

func calcSwapForBurstablePods(containerMemoryRequest, nodeTotalMemory, totalPodsSwapAvailable int64) int64 {
	containerMemoryProportion := float64(containerMemoryRequest) / float64(nodeTotalMemory)
	swapAllocation := containerMemoryProportion * float64(totalPodsSwapAvailable)

	return int64(swapAllocation)
}

func setSwapLimit(dirPath string, swapLimit int64) error {
	err := cgroups.WriteFile(dirPath, "memory.swap.max", strconv.FormatInt(swapLimit, 10))
	return err
}

func getContainerStatusResponse(containerUID string) (*runtimeapi.ContainerStatusResponse, error) {
	// Set up the gRPC connection to the CRI runtime
	conn, err := grpc.Dial("unix:///var/run/crio/crio.sock", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Create a RuntimeServiceClient
	client := runtimeapi.NewRuntimeServiceClient(conn)

	// Call the ContainerStatus API to get container information
	request := &runtimeapi.ContainerStatusRequest{ContainerId: containerUID, Verbose: true}
	response, err := client.ContainerStatus(context.Background(), request)

	return response, err
}

type Data struct {
	Pid int `json:"pid"`
}

func getContainerCgroupPath(containerUID string) (string, error) {
	containerStatusResponse, err := getContainerStatusResponse(containerUID)
	if err != nil {
		return "", err
	}
	if containerStatusResponse.Info == nil {
		return "", fmt.Errorf("Failed to get container status info")
	}

	var data Data
	err = json.Unmarshal([]byte(containerStatusResponse.Info["info"]), &data)
	if err != nil {
		return "", err
	}
	if data.Pid == 0 {
		return "", fmt.Errorf("PID not found in container info")
	}

	return getCgroupPath(strconv.Itoa(data.Pid))
}

// ReadCgroupFile reads the contents of the cgroup file at the given path.
func getCgroupPath(pid string) (string, error) {
	procCgroupBasePath := filepath.Join("/host/proc", pid, "cgroup")
	controllerPaths, err := cgroups.ParseCgroupFile(procCgroupBasePath)
	path, ok := controllerPaths[""]
	if err != nil {
		return "", err
	} else if !ok {
		return "", fmt.Errorf("could not get cgroup path")
	}
	return filepath.Join(cgroupPathBase, path), nil
}

func getContainerUID(pod *v1.Pod, container v1.Container) (string, error) {
	prefix := "cri-o://"
	for _, conatinerStatus := range pod.Status.ContainerStatuses {
		if conatinerStatus.Name == container.Name {
			return strings.TrimPrefix(conatinerStatus.ContainerID, prefix), nil
		}
	}
	for _, conatinerStatus := range pod.Status.InitContainerStatuses {
		if conatinerStatus.Name == container.Name {
			return strings.TrimPrefix(conatinerStatus.ContainerID, prefix), nil
		}
	}
	return "", fmt.Errorf("cannot find ContainerUID PodName: %v containerName: %v", pod.Name, container.Name)
}

func getContainerState(pod *v1.Pod, container v1.Container) (v1.ContainerState, bool) {
	for _, conatinerStatus := range pod.Status.ContainerStatuses {
		if conatinerStatus.Name == container.Name {
			return conatinerStatus.State, true
		}
	}
	for _, conatinerStatus := range pod.Status.InitContainerStatuses {
		if conatinerStatus.Name == container.Name {
			return conatinerStatus.State, true
		}
	}

	return v1.ContainerState{}, false
}

// hasExcludedPrefix checks if the given namespace name starts with any of the excluded prefixes
func hasExcludedPrefix(namespace string, excludedPrefixes []string) bool {
	for _, prefix := range excludedPrefixes {
		if strings.HasPrefix(namespace, prefix) {
			return true
		}
	}
	return false
}
