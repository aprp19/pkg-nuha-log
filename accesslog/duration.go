package accesslog

import "time"

func setDurationFromStart(event *AccessLogEvent, start time.Time) {
	elapsed := time.Since(start)
	if elapsed < time.Millisecond {
		us := elapsed.Microseconds()
		event.DurationUs = &us
		event.DurationMs = nil
		return
	}
	ms := elapsed.Milliseconds()
	event.DurationMs = &ms
	event.DurationUs = nil
}
