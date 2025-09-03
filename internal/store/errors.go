package store

import "errors"

// Common store errors
var (
	// ErrKeyNotFound is returned when a key is not found in the store
	ErrKeyNotFound = errors.New("key not found")
	
	// ErrStoreNotInitialized is returned when the store is not initialized
	ErrStoreNotInitialized = errors.New("store not initialized")
	
	// ErrStoreConnectionFailed is returned when connection to store fails
	ErrStoreConnectionFailed = errors.New("store connection failed")
	
	// ErrInvalidKey is returned when an invalid key is provided
	ErrInvalidKey = errors.New("invalid key")
	
	// ErrInvalidValue is returned when an invalid value is provided
	ErrInvalidValue = errors.New("invalid value")
	
	// ErrWatcherNotFound is returned when a watcher is not found
	ErrWatcherNotFound = errors.New("watcher not found")
	
	// ErrWatcherAlreadyExists is returned when a watcher already exists
	ErrWatcherAlreadyExists = errors.New("watcher already exists")
)

// IsKeyNotFoundError checks if the error is a key not found error
func IsKeyNotFoundError(err error) bool {
	return errors.Is(err, ErrKeyNotFound)
}

// IsConnectionError checks if the error is a connection error
func IsConnectionError(err error) bool {
	return errors.Is(err, ErrStoreConnectionFailed)
}
