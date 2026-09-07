package store

import "errors"

var (
	ErrNotFound             = errors.New("store: not found")
	ErrProjectNameMismatch  = errors.New("store: root path already registered under a different name")
	ErrStatusChangeRejected = errors.New("store: status cannot be changed through an update")
	ErrIllegalTransition    = errors.New("store: illegal status transition")
)
