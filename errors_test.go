package structhttp

import (
	"errors"
	"testing"
)

func TestError(t *testing.T) {
	wrapped := errors.New("test error")
	err := NewError(500, wrapped)

	if !errors.Is(err, wrapped) {
		t.Errorf("expected error to wrap %v", wrapped)
	}
}
