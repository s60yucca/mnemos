package util

import "time"

// NowUTC returns the current time in UTC
func NowUTC() time.Time { return time.Now().UTC() }

// TimeToUnixNano converts a time.Time to Unix nanoseconds
func TimeToUnixNano(t time.Time) int64 { return t.UnixNano() }

// UnixNanoToTime converts Unix nanoseconds to time.Time in UTC
func UnixNanoToTime(ns int64) time.Time { return time.Unix(0, ns).UTC() }
