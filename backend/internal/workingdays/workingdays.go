// Package workingdays implements the grace-period date arithmetic
// from SPEC.md §4: add_working_days(d, n) advances day by day,
// counting only Mon–Fri. Public holidays are deliberately out of
// scope (spec §2.5). Pure function; golden-case tests alongside.
package workingdays

import "time"

// Add returns the date n working days (Mon–Fri) after d.
// n == 0 returns d unchanged. Weekend start dates are handled by the
// same rule — days are only counted when they land on Mon–Fri.
func Add(d time.Time, n int) time.Time {
	counted := 0
	for counted < n {
		d = d.AddDate(0, 0, 1)
		wd := d.Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			counted++
		}
	}
	return d
}
