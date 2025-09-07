package guards

// Manager manages trading guards
type Manager struct{}

// NewManager creates a new guards manager
func NewManager() *Manager {
	return &Manager{}
}