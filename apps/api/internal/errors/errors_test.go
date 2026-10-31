package errors

import (
	stderrors "errors"
	"fmt"
	"testing"
)

func TestAppErrorIs(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{"same sentinel", ErrNotFound, ErrNotFound, true},
		{"Wrap keeps the code", Wrap(ErrNotFound, fmt.Errorf("db")), ErrNotFound, true},
		{"WithMessage keeps the code", WithMessage(ErrCategoryNotFound, "parent category not found"), ErrCategoryNotFound, true},
		{"wrapped by fmt.Errorf", fmt.Errorf("context: %w", WithMessage(ErrNotFound, "x")), ErrNotFound, true},
		{"different code", ErrNotFound, ErrCategoryNotFound, false},
		{"not an AppError", stderrors.New("boom"), ErrNotFound, false},
		{"internal error still reachable", Wrap(ErrInternalServer, errSentinel), errSentinel, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stderrors.Is(tc.err, tc.target); got != tc.want {
				t.Errorf("errors.Is = %v, want %v", got, tc.want)
			}
		})
	}
}

var errSentinel = stderrors.New("sentinel")
