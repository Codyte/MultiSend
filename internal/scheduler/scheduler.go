package scheduler

import (
	"sync"
	"time"

	"lab/multinet/internal/manifest"
)

type Channel struct {
	Name      string
	Active    bool
	InFlight  int
	Mbps30s   float64
	Failures  int
	LastError string
}

type Scheduler interface {
	NextChunk(pending []manifest.ChunkState, channels []Channel) (chunkIndex int64, channelName string, ok bool)
	ReportSuccess(channel string, bytes int64, duration time.Duration)
	ReportFailure(channel string, err error)
}

type state struct {
	failures     int
	cooldownEnds time.Time
	mbps30s      float64
}

type WeightedScheduler struct {
	now           func() time.Time
	cooldown      time.Duration
	maxCooldown   time.Duration
	mutex         sync.Mutex
	channelStates map[string]state
	nextChunkPos  int
}

const (
	InitialEstimatedMbps = 10.0
	HysteresisFactor     = 1.05
)

func NewWeightedScheduler() *WeightedScheduler {
	return &WeightedScheduler{
		now:           time.Now,
		cooldown:      10 * time.Second,
		maxCooldown:   40 * time.Second,
		channelStates: map[string]state{},
	}
}

func (s *WeightedScheduler) NextChunk(pending []manifest.ChunkState, channels []Channel) (int64, string, bool) {
	if len(pending) == 0 {
		return 0, "", false
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := s.now()
	var bestChannel string
	bestScore := -1.0

	for _, c := range channels {
		if !c.Active {
			continue
		}
		st := s.channelStates[c.Name]
		if now.Before(st.cooldownEnds) {
			continue
		}

		currentMbps := c.Mbps30s
		if st.mbps30s > currentMbps {
			currentMbps = st.mbps30s
		}

		if currentMbps == 0 && c.InFlight == 0 {
			currentMbps = InitialEstimatedMbps
		}

		score := currentMbps / float64(c.InFlight+1)
		if c.InFlight == 0 && currentMbps > 0 {
			score *= 1.5
		}

		if bestChannel != "" && score <= (bestScore*HysteresisFactor) {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestChannel = c.Name
		}
	}
	if bestChannel == "" {
		return 0, "", false
	}

	idx := pending[s.nextChunkPos%len(pending)].Index
	s.nextChunkPos++
	return idx, bestChannel, true
}

func (s *WeightedScheduler) ReportSuccess(channel string, bytes int64, duration time.Duration) {
	if duration <= 0 {
		return
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	st := s.channelStates[channel]
	mbps := (float64(bytes) * 8) / duration.Seconds() / 1_000_000
	if st.mbps30s == 0 {
		st.mbps30s = mbps
	} else {
		st.mbps30s = st.mbps30s*0.6 + mbps*0.4
	}
	st.failures = 0
	st.cooldownEnds = time.Time{}
	s.channelStates[channel] = st
}

func (s *WeightedScheduler) ReportFailure(channel string, _ error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	st := s.channelStates[channel]
	st.failures++
	if st.failures >= 3 && s.cooldown > 0 {
		backoffSteps := st.failures - 3
		cd := s.cooldown
		for i := 0; i < backoffSteps; i++ {
			cd *= 2
			if s.maxCooldown > 0 && cd >= s.maxCooldown {
				cd = s.maxCooldown
				break
			}
		}
		st.cooldownEnds = s.now().Add(cd)
	}
	s.channelStates[channel] = st
}
