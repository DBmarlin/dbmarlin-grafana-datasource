package plugin

import (
	"net/http"
	"strconv"
	"time"
)

// The archiver's interval ladder. Anything between two steps is snapped up.
var intervalLadder = []time.Duration{
	time.Second, 5 * time.Second, 15 * time.Second,
	time.Minute, 5 * time.Minute, 15 * time.Minute,
	time.Hour, 6 * time.Hour, 24 * time.Hour, 7 * 24 * time.Hour,
}

// forcedIntervals are the coarser buckets the archiver can impose, largest
// first: 1 hour for data older than raw retention, 1 minute on non-premium
// licences.
var forcedIntervals = []time.Duration{time.Hour, time.Minute}

// snapInterval rounds the panel's interval up to the nearest ladder step.
func snapInterval(d time.Duration) time.Duration {
	for _, step := range intervalLadder {
		if d <= step {
			return step
		}
	}
	return intervalLadder[len(intervalLadder)-1]
}

// reportedInterval reads the bucket size the archiver says it used, or 0 when
// the header is missing (DBmarlin older than 6.11) or not a positive number.
func reportedInterval(h http.Header) time.Duration {
	ms, err := strconv.ParseInt(h.Get(intervalHeader), 10, 64)
	if err != nil || ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

// effectiveInterval returns the bucket size the rows were actually returned
// at when the archiver did not say. The archiver silently coarsens the interval it was asked for; if every
// time slice falls on a multiple of a forced step, that step was used. AAS is
// divided by this, otherwise it would be wrong by up to 60x.
//
// This is a heuristic: the archiver does not say which interval it used, so
// sparse data whose only buckets sit on the hour looks like a forced 1 hour.
// The result is then too low by the ratio of the intervals.
func effectiveInterval(requested time.Duration, slices []time.Time) time.Duration {
	if len(slices) == 0 {
		return requested
	}
	for _, forced := range forcedIntervals {
		if requested < forced && aligned(slices, forced) {
			return forced
		}
	}
	return requested
}

// aligned reports whether every time slice is a multiple of step from midnight UTC.
func aligned(slices []time.Time, step time.Duration) bool {
	for _, t := range slices {
		t = t.UTC()
		sinceMidnight := time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute +
			time.Duration(t.Second())*time.Second + time.Duration(t.Nanosecond())
		if sinceMidnight%step != 0 {
			return false
		}
	}
	return true
}
