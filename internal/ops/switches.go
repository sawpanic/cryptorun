package ops

import (
	"sync"
	"time"
)

// SwitchManager manages emergency toggles and operational switches
type SwitchManager struct {
	config SwitchConfig
	mu     sync.RWMutex

	// Runtime state tracking
	switchStates map[string]bool
	lastUpdated  map[string]time.Time
}

// SwitchConfig holds all switch configurations
type SwitchConfig struct {
	Emergency EmergencySwitchConfig           `yaml:"emergency"`
	Providers map[string]ProviderSwitchConfig `yaml:"providers"`
	Venues    map[string]bool                 `yaml:"venues"`
}

// EmergencySwitchConfig configures emergency switches
type EmergencySwitchConfig struct {
	DisableAllScanners bool `yaml:"disable_all_scanners"`
	DisableLiveData    bool `yaml:"disable_live_data"`
	ReadOnlyMode       bool `yaml:"read_only_mode"`
}

// ProviderSwitchConfig configures switches for a specific provider
type ProviderSwitchConfig struct {
	Enabled        bool `yaml:"enabled"`
	AllowWebsocket bool `yaml:"allow_websocket"`
	AllowRest      bool `yaml:"allow_rest"`
}

// SwitchStatus represents the current status of all switches
type SwitchStatus struct {
	Emergency EmergencyStatus           `json:"emergency"`
	Providers map[string]ProviderStatus `json:"providers"`
	Venues    map[string]VenueStatus    `json:"venues"`
	LastCheck time.Time                 `json:"last_check"`
}

// EmergencyStatus represents emergency switch states
type EmergencyStatus struct {
	AllScannersDisabled bool `json:"all_scanners_disabled"`
	LiveDataDisabled    bool `json:"live_data_disabled"`
	ReadOnlyMode        bool `json:"read_only_mode"`
	AnyEmergencyActive  bool `json:"any_emergency_active"`
}

// ProviderStatus represents provider switch states
type ProviderStatus struct {
	Name             string    `json:"name"`
	Enabled          bool      `json:"enabled"`
	WebsocketAllowed bool      `json:"websocket_allowed"`
	RestAllowed      bool      `json:"rest_allowed"`
	FullyOperational bool      `json:"fully_operational"`
	LastUpdated      time.Time `json:"last_updated"`
}

// VenueStatus represents venue switch states
type VenueStatus struct {
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	LastUpdated time.Time `json:"last_updated"`
}

// NewSwitchManager creates a new switch manager
func NewSwitchManager(config SwitchConfig) *SwitchManager {
	return &SwitchManager{
		config:       config,
		switchStates: make(map[string]bool),
		lastUpdated:  make(map[string]time.Time),
	}
}

// GetStatus returns current status of all switches
func (s *SwitchManager) GetStatus() SwitchStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()

	// Emergency status
	emergency := EmergencyStatus{
		AllScannersDisabled: s.config.Emergency.DisableAllScanners,
		LiveDataDisabled:    s.config.Emergency.DisableLiveData,
		ReadOnlyMode:        s.config.Emergency.ReadOnlyMode,
	}
	emergency.AnyEmergencyActive = emergency.AllScannersDisabled ||
		emergency.LiveDataDisabled || emergency.ReadOnlyMode

	// Provider status
	providers := make(map[string]ProviderStatus)
	for name, config := range s.config.Providers {
		status := ProviderStatus{
			Name:             name,
			Enabled:          config.Enabled,
			WebsocketAllowed: config.AllowWebsocket,
			RestAllowed:      config.AllowRest,
			LastUpdated:      s.lastUpdated[name+"_provider"],
		}
		status.FullyOperational = status.Enabled && status.WebsocketAllowed && status.RestAllowed
		providers[name] = status
	}

	// Venue status
	venues := make(map[string]VenueStatus)
	for name, enabled := range s.config.Venues {
		venues[name] = VenueStatus{
			Name:        name,
			Enabled:     enabled,
			LastUpdated: s.lastUpdated[name+"_venue"],
		}
	}

	return SwitchStatus{
		Emergency: emergency,
		Providers: providers,
		Venues:    venues,
		LastCheck: now,
	}
}

// IsScannersEnabled checks if scanners are enabled
func (s *SwitchManager) IsScannersEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return !s.config.Emergency.DisableAllScanners
}

// IsLiveDataEnabled checks if live data is enabled
func (s *SwitchManager) IsLiveDataEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return !s.config.Emergency.DisableLiveData
}

// IsReadOnlyMode checks if system is in read-only mode
func (s *SwitchManager) IsReadOnlyMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config.Emergency.ReadOnlyMode
}

// IsProviderEnabled checks if a provider is enabled
func (s *SwitchManager) IsProviderEnabled(provider string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.config.Providers[provider]
	if !exists {
		return false
	}

	return config.Enabled
}

// IsProviderWebsocketAllowed checks if websocket is allowed for provider
func (s *SwitchManager) IsProviderWebsocketAllowed(provider string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.config.Providers[provider]
	if !exists {
		return false
	}

	return config.Enabled && config.AllowWebsocket
}

// IsProviderRestAllowed checks if REST is allowed for provider
func (s *SwitchManager) IsProviderRestAllowed(provider string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.config.Providers[provider]
	if !exists {
		return false
	}

	return config.Enabled && config.AllowRest
}

// IsVenueEnabled checks if a venue is enabled
func (s *SwitchManager) IsVenueEnabled(venue string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	enabled, exists := s.config.Venues[venue]
	if !exists {
		return false
	}

	return enabled
}

// SetEmergencySwitch sets an emergency switch state
func (s *SwitchManager) SetEmergencySwitch(switchType string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	switch switchType {
	case "disable_all_scanners":
		s.config.Emergency.DisableAllScanners = enabled
		s.lastUpdated["emergency_scanners"] = now
	case "disable_live_data":
		s.config.Emergency.DisableLiveData = enabled
		s.lastUpdated["emergency_live_data"] = now
	case "read_only_mode":
		s.config.Emergency.ReadOnlyMode = enabled
		s.lastUpdated["emergency_readonly"] = now
	}
}

// SetProviderSwitch sets a provider switch state
func (s *SwitchManager) SetProviderSwitch(provider, switchType string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	config, exists := s.config.Providers[provider]
	if !exists {
		// Create new provider config
		config = ProviderSwitchConfig{
			Enabled:        true,
			AllowWebsocket: true,
			AllowRest:      true,
		}
	}

	switch switchType {
	case "enabled":
		config.Enabled = enabled
	case "websocket":
		config.AllowWebsocket = enabled
	case "rest":
		config.AllowRest = enabled
	}

	s.config.Providers[provider] = config
	s.lastUpdated[provider+"_provider"] = now
}

// SetVenueSwitch sets a venue switch state
func (s *SwitchManager) SetVenueSwitch(venue string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.Venues[venue] = enabled
	s.lastUpdated[venue+"_venue"] = time.Now()
}

// GetEnabledProviders returns list of enabled providers
func (s *SwitchManager) GetEnabledProviders() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var enabled []string
	for name, config := range s.config.Providers {
		if config.Enabled {
			enabled = append(enabled, name)
		}
	}

	return enabled
}

// GetEnabledVenues returns list of enabled venues
func (s *SwitchManager) GetEnabledVenues() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var enabled []string
	for name, isEnabled := range s.config.Venues {
		if isEnabled {
			enabled = append(enabled, name)
		}
	}

	return enabled
}

// GetFullyOperationalProviders returns providers that are fully operational
func (s *SwitchManager) GetFullyOperationalProviders() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var operational []string
	for name, config := range s.config.Providers {
		if config.Enabled && config.AllowWebsocket && config.AllowRest {
			operational = append(operational, name)
		}
	}

	return operational
}

// HasAnyEmergencyActive checks if any emergency switches are active
func (s *SwitchManager) HasAnyEmergencyActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config.Emergency.DisableAllScanners ||
		s.config.Emergency.DisableLiveData ||
		s.config.Emergency.ReadOnlyMode
}

// GetEmergencyState returns detailed emergency state
func (s *SwitchManager) GetEmergencyState() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"disable_all_scanners": s.config.Emergency.DisableAllScanners,
		"disable_live_data":    s.config.Emergency.DisableLiveData,
		"read_only_mode":       s.config.Emergency.ReadOnlyMode,
		"any_active":           s.HasAnyEmergencyActive(),
		"last_updated": map[string]interface{}{
			"scanners":  s.lastUpdated["emergency_scanners"],
			"live_data": s.lastUpdated["emergency_live_data"],
			"read_only": s.lastUpdated["emergency_readonly"],
		},
	}
}

// EnableAllSystems enables all systems (clears all emergency switches)
func (s *SwitchManager) EnableAllSystems() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	s.config.Emergency.DisableAllScanners = false
	s.config.Emergency.DisableLiveData = false
	s.config.Emergency.ReadOnlyMode = false

	s.lastUpdated["emergency_scanners"] = now
	s.lastUpdated["emergency_live_data"] = now
	s.lastUpdated["emergency_readonly"] = now
}

// DisableAllSystems disables all systems (sets all emergency switches)
func (s *SwitchManager) DisableAllSystems() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	s.config.Emergency.DisableAllScanners = true
	s.config.Emergency.DisableLiveData = true
	s.config.Emergency.ReadOnlyMode = true

	s.lastUpdated["emergency_scanners"] = now
	s.lastUpdated["emergency_live_data"] = now
	s.lastUpdated["emergency_readonly"] = now
}
