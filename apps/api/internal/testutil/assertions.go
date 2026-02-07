package testutil

import (
	"errors"
	"testing"

	apperrors "kuberan/internal/errors"
)

// AssertAppError checks that err is an *AppError with the expected error code.
func AssertAppError(t *testing.T, err error, expectedCode string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected AppError with code %q, got nil", expectedCode)
	}

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}

	if appErr.Code != expectedCode {
		t.Errorf("expected error code %q, got %q (message: %s)", expectedCode, appErr.Code, appErr.Message)
	}
}

// AssertNoError fails the test if err is not nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
