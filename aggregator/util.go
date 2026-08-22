package main

import "time"

func stale(t time.Time, maxAge time.Duration) bool {
	return time.Since(t) > maxAge
}
