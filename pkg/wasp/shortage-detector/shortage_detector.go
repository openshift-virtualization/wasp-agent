package shortage_detector

import (
	"fmt"
	"github.com/openshift-virtualization/wasp-agent/pkg/log"
	stats_collector "github.com/openshift-virtualization/wasp-agent/pkg/wasp/stats-collector"
	"time"
)

// ShortageDetector is an interface for shortage detection
type ShortageDetector interface {
	ShouldEvict() (bool, error)
}

type ShortageDetectorImpl struct {
	sc                              stats_collector.StatsCollector
	psc                             stats_collector.PodStatsCollector
	maxAverageSwapInPagesPerSecond  float32
	maxAverageSwapOutPagesPerSecond float32
	swapUtilizationThresholdFactor  float64
	AverageWindowSizeSeconds        time.Duration
}

func NewShortageDetectorImpl(sc stats_collector.StatsCollector,
	psc stats_collector.PodStatsCollector,
	maxAverageSwapInPagesPerSecond,
	maxAverageSwapOutPagesPerSecond float32,
	swapUtilizationThresholdFactor float64,
	AverageWindowSizeSeconds time.Duration) *ShortageDetectorImpl {
	return &ShortageDetectorImpl{
		sc:                              sc,
		psc:                             psc,
		maxAverageSwapInPagesPerSecond:  maxAverageSwapInPagesPerSecond,
		maxAverageSwapOutPagesPerSecond: maxAverageSwapOutPagesPerSecond,
		AverageWindowSizeSeconds:        AverageWindowSizeSeconds,
		swapUtilizationThresholdFactor:  swapUtilizationThresholdFactor,
	}
}

func (sdi *ShortageDetectorImpl) ShouldEvict() (bool, error) {
	stats := sdi.sc.GetStatsList()
	if len(stats) < 2 {
		return false, fmt.Errorf("not enough Stats to detect shortage")
	}

	// Find the second newest Stats object after the first one with at least AverageWindowSizeSeconds difference
	firstStat := stats[0]
	var secondNewest *stats_collector.Stats
	minTimeInterval := time.Second * 5
	for i := 1; i < len(stats); i++ {
		if firstStat.Time.Sub(stats[i].Time) >= minTimeInterval {
			secondNewest = &stats[i]
		}
		if firstStat.Time.Sub(stats[i].Time) >= sdi.AverageWindowSizeSeconds {
			break
		}
	}

	if secondNewest == nil {
		return false, fmt.Errorf("not enough Stats to detect shortage")
	}

	// Calculate time difference in seconds
	timeDiffSeconds := float32(firstStat.Time.Sub(secondNewest.Time).Seconds())

	// Calculate rates
	averageSwapInPerSecond := float32(firstStat.SwapIn-secondNewest.SwapIn) / timeDiffSeconds
	averageSwapOutPerSecond := float32(firstStat.SwapOut-secondNewest.SwapOut) / timeDiffSeconds
	highTrafficCondition := averageSwapInPerSecond > sdi.maxAverageSwapInPagesPerSecond && averageSwapOutPerSecond > sdi.maxAverageSwapOutPagesPerSecond

	nodeSummary, err := sdi.psc.GetRootSummary()
	if err != nil {
		return false, err
	}
	highSwapUtilization := sdi.swapUtilizationThresholdMet(&nodeSummary)

	/*
		log.Log.Infof(fmt.Sprintf("Debug: ______________________________________________________________________________________________________________"))
		log.Log.Infof(fmt.Sprintf("Debug: averageSwapInPerSecond: %v condition: %v", averageSwapInPerSecond, averageSwapInPerSecond > sdi.maxAverageSwapInPagesPerSecond))
		log.Log.Infof(fmt.Sprintf("Debug: averageSwapOutPerSecond:%v condition: %v", averageSwapOutPerSecond, averageSwapOutPerSecond > sdi.maxAverageSwapOutPagesPerSecond))
		log.Log.Infof(fmt.Sprintf("Debug: overcommitment size:%v condition: %v", firstStat.SwapUsedBytes-firstStat.AvailableMemoryBytes-firstStat.InactiveFileBytes, overCommitmentRatioCondition))
	*/

	if highTrafficCondition {
		log.Log.Infof("highTrafficCondition is true")
		log.Log.Infof(fmt.Sprintf("Debug: averageSwapInPerSecond: %v condition: %v", averageSwapInPerSecond, averageSwapInPerSecond > sdi.maxAverageSwapInPagesPerSecond))
		log.Log.Infof(fmt.Sprintf("Debug: averageSwapOutPerSecond:%v condition: %v", averageSwapOutPerSecond, averageSwapOutPerSecond > sdi.maxAverageSwapOutPagesPerSecond))
	}

	if highSwapUtilization {
		log.Log.Infof("highSwapUtilization is true")
		log.Log.Infof(fmt.Sprintf("Debug: TotalSwapBytes %v", nodeSummary.TotalSwapBytes))
		log.Log.Infof(fmt.Sprintf("Debug: TotalMemoryBytes %v", nodeSummary.TotalMemoryBytes))
		log.Log.Infof(fmt.Sprintf("Debug: WorkingSetBytes %v", nodeSummary.WorkingSetBytes))
		log.Log.Infof(fmt.Sprintf("Debug: SwapUsedBytes %v", nodeSummary.SwapUsedBytes))
	}
	
	return highTrafficCondition || highSwapUtilization, nil
}

func (sdi *ShortageDetectorImpl) swapUtilizationThresholdMet(nodeSummary *stats_collector.NodeSummary) bool {
	factoredSwap := float64(nodeSummary.TotalSwapBytes) * sdi.swapUtilizationThresholdFactor
	capacity := float64(nodeSummary.TotalMemoryBytes) + factoredSwap
	usedVirtualmemory := nodeSummary.WorkingSetBytes + nodeSummary.SwapUsedBytes
	log.Log.V(4).Info(fmt.Sprintf("Debug: nodeSummary.TotalSwapBytes %v nodeSummary.TotalMemoryBytes %v ",
		nodeSummary.TotalSwapBytes,
		nodeSummary.TotalMemoryBytes))
	log.Log.V(4).Info(fmt.Sprintf("Debug: nodeSummary.WorkingSetBytes %v nodeSummary.SwapUsedBytes %v ",
		nodeSummary.WorkingSetBytes,
		nodeSummary.SwapUsedBytes))

	return usedVirtualmemory > uint64(capacity)
}
