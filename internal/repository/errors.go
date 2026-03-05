package repository

import "errors"

// Sentinel errors for repository layer.
// Services check against these using errors.Is() to handle
// specific cases without string matching.
//
// Example:
//
//	user, err := repo.FindByUsername(ctx, username)
//	if errors.Is(err, repository.ErrUserNotFound) {
//	    // handle not found case specifically
//	}
var (
	ErrUserNotFound        = errors.New("user not found")
	ErrApplicationNotFound = errors.New("application not found")
	ErrCustomerNotFound    = errors.New("customer not found")
	ErrSessionNotFound     = errors.New("session not found or expired")
	ErrDuplicateEntry      = errors.New("duplicate entry")
)

// ErrNotFound is a generic not-found sentinel for repositories that don't
// have a domain-specific variant (e.g. config, review actions).
var ErrNotFound = errors.New("record not found")
