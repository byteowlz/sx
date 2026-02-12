package backends

import (
	"fmt"
	"strings"
)

// Manager coordinates search across multiple backends with fallback support
type Manager struct {
	primary   SearchBackend
	fallbacks []SearchBackend
	registry  map[string]SearchBackend
}

// NewManager creates a new backend manager
func NewManager() *Manager {
	return &Manager{
		registry: make(map[string]SearchBackend),
	}
}

// Register adds a backend to the registry
func (m *Manager) Register(backend SearchBackend) {
	m.registry[backend.Name()] = backend
}

// SetPrimary sets the primary search backend by name
func (m *Manager) SetPrimary(name string) error {
	backend, ok := m.registry[name]
	if !ok {
		return fmt.Errorf("unknown backend: %s (available: %s)", name, m.availableNames())
	}
	m.primary = backend
	return nil
}

// SetFallbacks sets the fallback backends in order
func (m *Manager) SetFallbacks(names []string) error {
	m.fallbacks = nil
	for _, name := range names {
		backend, ok := m.registry[name]
		if !ok {
			return fmt.Errorf("unknown fallback backend: %s (available: %s)", name, m.availableNames())
		}
		m.fallbacks = append(m.fallbacks, backend)
	}
	return nil
}

// Search performs a search using the primary backend, falling back to alternatives
// Returns the results, the backend name that succeeded, and any error
func (m *Manager) Search(opts SearchOptions) ([]SearchResult, string, error) {
	if m.primary == nil {
		return nil, "", fmt.Errorf("no primary backend configured")
	}

	// Try primary backend first
	results, err := m.primary.Search(opts)
	if err == nil {
		return results, m.primary.Name(), nil
	}

	// Primary failed - collect errors
	errors := []string{err.Error()}

	// Try fallbacks in order
	for _, fb := range m.fallbacks {
		if !fb.IsAvailable() {
			errors = append(errors, fmt.Sprintf("%s: not configured", fb.Name()))
			continue
		}

		results, fbErr := fb.Search(opts)
		if fbErr == nil {
			return results, fb.Name(), nil
		}
		errors = append(errors, fbErr.Error())
	}

	return nil, "", fmt.Errorf("all backends failed:\n  %s", strings.Join(errors, "\n  "))
}

// SearchExplicit searches using a specific backend by name (no fallback)
func (m *Manager) SearchExplicit(name string, opts SearchOptions) ([]SearchResult, error) {
	backend, ok := m.registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown backend: %s (available: %s)", name, m.availableNames())
	}
	if !backend.IsAvailable() {
		return nil, fmt.Errorf("backend %s is not configured (missing API key?)", name)
	}
	return backend.Search(opts)
}

// GetBackend returns a backend by name
func (m *Manager) GetBackend(name string) (SearchBackend, bool) {
	b, ok := m.registry[name]
	return b, ok
}

// AvailableBackends returns names of all registered backends
func (m *Manager) AvailableBackends() []string {
	names := make([]string, 0, len(m.registry))
	for name := range m.registry {
		names = append(names, name)
	}
	return names
}

// ConfiguredBackends returns names of backends that are available (configured)
func (m *Manager) ConfiguredBackends() []string {
	names := make([]string, 0, len(m.registry))
	for name, backend := range m.registry {
		if backend.IsAvailable() {
			names = append(names, name)
		}
	}
	return names
}

func (m *Manager) availableNames() string {
	return strings.Join(m.AvailableBackends(), ", ")
}
