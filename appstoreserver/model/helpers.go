// Package model defines the App Store Server Library wire models and enums.
package model

// Ptr constructs an optional value while preserving zero and false values.
func Ptr[T any](value T) *T { return &value }
