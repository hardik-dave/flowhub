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

// Count cases: grace-remaining arithmetic for §4 warning strings.
func TestCount(t *testing.T) {
	cases := []struct {
		name string
		a, b time.Time
		want int
	}{
		{"same day = 0", d(2026, time.September, 14), d(2026, time.September, 14), 0},
		{"b before a = 0", d(2026, time.September, 17), d(2026, time.September, 14), 0},
		{"Mon 14 → Thu 17 = 3 (spec §6.1 example numbers)", d(2026, time.September, 14), d(2026, time.September, 17), 3},
		{"Fri 11 → Mon 14 skips weekend = 1", d(2026, time.September, 11), d(2026, time.September, 14), 1},
		{"Fri 11 → Fri 18 = 5", d(2026, time.September, 11), d(2026, time.September, 18), 5},
		{"Sat 12 → Fri 18 = 5 (weekend start)", d(2026, time.September, 12), d(2026, time.September, 18), 5},
		{"weekend-only span = 0", d(2026, time.September, 12), d(2026, time.September, 13), 0},
		{"month boundary: Wed Sep 30 → Mon Oct 5 = 3", d(2026, time.September, 30), d(2026, time.October, 5), 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Count(c.a, c.b); got != c.want {
				t.Fatalf("Count(%s, %s) = %d, want %d", c.a.Format("2006-01-02"), c.b.Format("2006-01-02"), got, c.want)
			}
		})
	}
}

// Round-trip: Count is the inverse of Add on working-day spans.
func TestAddCountRoundTrip(t *testing.T) {
	for _, start := range []time.Time{
		d(2026, time.September, 11),
		d(2026, time.September, 12),
		d(2026, time.September, 14),
		d(2026, time.September, 30),
	} {
		for n := 0; n <= 10; n++ {
			end := Add(start, n)
			if got := Count(start, end); got != n {
				t.Fatalf("Count(%s, Add(%s, %d)) = %d, want %d",
					start.Format("2006-01-02"), start.Format("2006-01-02"), n, got, n)
			}
		}
	}
}
