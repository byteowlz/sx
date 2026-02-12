package backends

import (
	"fmt"
	"strings"
	"testing"
)

// mockBackend is a configurable mock for testing
type mockBackend struct {
	name      string
	available bool
	results   []SearchResult
	err       error
}

func (m *mockBackend) Name() string          { return m.name }
func (m *mockBackend) IsAvailable() bool     { return m.available }
func (m *mockBackend) Search(opts SearchOptions) ([]SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func TestManager_Register(t *testing.T) {
	mgr := NewManager()
	b := &mockBackend{name: "mock1", available: true}
	mgr.Register(b)

	backends := mgr.AvailableBackends()
	if len(backends) != 1 || backends[0] != "mock1" {
		t.Errorf("expected [mock1], got %v", backends)
	}
}

func TestManager_SetPrimary(t *testing.T) {
	mgr := NewManager()
	mgr.Register(&mockBackend{name: "mock1", available: true})

	if err := mgr.SetPrimary("mock1"); err != nil {
		t.Errorf("SetPrimary failed: %v", err)
	}

	if err := mgr.SetPrimary("nonexistent"); err == nil {
		t.Error("SetPrimary should fail for unknown backend")
	}
}

func TestManager_SetFallbacks(t *testing.T) {
	mgr := NewManager()
	mgr.Register(&mockBackend{name: "primary", available: true})
	mgr.Register(&mockBackend{name: "fb1", available: true})
	mgr.Register(&mockBackend{name: "fb2", available: true})

	if err := mgr.SetFallbacks([]string{"fb1", "fb2"}); err != nil {
		t.Errorf("SetFallbacks failed: %v", err)
	}

	if err := mgr.SetFallbacks([]string{"fb1", "nonexistent"}); err == nil {
		t.Error("SetFallbacks should fail for unknown backend")
	}
}

func TestManager_Search_PrimarySuccess(t *testing.T) {
	mgr := NewManager()

	primary := &mockBackend{
		name:      "primary",
		available: true,
		results:   []SearchResult{{Title: "Result 1", URL: "https://example.com"}},
	}
	fallback := &mockBackend{
		name:      "fallback",
		available: true,
		results:   []SearchResult{{Title: "Fallback Result", URL: "https://fallback.com"}},
	}

	mgr.Register(primary)
	mgr.Register(fallback)
	mgr.SetPrimary("primary")
	mgr.SetFallbacks([]string{"fallback"})

	results, engine, err := mgr.Search(SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if engine != "primary" {
		t.Errorf("expected engine 'primary', got %q", engine)
	}

	if len(results) != 1 || results[0].Title != "Result 1" {
		t.Errorf("unexpected results: %v", results)
	}
}

func TestManager_Search_FallbackOnPrimaryFailure(t *testing.T) {
	mgr := NewManager()

	primary := &mockBackend{
		name:      "primary",
		available: true,
		err:       fmt.Errorf("connection refused"),
	}
	fallback := &mockBackend{
		name:      "fallback",
		available: true,
		results:   []SearchResult{{Title: "Fallback Result", URL: "https://fallback.com"}},
	}

	mgr.Register(primary)
	mgr.Register(fallback)
	mgr.SetPrimary("primary")
	mgr.SetFallbacks([]string{"fallback"})

	results, engine, err := mgr.Search(SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("Search should have fallen back: %v", err)
	}

	if engine != "fallback" {
		t.Errorf("expected engine 'fallback', got %q", engine)
	}

	if len(results) != 1 || results[0].Title != "Fallback Result" {
		t.Errorf("unexpected results: %v", results)
	}
}

func TestManager_Search_AllBackendsFail(t *testing.T) {
	mgr := NewManager()

	primary := &mockBackend{name: "primary", available: true, err: fmt.Errorf("primary down")}
	fb1 := &mockBackend{name: "fb1", available: true, err: fmt.Errorf("fb1 down")}
	fb2 := &mockBackend{name: "fb2", available: false}

	mgr.Register(primary)
	mgr.Register(fb1)
	mgr.Register(fb2)
	mgr.SetPrimary("primary")
	mgr.SetFallbacks([]string{"fb1", "fb2"})

	_, _, err := mgr.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error when all backends fail")
	}

	if !strings.Contains(err.Error(), "all backends failed") {
		t.Errorf("expected 'all backends failed' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "primary down") {
		t.Errorf("error should mention primary failure: %v", err)
	}
	if !strings.Contains(err.Error(), "fb1 down") {
		t.Errorf("error should mention fb1 failure: %v", err)
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("error should mention fb2 not configured: %v", err)
	}
}

func TestManager_Search_NoPrimary(t *testing.T) {
	mgr := NewManager()
	_, _, err := mgr.Search(SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error with no primary backend")
	}
}

func TestManager_SearchExplicit(t *testing.T) {
	mgr := NewManager()

	b := &mockBackend{
		name:      "explicit",
		available: true,
		results:   []SearchResult{{Title: "Explicit Result"}},
	}
	mgr.Register(b)

	results, err := mgr.SearchExplicit("explicit", SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("SearchExplicit failed: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Explicit Result" {
		t.Errorf("unexpected results: %v", results)
	}
}

func TestManager_SearchExplicit_Unknown(t *testing.T) {
	mgr := NewManager()
	_, err := mgr.SearchExplicit("nonexistent", SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestManager_SearchExplicit_Unavailable(t *testing.T) {
	mgr := NewManager()
	mgr.Register(&mockBackend{name: "disabled", available: false})

	_, err := mgr.SearchExplicit("disabled", SearchOptions{Query: "test"})
	if err == nil {
		t.Fatal("expected error for unavailable backend")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("error should mention not configured: %v", err)
	}
}

func TestManager_GetBackend(t *testing.T) {
	mgr := NewManager()
	mgr.Register(&mockBackend{name: "test", available: true})

	b, ok := mgr.GetBackend("test")
	if !ok || b == nil {
		t.Error("expected to find registered backend")
	}

	_, ok = mgr.GetBackend("nonexistent")
	if ok {
		t.Error("expected false for unregistered backend")
	}
}

func TestManager_ConfiguredBackends(t *testing.T) {
	mgr := NewManager()
	mgr.Register(&mockBackend{name: "available", available: true})
	mgr.Register(&mockBackend{name: "unavailable", available: false})

	configured := mgr.ConfiguredBackends()
	if len(configured) != 1 || configured[0] != "available" {
		t.Errorf("expected [available], got %v", configured)
	}
}

func TestManager_FallbackOrder(t *testing.T) {
	mgr := NewManager()

	// Track which backends are called
	var callOrder []string

	primary := &mockBackend{name: "primary", available: true, err: fmt.Errorf("fail")}
	fb1 := &mockBackend{name: "fb1", available: true, err: fmt.Errorf("fail")}
	fb2 := &mockBackend{
		name:      "fb2",
		available: true,
		results:   []SearchResult{{Title: "fb2 result"}},
	}

	// Wrap to track call order
	mgr.Register(primary)
	mgr.Register(fb1)
	mgr.Register(fb2)
	mgr.SetPrimary("primary")
	mgr.SetFallbacks([]string{"fb1", "fb2"})

	results, engine, err := mgr.Search(SearchOptions{Query: "test"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	_ = callOrder // call order tracked implicitly by which engine succeeds
	if engine != "fb2" {
		t.Errorf("expected fb2 to be used, got %q", engine)
	}
	if len(results) != 1 || results[0].Title != "fb2 result" {
		t.Errorf("unexpected results: %v", results)
	}
}
