package scheduler

import (
	"errors"
	"testing"
	"time"

	"github.com/Codyte/MultiSend/internal/manifest"
)

func TestChoosesFastestWithSameInflight(t *testing.T) {
	s := NewWeightedScheduler()
	pending := []manifest.ChunkState{{Index: 1}}
	channels := []Channel{{Name: "wifi", Active: true, Mbps30s: 20}, {Name: "cable", Active: true, Mbps30s: 80}}
	_, ch, ok := s.NextChunk(pending, channels)
	if !ok || ch != "cable" {
		t.Fatalf("want cable got %s", ch)
	}
}

func TestCooldownAfterFailures(t *testing.T) {
	now := time.Now()
	s := NewWeightedScheduler()
	s.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		s.ReportFailure("wifi", errors.New("x"))
	}
	pending := []manifest.ChunkState{{Index: 1}}
	channels := []Channel{{Name: "wifi", Active: true, Mbps30s: 100}, {Name: "cable", Active: true, Mbps30s: 10}}
	_, ch, ok := s.NextChunk(pending, channels)
	if !ok || ch != "cable" {
		t.Fatalf("want cable during wifi cooldown, got %s", ch)
	}
	now = now.Add(11 * time.Second)
	_, ch, ok = s.NextChunk(pending, channels)
	if !ok {
		t.Fatal("want selectable channel")
	}
	if ch != "wifi" {
		t.Fatalf("want wifi after cooldown, got %s", ch)
	}
}

func TestSingleChannel(t *testing.T) {
	s := NewWeightedScheduler()
	pending := []manifest.ChunkState{{Index: 7}}
	channels := []Channel{{Name: "wifi", Active: true}}
	idx, ch, ok := s.NextChunk(pending, channels)
	if !ok || idx != 7 || ch != "wifi" {
		t.Fatalf("bad pick idx=%d ch=%s", idx, ch)
	}
}

func TestNextChunkRoundRobinFairness(t *testing.T) {
	s := NewWeightedScheduler()
	pending := []manifest.ChunkState{{Index: 0}, {Index: 1}, {Index: 2}}
	channels := []Channel{{Name: "wifi", Active: true}}
	idx1, _, _ := s.NextChunk(pending, channels)
	idx2, _, _ := s.NextChunk(pending, channels)
	if idx1 == idx2 {
		t.Fatalf("expected rotating chunk selection, got %d and %d", idx1, idx2)
	}
}

func TestCooldownBackoffCapsAtMax(t *testing.T) {
	now := time.Now()
	s := NewWeightedScheduler()
	s.now = func() time.Time { return now }
	for i := 0; i < 6; i++ {
		s.ReportFailure("wifi", errors.New("x"))
	}
	st := s.channelStates["wifi"]
	if st.cooldownEnds.Sub(now) > s.maxCooldown {
		t.Fatalf("cooldown exceeded max: %v", st.cooldownEnds.Sub(now))
	}
}

func TestHysteresisPreventsFlappingOnNearScores(t *testing.T) {
	s := NewWeightedScheduler()
	pending := []manifest.ChunkState{{Index: 0}}
	// cable score=100, wifi score=101 (1% gain) -> below 5% hysteresis
	channels := []Channel{
		{Name: "cable", Active: true, Mbps30s: 100, InFlight: 0},
		{Name: "wifi", Active: true, Mbps30s: 101, InFlight: 0},
	}
	_, ch, ok := s.NextChunk(pending, channels)
	if !ok {
		t.Fatal("expected selectable channel")
	}
	// First channel keeps leadership since competitor is within hysteresis window.
	if ch != "cable" {
		t.Fatalf("expected cable due to hysteresis, got %s", ch)
	}
}
