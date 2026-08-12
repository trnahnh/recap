package store

import "errors"

var (
	ErrNotFound            = errors.New("store: not found")
	ErrProjectNameMismatch = errors.New("store: root path already registered under a different name")
)
