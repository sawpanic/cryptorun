package breakers

import (
    "time"
    cb "github.com/sony/gobreaker"
)

type Breaker struct{ cb *cb.CircuitBreaker }

func New(name string) *Breaker {
    st := cb.Settings{Name: name}
    st.Interval = 60 * time.Second
    st.Timeout = 60 * time.Second
    st.ReadyToTrip = func(counts cb.Counts) bool {
        if counts.ConsecutiveFailures >= 3 { return true }
        total := counts.Requests
        if total < 20 { return false }
        if float64(counts.TotalFailures)/float64(total) > 0.05 { return true }
        return false
    }
    return &Breaker{cb: cb.NewCircuitBreaker(st)}
}

func (b *Breaker) Execute(fn func() (any, error)) (any, error) { return b.cb.Execute(fn) }

