package scoring

// GramSchmidtOrthonormal performs a simple Gram-Schmidt process to produce
// an orthonormal basis given input column vectors. It returns the resulting
// vectors as columns of a new matrix. This is a small utility to support the
// orthogonal factor system; production scoring will layer weights on top.
func GramSchmidtOrthonormal(cols [][]float64) [][]float64 {
    if len(cols) == 0 {
        return nil
    }
    m := len(cols[0])
    var basis [][]float64
    for _, v := range cols {
        if len(v) != m {
            // skip malformed
            continue
        }
        // Make a copy we can mutate
        u := make([]float64, m)
        copy(u, v)
        // Subtract projections on previous basis vectors
        for _, b := range basis {
            coeff := dot(u, b)
            for i := 0; i < m; i++ {
                u[i] -= coeff * b[i]
            }
        }
        // Normalize
        n := norm(u)
        if n == 0 {
            continue
        }
        for i := 0; i < m; i++ {
            u[i] /= n
        }
        basis = append(basis, u)
    }
    return basis
}

func dot(a, b []float64) float64 {
    var s float64
    for i := range a {
        s += a[i] * b[i]
    }
    return s
}

func norm(a []float64) float64 {
    var s float64
    for i := range a {
        s += a[i] * a[i]
    }
    return sqrt(s)
}

// tiny local sqrt to avoid extra deps; use math.Sqrt in production
func sqrt(x float64) float64 {
    // Newton-Raphson iterations
    if x <= 0 {
        return 0
    }
    z := x
    for i := 0; i < 12; i++ {
        z -= (z*z - x) / (2 * z)
    }
    return z
}

