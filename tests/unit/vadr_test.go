package unit

import (
	d "cryptorun/internal/domain"
	"testing"
)

func TestVADR(t *testing.T) {
	v := make([]float64, 20)
	for i := range v {
		v[i] = 1
	}
	r, ok := d.VADR(v)
	if !ok || r != 1 {
		t.Fatal("bad vadr")
	}
}
