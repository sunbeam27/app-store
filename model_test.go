package app_store

import (
	"errors"
	"testing"
)

func TestErrorResponse(t *testing.T) {
	e := &ErrorResponse{
		ErrorCode:    4000,
		ErrorMessage: "test error",
	}

	expected := "error code: 4000, error message: test error"
	if e.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, e.Error())
	}
}

func TestUnwrapError(t *testing.T) {
	e := &ErrorResponse{
		ErrorCode:    4000,
		ErrorMessage: "test error",
	}

	unwrapped := UnwrapError(e)
	if unwrapped == nil {
		t.Fatal("expected unwrapped error, got nil")
	}
	if unwrapped.ErrorCode != 4000 {
		t.Errorf("expected error code 4000, got %d", unwrapped.ErrorCode)
	}
}

func TestUnwrapErrorNonMatch(t *testing.T) {
	err := errors.New("regular error")
	unwrapped := UnwrapError(err)
	if unwrapped != nil {
		t.Error("expected nil for non-ErrorResponse")
	}
}
