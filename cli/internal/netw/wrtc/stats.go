package wrtc

import (
	"time"

	"github.com/pion/rtcp"
)

// stats.go contains functions for calculating statistics about a peer connection.

// calculateRTT computes the round-trip-time from receiver reports.
// Reference: https://webrtcforthecurious.com/docs/06-media-communication/#receiver-reports--sender-reports
func calculateRTT(rr *rtcp.ReceptionReport) time.Duration {
	if rr.LastSenderReport == 0 {
		return 0 // no SR received yet, can't calculate
	}

	// nowNTP: current time as compact NTP
	nowNTP := toNTP32(time.Now())

	// RTT = now - LSR - DLSR
	rtt := nowNTP - rr.LastSenderReport - rr.Delay
	return time.Duration(float64(rtt) / 65536.0 * float64(time.Second))
}

// toNTP32 computes seconds since 1900 from t, returning the middle 32 bits
func toNTP32(t time.Time) uint32 {
	// NTP epoch is Jan 1, 1900
	const ntpEpoch = 2208988800
	sec := uint64(t.Unix()) + ntpEpoch
	frac := uint64(t.Nanosecond()) * (1 << 32) / 1e9

	// Middle 32 bits: low 16 of seconds + high 16 of fraction
	return uint32(sec<<16) | uint32(frac>>16)
}

// jitterToMs takes a jitter uint32 timestamp from a rtcp.ReceptionReport.Jitter value
// and converts it to milliseconds using the codec's clock rate.
func jitterToMs(jitter uint32) float64 {
	return float64(jitter) * 1000.0 / float64(opusCodec.ClockRate)
}
