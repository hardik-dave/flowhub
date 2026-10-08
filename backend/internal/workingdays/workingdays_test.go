package workingdays

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// Golden cases required by SPEC.md §4 / M-B acceptance.
func TestAdd(t *testing.T) {
	cases := []struct {
		name  string
		start time.Time
		n     int
		want  time.Time
	}{
		{"zero days is identity", d(2026, time.September, 11), 0, d(2026, time.September, 11)},
		{"Fri + 5 working days = next Fri", d(2026, time.September, 11), 5, d(2026, time.September, 18)},
		{"Mon + 1 = Tue", d(2026, time.September, 14), 1, d(2026, time.September, 15)},
		{"Fri + 1 skips weekend to Mon", d(2026, time.September, 11), 1, d(2026, time.September, 14)},
		{"Sat + 1 lands Mon", d(2026, time.September, 12), 1, d(2026, time.September, 14)},
		{"Sun + 1 lands Mon", d(2026, time.September, 13), 1, d(2026, time.September, 14)},
		{"Thu + 2 spans weekend to Mon", d(2026, time.September, 10), 2, d(2026, time.September, 14)},
		{"month boundary: Wed Sep 30 + 3 = Mon Oct 5", d(2026, time.September, 30), 3, d(2026, time.October, 5)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Add(c.start, c.n)
			if !got.Equal(c.want) {
				t.Fatalf("Add(%v, %d) = %v, want %v", c.start.Format("2006-01-02"), c.n, got.Format("2006-01-02"), c.want.Format("2006-01-02"))
			}
		})
	}
}
