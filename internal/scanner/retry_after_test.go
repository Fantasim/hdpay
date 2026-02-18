package scanner

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		wantMin  time.Duration
		wantMax  time.Duration
		wantZero bool
	}{
		{
			name:     "missing header",
			header:   "",
			wantZero: true,
		},
		{
			name:    "seconds format 30",
			header:  "30",
			wantMin: 30 * time.Second,
			wantMax: 30 * time.Second,
		},
		{
			name:    "seconds format 1",
			header:  "1",
			wantMin: 1 * time.Second,
			wantMax: 1 * time.Second,
		},
		{
			name:     "zero seconds",
			header:   "0",
			wantZero: true,
		},
		{
			name:     "negative seconds",
			header:   "-5",
			wantZero: true,
		},
		{
			name:     "garbage value",
			header:   "not-a-number",
			wantZero: true,
		},
		{
			name:     "empty string",
			header:   "",
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := make(http.Header)
			if tt.header != "" {
				h.Set("Retry-After", tt.header)
			}

			got := parseRetryAfter(h)

			if tt.wantZero {
				if got != 0 {
					t.Errorf("parseRetryAfter() = %v, want 0", got)
				}
				return
			}

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("parseRetryAfter() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	// HTTP-date 10 seconds in the future.
	futureTime := time.Now().Add(10 * time.Second)
	httpDate := futureTime.UTC().Format(http.TimeFormat)

	h := make(http.Header)
	h.Set("Retry-After", httpDate)

	got := parseRetryAfter(h)
	if got <= 0 {
		t.Errorf("parseRetryAfter(future HTTP-date) = %v, want > 0", got)
	}
	if got > 11*time.Second {
		t.Errorf("parseRetryAfter(future HTTP-date) = %v, want <= 11s", got)
	}

	// HTTP-date in the past.
	pastTime := time.Now().Add(-10 * time.Second)
	pastDate := pastTime.UTC().Format(http.TimeFormat)

	h2 := make(http.Header)
	h2.Set("Retry-After", pastDate)

	got2 := parseRetryAfter(h2)
	if got2 != 0 {
		t.Errorf("parseRetryAfter(past HTTP-date) = %v, want 0", got2)
	}
}
