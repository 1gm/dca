package dca

import (
	"errors"
	"fmt"
)

// AddErr adds context and creates an opaque error.
// Example use:
//
//	defer AddErr(&err, "LoginUser('%q','******')", user.Name)
func AddErr(err *error, tmpl string, args ...any) {
	if *err != nil {
		*err = fmt.Errorf("%s: %v", fmt.Sprintf(tmpl, args...), *err)
	}
}

// WrapErr adds context and creates an unwrappable error.
// Example use:
//
//	defer WrapErr(&err, "LoginUser('%q','******')", user.Name)
func WrapErr(err *error, tmpl string, args ...any) {
	if *err != nil {
		*err = fmt.Errorf("%s: %w", fmt.Sprintf(tmpl, args...), *err)
	}
}

var (
	// ErrOrderToSmall happens when an order is rejected due to volume being too low
	ErrOrderToSmall = errors.New("order is too small")
	// ErrInvalidAuth occurs when an API credential is invalid
	ErrInvalidAuth = errors.New("invalid auth")
)
