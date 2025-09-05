package apis

import (
	"context"
	"time"

	"github.com/sony/gobreaker"
)

type Circuit struct { cb *gobreaker.CircuitBreaker; probeInterval time.Duration }

func NewCircuit(name string, failure, success uint32, timeout, probe time.Duration) *Circuit {
	st := gobreaker.Settings{Name: name}
	st.ReadyToTrip = func(counts gobreaker.Counts) bool { return counts.ConsecutiveFailures >= failure }
	st.Interval = 0
	st.Timeout = timeout
	c := &Circuit{ cb: gobreaker.NewCircuitBreaker(st), probeInterval: probe }
	return c
}

func (c *Circuit) Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	return c.cb.Execute(fn)
}

func (c *Circuit) StartProbe(ctx context.Context, fn func(context.Context)) {
	ticker := time.NewTicker(c.probeInterval)
	go func(){
		for {
			select {
			case <-ctx.Done(): return
			case <-ticker.C:
				fn(ctx)
			}
		}
	}()
}
