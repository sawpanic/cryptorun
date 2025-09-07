package unit

import (
	d "github.com/sawpanic/cryptorun/internal/domain"
	"testing"
)

func TestGramSchmidt(t *testing.T) {
	X := [][]float64{{1, 1}, {0, 1}}
	Q := d.GramSchmidt(X)
	if len(Q) != 2 || len(Q[0]) != 2 {
		t.Fatal("bad shape")
	}
}
