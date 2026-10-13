// Package model defines App Store Connect OpenAPI wire models.
package model

// Ptr constructs an optional value while preserving zero and false values.
func Ptr[T any](value T) *T { return &value }
