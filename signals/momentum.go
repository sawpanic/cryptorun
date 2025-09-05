package signals

import "math"

func Last(xs []float64) float64 { if len(xs)==0 { return 0 }; return xs[len(xs)-1] }

func MomentumCore(close []float64) float64 {
    // simple z-score of last 4h return over 20-bar window (placeholder for slice)
    n := len(close)
    if n < 21 { return 0 }
    r := make([]float64, n-1)
    for i := 1; i < n; i++ { r[i-1] = close[i]/close[i-1]-1 }
    // last return vs mean/std of prior 20
    m := 0.0
    for i := n-21; i < n-1; i++ { m += r[i] }
    m /= 20
    v := 0.0
    for i := n-21; i < n-1; i++ { d := r[i]-m; v += d*d }
    v /= 20
    std := math.Sqrt(v)
    if std == 0 { return 0 }
    z := (r[n-2]-m)/std
    return z
}

func ATR(close []float64, n int) float64 {
    if len(close) < n+1 || n <= 0 { return 0 }
    sum := 0.0
    for i := len(close)-n; i < len(close); i++ {
        d := close[i]-close[i-1]
        if d < 0 { d = -d }
        sum += d
    }
    return sum/float64(n)
}

func RSI(close []float64, n int) float64 {
    if len(close) < n+1 || n <= 0 { return 50 }
    var gain, loss float64
    for i := len(close)-n; i < len(close); i++ {
        d := close[i]-close[i-1]
        if d > 0 { gain += d } else { loss -= d }
    }
    if loss == 0 { return 100 }
    rs := (gain/float64(n))/(loss/float64(n))
    return 100 - (100/(1+rs))
}

func Accel4h(close []float64) float64 {
    if len(close) < 3 { return 0 }
    r1 := close[len(close)-1]/close[len(close)-2]-1
    r2 := close[len(close)-2]/close[len(close)-3]-1
    return r1 - r2
}

func VADR(vol []float64) float64 {
    if len(vol) < 24+20 { return 1 }
    // 24h volume vs 20-bar avg (1h bars expected by caller for proxy)
    n := len(vol)
    v24 := 0.0
    for i := n-24; i < n; i++ { v24 += vol[i] }
    avg20 := 0.0
    for i := n-44; i < n-24; i++ { avg20 += vol[i] }
    avg20 /= 20
    if avg20 == 0 { return 1 }
    return v24/avg20
}

