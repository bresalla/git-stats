package metrics

import (
	"testing"
	"time"
)

func TestPercentileDurations(t *testing.T) {
	durations := []time.Duration{
		1 * time.Hour,
		2 * time.Hour,
		3 * time.Hour,
		4 * time.Hour,
		5 * time.Hour,
	}

	if got := PercentileDurations(durations, 50); got != 3*time.Hour {
		t.Errorf("expected P50 to be 3h, got %v", got)
	}
	if got := PercentileDurations(durations, 75); got != 4*time.Hour {
		t.Errorf("expected P75 to be 4h, got %v", got)
	}
	if got := PercentileDurations(nil, 50); got != 0 {
		t.Errorf("expected 0 for empty input, got %v", got)
	}
	if got := PercentileDurations([]time.Duration{7 * time.Hour}, 90); got != 7*time.Hour {
		t.Errorf("expected single value to be returned regardless of percentile, got %v", got)
	}
}

func TestDistributionStats(t *testing.T) {
	durations := []time.Duration{
		1 * time.Hour,
		2 * time.Hour,
		3 * time.Hour,
		4 * time.Hour,
		5 * time.Hour,
	}

	stats := DistributionStats(durations)
	for _, key := range []string{"p50", "p75", "p90", "p95", "p99"} {
		if _, ok := stats[key]; !ok {
			t.Errorf("expected %s in stats", key)
		}
	}
}
