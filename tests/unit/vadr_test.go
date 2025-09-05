package unit

import (
    "testing"
    d "cprotocol/domain"
)

func TestVADR(t *testing.T){
    v := make([]float64, 20)
    for i := range v { v[i] = 1 }
    r, ok := d.VADR(v)
    if !ok || r != 1 { t.Fatal("bad vadr") }
}
