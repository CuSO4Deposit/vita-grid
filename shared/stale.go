package shared

import "time"

func Stale(t time.Time, maxAge time.Duration) bool {
	return time.Since(t) > maxAge
}
