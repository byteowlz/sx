package backends

import (
	"fmt"
	"strings"
	"time"
)

const (
	SearxngStrategyOrdered         = "ordered"
	SearxngStrategyParallelFastest = "parallel-fastest"
)

// MultiSearxngBackend wraps one or more SearXNG instances and applies a strategy.
type MultiSearxngBackend struct {
	instances []*SearxngBackend
	strategy  string
}

// NewMultiSearxngBackend creates a multi-instance SearXNG backend.
// Invalid/empty URLs are accepted at construction time and filtered by IsAvailable/Search.
func NewMultiSearxngBackend(
	urls []string,
	username, password, httpMethod string,
	timeout time.Duration,
	noVerifySSL, noUserAgent bool,
	strategy string,
) *MultiSearxngBackend {
	instances := make([]*SearxngBackend, 0, len(urls))
	for _, u := range urls {
		instances = append(instances, NewSearxngBackend(
			u,
			username,
			password,
			httpMethod,
			timeout,
			noVerifySSL,
			noUserAgent,
		))
	}

	if strategy == "" {
		strategy = SearxngStrategyOrdered
	}

	return &MultiSearxngBackend{
		instances: instances,
		strategy:  strategy,
	}
}

func (m *MultiSearxngBackend) Name() string {
	return "searxng"
}

func (m *MultiSearxngBackend) IsAvailable() bool {
	for _, instance := range m.instances {
		if instance.IsAvailable() {
			return true
		}
	}
	return false
}

func (m *MultiSearxngBackend) Search(opts SearchOptions) ([]SearchResult, error) {
	available := make([]*SearxngBackend, 0, len(m.instances))
	for _, instance := range m.instances {
		if instance.IsAvailable() {
			available = append(available, instance)
		}
	}

	if len(available) == 0 {
		return nil, &BackendError{
			Backend: m.Name(),
			Err:     fmt.Errorf("no reachable SearXNG instances configured"),
			Code:    ErrCodeUnavailable,
		}
	}

	switch m.strategy {
	case SearxngStrategyParallelFastest:
		return m.searchParallelFastest(available, opts)
	case SearxngStrategyOrdered:
		fallthrough
	default:
		return m.searchOrdered(available, opts)
	}
}

func (m *MultiSearxngBackend) searchOrdered(instances []*SearxngBackend, opts SearchOptions) ([]SearchResult, error) {
	var errs []error
	for _, instance := range instances {
		results, err := instance.Search(opts)
		if err == nil {
			return results, nil
		}
		errs = append(errs, err)
	}

	return nil, &BackendError{
		Backend: m.Name(),
		Err:     fmt.Errorf("all SearXNG instances failed (%d)", len(errs)),
		Code:    ErrCodeNetwork,
	}
}

func (m *MultiSearxngBackend) searchParallelFastest(instances []*SearxngBackend, opts SearchOptions) ([]SearchResult, error) {
	type result struct {
		results []SearchResult
		err     error
	}

	ch := make(chan result, len(instances))

	for _, instance := range instances {
		inst := instance
		go func() {
			results, err := inst.Search(opts)
			ch <- result{results: results, err: err}
		}()
	}

	var errs []error
	for i := 0; i < len(instances); i++ {
		res := <-ch
		if res.err == nil {
			return res.results, nil
		}
		errs = append(errs, res.err)
	}

	return nil, &BackendError{
		Backend: m.Name(),
		Err:     fmt.Errorf("all SearXNG instances failed (%d)", len(errs)),
		Code:    ErrCodeNetwork,
	}
}

func (m *MultiSearxngBackend) Strategy() string {
	return m.strategy
}

func (m *MultiSearxngBackend) InstanceCount() int {
	return len(m.instances)
}

// DeduplicateSearxngURLs removes empty and duplicate URLs while preserving order.
func DeduplicateSearxngURLs(urls []string) []string {
	seen := make(map[string]struct{}, len(urls))
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		trimmed := strings.TrimSpace(u)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
