package store

import "time"

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTimeValue(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func nullableTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}
