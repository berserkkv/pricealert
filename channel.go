package main

import "time"

// ChannelBounds returns upper and lower parallel line prices at unix second t.
// Uses y1 + m*(t-x1) for numeric stability (unix timestamps are large).
func ChannelBounds(alert Alert, t int64) (upper, lower float64) {
	line1 := linePriceAt(alert.P1Time, alert.P1Price, alert.P2Time, alert.P2Price, t)
	line2 := line1 + alert.Offset

	if line1 >= line2 {
		return line1, line2
	}
	return line2, line1
}

// linePriceAt is the price on the line through (x1,y1)-(x2,y2) at unix time t.
func linePriceAt(x1 int64, y1 float64, x2 int64, y2 float64, t int64) float64 {
	if x2 == x1 {
		return y1
	}
	m := (y2 - y1) / float64(x2-x1)
	return y1 + m*float64(t-x1)
}

// ChannelBoundsNow uses the current time for evaluation.
func ChannelBoundsNow(alert Alert) (upper, lower float64) {
	return ChannelBounds(alert, time.Now().Unix())
}
