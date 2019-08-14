package alohartc

import "errors"

var (
	errNotFound       = errors.New("Not found")
	errNotImplemented = errors.New("Not implemented") // "to do" items
	errNotSupported   = errors.New("Not supported")   // "can't do" items
)
