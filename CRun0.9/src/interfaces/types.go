package interfaces

type VenueHealth struct{ RejectRate, ErrorRate, LatencyP99 float64; HeartbeatGapSec int }
