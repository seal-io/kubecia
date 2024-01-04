package cache

import (
	"context"
	"errors"
	"io"
)

var (
	ErrEntryNotFound = errors.New("entry is not found")
	ErrEntryTooBig   = errors.New("entry is too big")
)

// Cache holds the action of caching.
type Cache interface {
	io.Closer

	Name() string

	// Set saves entry with the given key,
	// it returns an ErrEntryTooBig when entry is too big.
	Set(ctx context.Context, key string, entry []byte) error

	// Delete removes the given key.
	Delete(ctx context.Context, key string) ([]byte, error)

	// Get reads entry for the given key,
	// it returns an ErrEntryNotFound when no entry exists for the given key.
	Get(ctx context.Context, key string) ([]byte, error)
}
