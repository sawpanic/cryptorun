package apis

import (
	"context"
)

type Provider interface { Name() string; Call(ctx context.Context) error }

type Manager struct { Circuits map[string]*Circuit }

func (m *Manager) Execute(ctx context.Context, p Provider, fn func(context.Context) error) error {
	c := m.Circuits[p.Name()]
	if c == nil { return fn(ctx) }
	_, err := c.Execute(ctx, func() (interface{}, error) { return nil, fn(ctx) })
	return err
}
