package stats_collector

import (
	"fmt"
	"github.com/shirou/gopsutil/mem"
	"sync"
	"time"
)

// StatsCollector is an interface for gathering statistics
type StatsCollector interface {
	GatherStats()
	GetStatsList() []Stats
	FlushStats()
}

type Stats struct {
	AvailableMemoryBytes int64
	InactiveFileBytes    int64
	SwapUsedBytes        int64
	SwapIn               uint64
	SwapOut              uint64
	Time                 time.Time
}

type StatsCollectorImpl struct {
	mu               sync.Mutex
	statsList        []Stats
	statsListMaxSize int
}

func NewStatsCollectorImpl() *StatsCollectorImpl {
	return &StatsCollectorImpl{
		mu:               sync.Mutex{},
		statsListMaxSize: 10000,
	}
}

func (sc *StatsCollectorImpl) GatherStats() {
	// Get current swap and memory stats
	swap, err := mem.SwapMemory()
	if err != nil {
		fmt.Println("Error fetching swap memory:", err)
		return
	}

	newStats := Stats{
		SwapIn:  swap.Sin,
		SwapOut: swap.Sout,
		Time:    time.Now(),
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.statsList = append([]Stats{newStats}, sc.statsList...)

	if len(sc.statsList) > sc.statsListMaxSize {
		sc.statsList = sc.statsList[:sc.statsListMaxSize]
	}
}

func (sc *StatsCollectorImpl) GetStatsList() []Stats {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.statsList
}

func (sc *StatsCollectorImpl) FlushStats() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.statsList = []Stats{}
}
