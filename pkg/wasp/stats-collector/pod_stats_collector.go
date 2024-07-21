package stats_collector

import (
	"fmt"
	cadvisorapiv2 "github.com/google/cadvisor/info/v2"
	"github.com/openshift-virtualization/wasp-agent/pkg/cadvisor"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	statsapi "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
	"path/filepath"
	"sort"
	"strings"
)

// defaultNetworkInterfaceName is used for collectng network stats.
// This logic relies on knowledge of the container runtime implementation and
// is not reliable.
const defaultNetworkInterfaceName = "eth0"

type ContainerSummary struct {
	MemorySwapMaxBytes     uint64
	MemorySwapCurrentBytes uint64
	MemoryWorkingSetBytes  uint64
}

type PodSummary struct {
	UID                    types.UID
	CgroupKey              string
	MemoryWorkingSetBytes  uint64
	MemorySwapCurrentBytes uint64
	Containers             map[string]ContainerSummary
}

// PodReference contains enough information to locate the referenced pod.
type PodReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`
}

type PodStatsCollector interface {
	Init() error
	GetPodSummary(pod *v1.Pod) (PodSummary, error)
	ListPodsSummary() ([]PodSummary, error)
}

type PodStatsCollectorImpl struct {
	cadviorInterface cadvisor.Interface
	PodSummary       map[string]*PodSummary
}

func (psc *PodStatsCollectorImpl) GetPodSummary(pod *v1.Pod) (PodSummary, error) {
	var summary PodSummary
	traslator := PodToCgroupV2Translate{
		Pod: pod,
	}
	podKey, err := traslator.PodAbsCgroupPath()
	if err != nil {
		return PodSummary{}, err
	}
	summary.CgroupKey = podKey
	podCinfo, err := psc.cadviorInterface.ContainerInfoV2(podKey, cadvisorapiv2.RequestOptions{
		IdType:    cadvisorapiv2.TypeName,
		Count:     2, // 2 samples are needed to compute "instantaneous" CPU
		Recursive: true,
	})
	if err != nil {
		return PodSummary{}, err
	}

	filteredInfos, allInfos := filterTerminatedContainerInfoAndAssembleByPodCgroupKey(podCinfo)
	podInfo := getCadvisorPodInfoFromPodUID(pod.UID, allInfos)
	if podInfo == nil {
		return PodSummary{}, fmt.Errorf("could not find pod info in cadvisor")
	}

	_, memory := cadvisorInfoToCPUandMemoryStats(podInfo)
	summary.MemoryWorkingSetBytes = *memory.WorkingSetBytes
	swapStats := cadvisorInfoToSwapStats(podInfo)
	summary.MemorySwapCurrentBytes = *swapStats.SwapUsageBytes

	summary.Containers = make(map[string]ContainerSummary, len(filteredInfos))
	for _, cinfo := range filteredInfos {
		containerName := GetContainerName(cinfo.Spec.Labels)
		if containerName == PodInfraContainerName {
			continue
		}
		containerStats := cadvisorInfoToContainerStats(containerName, &cinfo, nil, nil)
		summary.Containers[containerName] = ContainerSummary{
			MemorySwapMaxBytes:    cinfo.Spec.Memory.SwapLimit,
			MemoryWorkingSetBytes: *containerStats.Memory.WorkingSetBytes,
		}
	}

	return summary, nil
}

func NewPodSummaryCollector() PodStatsCollector {
	return &PodStatsCollectorImpl{}
}

func (psc *PodStatsCollectorImpl) Init() error {
	cadvisorConfig := cadvisor.NewCAdvisorConfigForCRIO()
	cadviorInterface, err := cadvisor.New(
		cadvisorConfig.ImageFsInfoProvider,
		cadvisorConfig.RootPath,
		cadvisorConfig.CgroupRoots,
		cadvisorConfig.UsingLegacyStats,
		cadvisorConfig.LocalStorageCapacityIsolation)

	if err != nil {
		return err
	}

	psc.cadviorInterface = cadviorInterface
	if err = psc.cadviorInterface.Start(); err != nil {
		return err
	}
	psc.PodSummary = make(map[string]*PodSummary)
	return nil
}

func (psc *PodStatsCollectorImpl) ListPodsSummary() ([]PodSummary, error) {
	infos, err := getCadvisorContainerInfo(psc.cadviorInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to get container info from cadvisor: %v", err)
	}
	filteredInfos, allInfos := filterTerminatedContainerInfoAndAssembleByPodCgroupKey(infos)
	// Map each container to a pod and update the PodStats with container data.
	podToStats := map[statsapi.PodReference]*statsapi.PodStats{}
	for key, cinfo := range filteredInfos {
		// On systemd using devicemapper each mount into the container has an
		// associated cgroup. We ignore them to ensure we do not get duplicate
		// entries in our summary. For details on .mount units:
		// http://man7.org/linux/man-pages/man5/systemd.mount.5.html
		if strings.HasSuffix(key, ".mount") {
			continue
		}
		// Build the Pod key if this container is managed by a Pod
		if !isPodManagedContainer(&cinfo) {
			continue
		}
		ref := buildPodRef(cinfo.Spec.Labels)

		// Lookup the PodStats for the pod using the PodRef. If none exists,
		// initialize a new entry.
		podStats, found := podToStats[ref]
		if !found {
			podStats = &statsapi.PodStats{PodRef: ref}
			podToStats[ref] = podStats
		}

		podSum, found := psc.PodSummary[ref.UID]
		if !found {
			podSum = &PodSummary{}
			podSum.UID = types.UID(ref.UID)
			podSum.Containers = make(map[string]ContainerSummary)
			psc.PodSummary[ref.UID] = podSum
		}

		// Update the PodStats entry with the stats from the container by
		// adding it to podStats.Containers.
		containerName := GetContainerName(cinfo.Spec.Labels)
		if containerName == PodInfraContainerName {
			// Special case for infrastructure container which is hidden from
			// the user and has network stats.
			podStats.Network = cadvisorInfoToNetworkStats(&cinfo)
		} else {
			containerStat := cadvisorInfoToContainerStats(containerName, &cinfo, nil, nil)
			containerStat.Logs = nil
			podStats.Containers = append(podStats.Containers, *containerStat)
			podSum.Containers[containerName] = ContainerSummary{
				MemorySwapMaxBytes:     cinfo.Spec.Memory.SwapLimit,
				MemoryWorkingSetBytes:  *containerStat.Memory.WorkingSetBytes,
				MemorySwapCurrentBytes: *containerStat.Swap.SwapUsageBytes,
			}
		}
	}

	// Add each PodStats to the result.
	podSummaryList := make([]PodSummary, 0, len(psc.PodSummary))
	for _, podStats := range podToStats {
		podUID := types.UID(podStats.PodRef.UID)
		// Lookup the pod-level cgroup's CPU and memory stats
		podInfo := getCadvisorPodInfoFromPodUID(podUID, allInfos)
		if podInfo != nil {
			cpu, memory := cadvisorInfoToCPUandMemoryStats(podInfo)
			podStats.CPU = cpu
			podStats.Memory = memory
			podStats.Swap = cadvisorInfoToSwapStats(podInfo)
			podStats.ProcessStats = cadvisorInfoToProcessStats(podInfo)

			psc.PodSummary[string(podUID)].MemoryWorkingSetBytes = *podStats.Memory.WorkingSetBytes
			psc.PodSummary[string(podUID)].MemorySwapCurrentBytes = *podStats.Swap.SwapUsageBytes
		}
		podSummaryList = append(podSummaryList, *psc.PodSummary[string(podUID)])
	}
	return podSummaryList, nil
}

// buildPodRef returns a PodReference that identifies the Pod managing cinfo
func buildPodRef(containerLabels map[string]string) statsapi.PodReference {
	podName := GetPodName(containerLabels)
	podNamespace := GetPodNamespace(containerLabels)
	podUID := GetPodUID(containerLabels)
	return statsapi.PodReference{Name: podName, Namespace: podNamespace, UID: podUID}
}

// isPodManagedContainer returns true if the cinfo container is managed by a Pod
func isPodManagedContainer(cinfo *cadvisorapiv2.ContainerInfo) bool {
	podName := GetPodName(cinfo.Spec.Labels)
	podNamespace := GetPodNamespace(cinfo.Spec.Labels)
	managed := podName != "" && podNamespace != ""
	if !managed && podName != podNamespace {
		klog.InfoS(
			"Expect container to have either both podName and podNamespace labels, or neither",
			"podNameLabel", podName, "podNamespaceLabel", podNamespace)
	}
	return managed
}

// getCadvisorPodInfoFromPodUID returns a pod cgroup information by matching the podUID with its CgroupName identifier base name
func getCadvisorPodInfoFromPodUID(podUID types.UID, infos map[string]cadvisorapiv2.ContainerInfo) *cadvisorapiv2.ContainerInfo {
	if info, found := infos[GetPodCgroupNameSuffix(podUID)]; found {
		return &info
	}
	return nil
}

// filterTerminatedContainerInfoAndAssembleByPodCgroupKey returns the specified containerInfo but with
// the stats of the terminated containers removed and all containerInfos assembled by pod cgroup key.
// the first return map is container cgroup name <-> ContainerInfo and
// the second return map is pod cgroup key <-> ContainerInfo.
// A ContainerInfo is considered to be of a terminated container if it has an
// older CreationTime and zero CPU instantaneous and memory RSS usage.
func filterTerminatedContainerInfoAndAssembleByPodCgroupKey(containerInfo map[string]cadvisorapiv2.ContainerInfo) (map[string]cadvisorapiv2.ContainerInfo, map[string]cadvisorapiv2.ContainerInfo) {
	cinfoMap := make(map[containerID][]containerInfoWithCgroup)
	cinfosByPodCgroupKey := make(map[string]cadvisorapiv2.ContainerInfo)
	for key, cinfo := range containerInfo {
		var podCgroupKey string
		if IsSystemdStyleName(key) {
			// Convert to internal cgroup name and take the last component only.
			internalCgroupName := ParseSystemdToCgroupName(key)
			podCgroupKey = internalCgroupName[len(internalCgroupName)-1]
		} else {
			// Take last component only.
			podCgroupKey = filepath.Base(key)
		}
		cinfosByPodCgroupKey[podCgroupKey] = cinfo
		if !isPodManagedContainer(&cinfo) {
			continue
		}
		cinfoID := containerID{
			podRef:        buildPodRef(cinfo.Spec.Labels),
			containerName: GetContainerName(cinfo.Spec.Labels),
		}
		cinfoMap[cinfoID] = append(cinfoMap[cinfoID], containerInfoWithCgroup{
			cinfo:  cinfo,
			cgroup: key,
		})
	}
	result := make(map[string]cadvisorapiv2.ContainerInfo)
	for _, refs := range cinfoMap {
		if len(refs) == 1 {
			// ContainerInfo with no CPU/memory/network usage for uncleaned cgroups of
			// already terminated containers, which should not be shown in the results.
			if !isContainerTerminated(&refs[0].cinfo) {
				result[refs[0].cgroup] = refs[0].cinfo
			}
			continue
		}
		sort.Sort(ByCreationTime(refs))
		for i := len(refs) - 1; i >= 0; i-- {
			if hasMemoryAndCPUInstUsage(&refs[i].cinfo) {
				result[refs[i].cgroup] = refs[i].cinfo
				break
			}
		}
	}
	return result, cinfosByPodCgroupKey
}

// ByCreationTime implements sort.Interface for []containerInfoWithCgroup based
// on the cinfo.Spec.CreationTime field.
type ByCreationTime []containerInfoWithCgroup

func (a ByCreationTime) Len() int      { return len(a) }
func (a ByCreationTime) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByCreationTime) Less(i, j int) bool {
	if a[i].cinfo.Spec.CreationTime.Equal(a[j].cinfo.Spec.CreationTime) {
		// There shouldn't be two containers with the same name and/or the same
		// creation time. However, to make the logic here robust, we break the
		// tie by moving the one without CPU instantaneous or memory RSS usage
		// to the beginning.
		return hasMemoryAndCPUInstUsage(&a[j].cinfo)
	}
	return a[i].cinfo.Spec.CreationTime.Before(a[j].cinfo.Spec.CreationTime)
}

// containerID is the identity of a container in a pod.
type containerID struct {
	podRef        statsapi.PodReference
	containerName string
}

// containerInfoWithCgroup contains the ContainerInfo and its cgroup name.
type containerInfoWithCgroup struct {
	cinfo  cadvisorapiv2.ContainerInfo
	cgroup string
}

// hasMemoryAndCPUInstUsage returns true if the specified container info has
// both non-zero CPU instantaneous usage and non-zero memory RSS usage, and
// false otherwise.
func hasMemoryAndCPUInstUsage(info *cadvisorapiv2.ContainerInfo) bool {
	if !info.Spec.HasCpu || !info.Spec.HasMemory {
		return false
	}
	cstat, found := latestContainerStats(info)
	if !found {
		return false
	}
	if cstat.CpuInst == nil {
		return false
	}
	return cstat.CpuInst.Usage.Total != 0 && cstat.Memory.RSS != 0
}

// isContainerTerminated returns true if the specified container meet one of the following conditions
// 1. info.spec both cpu memory and network are false conditions
// 2. info.Stats both network and cpu or memory are nil
// 3. both zero CPU instantaneous usage zero memory RSS usage and zero network usage,
// and false otherwise.
func isContainerTerminated(info *cadvisorapiv2.ContainerInfo) bool {
	if !info.Spec.HasCpu && !info.Spec.HasMemory && !info.Spec.HasNetwork {
		return true
	}
	cstat, found := latestContainerStats(info)
	if !found {
		return true
	}
	if cstat.Network != nil {
		iStats := cadvisorInfoToNetworkStats(info)
		if iStats != nil {
			for _, iStat := range iStats.Interfaces {
				if *iStat.RxErrors != 0 || *iStat.TxErrors != 0 || *iStat.RxBytes != 0 || *iStat.TxBytes != 0 {
					return false
				}
			}
		}
	}
	if cstat.CpuInst == nil || cstat.Memory == nil {
		return true
	}
	return cstat.CpuInst.Usage.Total == 0 && cstat.Memory.RSS == 0
}

func getCadvisorContainerInfo(ca cadvisor.Interface) (map[string]cadvisorapiv2.ContainerInfo, error) {
	infos, err := ca.ContainerInfoV2("/", cadvisorapiv2.RequestOptions{
		IdType:    cadvisorapiv2.TypeName,
		Count:     2, // 2 samples are needed to compute "instantaneous" CPU
		Recursive: true,
	})
	if err != nil {
		if _, ok := infos["/"]; ok {
			// If the failure is partial, log it and return a best-effort
			// response.
			klog.ErrorS(err, "Partial failure issuing cadvisor.ContainerInfoV2")
		} else {
			return nil, fmt.Errorf("failed to get root cgroup stats: %v", err)
		}
	}
	return infos, nil
}

// cadvisorInfoToContainerStats returns the statsapi.ContainerStats converted
// from the container and filesystem info.
func cadvisorInfoToContainerStats(name string, info *cadvisorapiv2.ContainerInfo, rootFs, imageFs *cadvisorapiv2.FsInfo) *statsapi.ContainerStats {
	result := &statsapi.ContainerStats{
		StartTime: metav1.NewTime(info.Spec.CreationTime),
		Name:      name,
	}
	_, found := latestContainerStats(info)
	if !found {
		return result
	}

	cpu, memory := cadvisorInfoToCPUandMemoryStats(info)
	result.CPU = cpu
	result.Memory = memory
	result.Swap = cadvisorInfoToSwapStats(info)

	return result
}

// latestContainerStats returns the latest container stats from cadvisor, or nil if none exist
func latestContainerStats(info *cadvisorapiv2.ContainerInfo) (*cadvisorapiv2.ContainerStats, bool) {
	stats := info.Stats
	if len(stats) < 1 {
		return nil, false
	}
	latest := stats[len(stats)-1]
	if latest == nil {
		return nil, false
	}
	return latest, true
}

func cadvisorInfoToCPUandMemoryStats(info *cadvisorapiv2.ContainerInfo) (*statsapi.CPUStats, *statsapi.MemoryStats) {
	cstat, found := latestContainerStats(info)
	if !found {
		return nil, nil
	}
	var cpuStats *statsapi.CPUStats
	var memoryStats *statsapi.MemoryStats
	cpuStats = &statsapi.CPUStats{
		Time:                 metav1.NewTime(cstat.Timestamp),
		UsageNanoCores:       uint64Ptr(0),
		UsageCoreNanoSeconds: uint64Ptr(0),
	}
	if info.Spec.HasCpu {
		if cstat.CpuInst != nil {
			cpuStats.UsageNanoCores = &cstat.CpuInst.Usage.Total
		}
		if cstat.Cpu != nil {
			cpuStats.UsageCoreNanoSeconds = &cstat.Cpu.Usage.Total
		}
	}
	if info.Spec.HasMemory && cstat.Memory != nil {
		pageFaults := cstat.Memory.ContainerData.Pgfault
		majorPageFaults := cstat.Memory.ContainerData.Pgmajfault
		memoryStats = &statsapi.MemoryStats{
			Time:            metav1.NewTime(cstat.Timestamp),
			UsageBytes:      &cstat.Memory.Usage,
			WorkingSetBytes: &cstat.Memory.WorkingSet,
			RSSBytes:        &cstat.Memory.RSS,
			PageFaults:      &pageFaults,
			MajorPageFaults: &majorPageFaults,
		}
		// availableBytes = memory limit (if known) - workingset
		if !isMemoryUnlimited(info.Spec.Memory.Limit) {
			availableBytes := info.Spec.Memory.Limit - cstat.Memory.WorkingSet
			memoryStats.AvailableBytes = &availableBytes
		}
	} else {
		memoryStats = &statsapi.MemoryStats{
			Time:            metav1.NewTime(cstat.Timestamp),
			WorkingSetBytes: uint64Ptr(0),
		}
	}
	return cpuStats, memoryStats
}

func isMemoryUnlimited(v uint64) bool {
	// Size after which we consider memory to be "unlimited". This is not
	// MaxInt64 due to rounding by the kernel.
	// TODO: cadvisor should export this https://github.com/google/cadvisor/blob/master/metrics/prometheus.go#L596
	const maxMemorySize = uint64(1 << 62)

	return v > maxMemorySize
}

func uint64Ptr(i uint64) *uint64 {
	return &i
}

func cadvisorInfoToSwapStats(info *cadvisorapiv2.ContainerInfo) *statsapi.SwapStats {
	cstat, found := latestContainerStats(info)
	if !found {
		return nil
	}

	var swapStats *statsapi.SwapStats

	if info.Spec.HasMemory && cstat.Memory != nil {
		swapStats = &statsapi.SwapStats{
			Time:           metav1.NewTime(cstat.Timestamp),
			SwapUsageBytes: &cstat.Memory.Swap,
		}

		if !isMemoryUnlimited(info.Spec.Memory.SwapLimit) {
			swapAvailableBytes := info.Spec.Memory.SwapLimit - cstat.Memory.Swap
			swapStats.SwapAvailableBytes = &swapAvailableBytes
		}
	}

	return swapStats
}

func cadvisorInfoToProcessStats(info *cadvisorapiv2.ContainerInfo) *statsapi.ProcessStats {
	cstat, found := latestContainerStats(info)
	if !found || cstat.Processes == nil {
		return nil
	}
	num := cstat.Processes.ProcessCount
	return &statsapi.ProcessStats{ProcessCount: uint64Ptr(num)}
}

// cadvisorInfoToNetworkStats returns the statsapi.NetworkStats converted from
// the container info from cadvisor.
func cadvisorInfoToNetworkStats(info *cadvisorapiv2.ContainerInfo) *statsapi.NetworkStats {
	if !info.Spec.HasNetwork {
		return nil
	}
	cstat, found := latestContainerStats(info)
	if !found {
		return nil
	}

	if cstat.Network == nil {
		return nil
	}

	iStats := statsapi.NetworkStats{
		Time: metav1.NewTime(cstat.Timestamp),
	}

	for i := range cstat.Network.Interfaces {
		inter := cstat.Network.Interfaces[i]
		iStat := statsapi.InterfaceStats{
			Name:     inter.Name,
			RxBytes:  &inter.RxBytes,
			RxErrors: &inter.RxErrors,
			TxBytes:  &inter.TxBytes,
			TxErrors: &inter.TxErrors,
		}

		if inter.Name == defaultNetworkInterfaceName {
			iStats.InterfaceStats = iStat
		}

		iStats.Interfaces = append(iStats.Interfaces, iStat)
	}

	return &iStats
}
