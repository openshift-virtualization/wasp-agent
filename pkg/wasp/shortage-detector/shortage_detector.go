package shortage_detector

import (
	"fmt"
	"github.com/openshift-virtualization/wasp-agent/pkg/log"
	stats_collector "github.com/openshift-virtualization/wasp-agent/pkg/wasp/stats-collector"
	"time"
)

// ShortageDetector is an interface for shortage detection
type ShortageDetector interface {
	ShouldEvict() bool
}

type ShortageDetectorImpl struct {
	sc                              stats_collector.StatsCollector
	maxAverageSwapInPagesPerSecond  float32
	maxAverageSwapOutPagesPerSecond float32
	maxMemoryOverCommitmentBytes    int64
	AverageWindowSizeSeconds        time.Duration
}

func NewShortageDetectorImpl(sc stats_collector.StatsCollector, maxAverageSwapInPagesPerSecond, maxAverageSwapOutPagesPerSecond float32, maxMemoryOverCommitmentBytes int64, AverageWindowSizeSeconds time.Duration) *ShortageDetectorImpl {
	return &ShortageDetectorImpl{
		sc:                              sc,
		maxAverageSwapInPagesPerSecond:  maxAverageSwapInPagesPerSecond,
		maxAverageSwapOutPagesPerSecond: maxAverageSwapOutPagesPerSecond,
		maxMemoryOverCommitmentBytes:    maxMemoryOverCommitmentBytes,
		AverageWindowSizeSeconds:        AverageWindowSizeSeconds,
	}
}

func (sdi *ShortageDetectorImpl) ShouldEvict() bool {
	stats := sdi.sc.GetStatsList()
	if len(stats) < 2 {
		log.Log.Infof("not enough stats provided, need at least 2")
		return false
	}

	// Find the second newest Stats object after the first one with at least AverageWindowSizeSeconds difference
	firstStat := stats[0]
	var secondNewest *stats_collector.Stats
	for i := 1; i < len(stats); i++ {
		if firstStat.Time.Sub(stats[i].Time) >= sdi.AverageWindowSizeSeconds {
			secondNewest = &stats[i]
			break
		}
	}

	if secondNewest == nil {
		log.Log.Infof("could not find second newest Stats with at least %v difference", sdi.AverageWindowSizeSeconds)
		return false
	}

	// Calculate time difference in seconds
	timeDiffSeconds := float32(firstStat.Time.Sub(secondNewest.Time).Seconds())

	// Calculate rates
	averageSwapInPerSecond := float32(firstStat.SwapIn-secondNewest.SwapIn) / timeDiffSeconds
	averageSwapOutPerSecond := float32(firstStat.SwapOut-secondNewest.SwapOut) / timeDiffSeconds
	highTrafficCondition := averageSwapInPerSecond > sdi.maxAverageSwapInPagesPerSecond && averageSwapOutPerSecond > sdi.maxAverageSwapOutPagesPerSecond
	overCommitmentRatioCondition := sdi.maxMemoryOverCommitmentBytes < firstStat.SwapUsedBytes-firstStat.AvailableMemoryBytes-firstStat.InactiveFileBytes

	log.Log.Infof(fmt.Sprintf("Debug: ______________________________________________________________________________________________________________"))
	log.Log.Infof(fmt.Sprintf("Debug: averageSwapInPerSecond: %v condition: %v", averageSwapInPerSecond, averageSwapInPerSecond > sdi.maxAverageSwapInPagesPerSecond))
	log.Log.Infof(fmt.Sprintf("Debug: averageSwapOutPerSecond:%v condition: %v", averageSwapOutPerSecond, averageSwapOutPerSecond > sdi.maxAverageSwapOutPagesPerSecond))
	log.Log.Infof(fmt.Sprintf("Debug: overcommitment size:%v condition: %v", firstStat.SwapUsedBytes-firstStat.AvailableMemoryBytes-firstStat.InactiveFileBytes, overCommitmentRatioCondition))

	return highTrafficCondition || overCommitmentRatioCondition
}
