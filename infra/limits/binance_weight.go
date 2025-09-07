package limits

import "net/http"

// ReadBinanceWeight extracts the X-MBX-USED-WEIGHT headers if present.
func ReadBinanceWeight(h http.Header) (string, string) {
	return h.Get("X-MBX-USED-WEIGHT-1M"), h.Get("X-MBX-USED-WEIGHT")
}
