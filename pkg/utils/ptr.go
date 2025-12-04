package utils

import "fmt"

func SafeDeref[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}

func DiffPtr[T comparable](oldVal, newVal *T) bool {
	if oldVal == nil && newVal == nil {
		return false
	}
	if oldVal == nil || newVal == nil {
		return true
	}
	return *oldVal != *newVal
}

func ToPtr[T any](v T) *T {
	return &v
}

func PtrToString(v *uint64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%d", *v)
}
