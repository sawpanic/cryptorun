package domain

import "time"

func LateFillGuard(signalTime, fillTime time.Time) bool {
	return fillTime.Sub(signalTime) <= 30*time.Second
}
