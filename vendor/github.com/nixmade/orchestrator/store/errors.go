package store

import "errors"

var (
	// ErrKeyNotFound returns an error if key is not found in store
	ErrKeyNotFound = errors.New("key not found in store")
)
