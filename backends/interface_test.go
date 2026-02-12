package backends

import (
	"fmt"
	"testing"
)

func TestBackendError_Error(t *testing.T) {
	err := &BackendError{
		Backend: "test",
		Err:     fmt.Errorf("something went wrong"),
		Code:    ErrCodeNetwork,
	}

	want := "test backend: something went wrong"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestBackendError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	err := &BackendError{
		Backend: "test",
		Err:     inner,
		Code:    ErrCodeAuth,
	}

	if err.Unwrap() != inner {
		t.Errorf("Unwrap() returned different error")
	}
}

func TestBackendError_Codes(t *testing.T) {
	// Verify error code constants are distinct
	codes := []int{ErrCodeUnavailable, ErrCodeNetwork, ErrCodeAuth, ErrCodeRateLimit, ErrCodeInvalidResponse}
	seen := make(map[int]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("duplicate error code: %d", code)
		}
		seen[code] = true
	}
}
