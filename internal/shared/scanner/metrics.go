package scanner

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricsReporter is implemented by any provider that tracks its own call metrics.
// pool.go checks this interface via type assertion after each provider call.
type MetricsReporter interface {
	RecordSuccess()
	RecordFailure(is429 bool)
}

// ProviderMetrics tracks API call counts for a single provider using atomic
// counters for lifetime totals and mutex-protected period buckets for
// daily / weekly / monthly resets.
//
// Resets are lazy: they happen on the next RecordSuccess or RecordFailure
// call after a period boundary passes — no background goroutine required.
type ProviderMetrics struct {
	name              string
	knownMonthlyLimit int64 // 0 = no documented cap

	// Lifetime counters — reset only on process restart.
	totalRequests  atomic.Int64
	totalSuccesses atomic.Int64
	totalFailures  atomic.Int64
	total429s      atomic.Int64

	mu      sync.Mutex
	daily   periodBucket // resets every 24 hours
	weekly  periodBucket // resets every 7 days
	monthly periodBucket // resets every 30 days
}

type periodBucket struct {
	requests  int64
	successes int64
	failures  int64
	hits429   int64
	resetAt   time.Time     // wall-clock time of next reset
	duration  time.Duration // bucket length (24h / 7d / 30d)
}

// MetricsSnapshot is a safe, point-in-time copy of ProviderMetrics suitable
// for JSON marshalling and API responses.
type MetricsSnapshot struct {
	Name              string         `json:"name"`
	KnownMonthlyLimit int64          `json:"knownMonthlyLimit"`
	Total             PeriodSnapshot `json:"total"`
	Daily             PeriodSnapshot `json:"daily"`
	Weekly            PeriodSnapshot `json:"weekly"`
	Monthly           PeriodSnapshot `json:"monthly"`
}

// PeriodSnapshot holds aggregated counts for one time window.
type PeriodSnapshot struct {
	Requests  int64 `json:"requests"`
	Successes int64 `json:"successes"`
	Failures  int64 `json:"failures"`
	Hits429   int64 `json:"hits429"`
}

// NewProviderMetrics creates a ProviderMetrics for the named provider.
// knownMonthlyLimit is the provider's documented monthly call cap (0 = none).
func NewProviderMetrics(name string, knownMonthlyLimit int64) *ProviderMetrics {
	now := time.Now()
	return &ProviderMetrics{
		name:              name,
		knownMonthlyLimit: knownMonthlyLimit,
		daily: periodBucket{
			duration: 24 * time.Hour,
			resetAt:  now.Add(24 * time.Hour),
		},
		weekly: periodBucket{
			duration: 7 * 24 * time.Hour,
			resetAt:  now.Add(7 * 24 * time.Hour),
		},
		monthly: periodBucket{
			duration: 30 * 24 * time.Hour,
			resetAt:  now.Add(30 * 24 * time.Hour),
		},
	}
}

// RecordSuccess increments success counters across all windows.
func (m *ProviderMetrics) RecordSuccess() {
	m.totalRequests.Add(1)
	m.totalSuccesses.Add(1)

	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.maybeReset(now)
	m.daily.requests++
	m.daily.successes++
	m.weekly.requests++
	m.weekly.successes++
	m.monthly.requests++
	m.monthly.successes++
}

// RecordFailure increments failure counters across all windows.
// Pass is429=true when the provider returned HTTP 429 (rate limit hit).
func (m *ProviderMetrics) RecordFailure(is429 bool) {
	m.totalRequests.Add(1)
	m.totalFailures.Add(1)
	if is429 {
		m.total429s.Add(1)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	m.maybeReset(now)
	m.daily.requests++
	m.daily.failures++
	m.weekly.requests++
	m.weekly.failures++
	m.monthly.requests++
	m.monthly.failures++
	if is429 {
		m.daily.hits429++
		m.weekly.hits429++
		m.monthly.hits429++
	}
}

// maybeReset zeroes any bucket whose resetAt has passed.
// Must be called with m.mu held.
func (m *ProviderMetrics) maybeReset(now time.Time) {
	resetBucket := func(b *periodBucket) {
		if now.After(b.resetAt) {
			b.requests = 0
			b.successes = 0
			b.failures = 0
			b.hits429 = 0
			b.resetAt = now.Add(b.duration)
		}
	}
	resetBucket(&m.daily)
	resetBucket(&m.weekly)
	resetBucket(&m.monthly)
}

// Snapshot returns a safe point-in-time copy of all metrics.
func (m *ProviderMetrics) Snapshot() MetricsSnapshot {
	m.mu.Lock()
	d := m.daily
	w := m.weekly
	mo := m.monthly
	m.mu.Unlock()

	return MetricsSnapshot{
		Name:              m.name,
		KnownMonthlyLimit: m.knownMonthlyLimit,
		Total: PeriodSnapshot{
			Requests:  m.totalRequests.Load(),
			Successes: m.totalSuccesses.Load(),
			Failures:  m.totalFailures.Load(),
			Hits429:   m.total429s.Load(),
		},
		Daily: PeriodSnapshot{
			Requests:  d.requests,
			Successes: d.successes,
			Failures:  d.failures,
			Hits429:   d.hits429,
		},
		Weekly: PeriodSnapshot{
			Requests:  w.requests,
			Successes: w.successes,
			Failures:  w.failures,
			Hits429:   w.hits429,
		},
		Monthly: PeriodSnapshot{
			Requests:  mo.requests,
			Successes: mo.successes,
			Failures:  mo.failures,
			Hits429:   mo.hits429,
		},
	}
}
