package domain

import "testing"

func TestVADR(t *testing.T){
	v := make([]float64, 20)
	for i := range v { v[i] = 1 }
	r, ok := VADR(v)
	if !ok || r != 1 { t.Fatal("bad vadr") }
}
