package metrics

import (
	"sort"
	"time"
)

// PercentileDurations returns the p-th percentile (0-100) of durations,
// using linear interpolation between ordered values.
func PercentileDurations(durations []time.Duration, p float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	if len(durations) == 1 {
		return durations[0]
	}

	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	pos := (p / 100.0) * float64(len(sorted)-1)
	lower := int(pos)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	fraction := pos - float64(lower)
	return time.Duration(float64(sorted[lower]) + fraction*(float64(sorted[upper])-float64(sorted[lower])))
}

// DistributionStats returns percentile labels ("p50", "p75", "p90", "p95", "p99") mapped to duration values.
func DistributionStats(durations []time.Duration) map[string]time.Duration {
	return map[string]time.Duration{
		"p50": PercentileDurations(durations, 50),
		"p75": PercentileDurations(durations, 75),
		"p90": PercentileDurations(durations, 90),
		"p95": PercentileDurations(durations, 95),
		"p99": PercentileDurations(durations, 99),
	}
}
