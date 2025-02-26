package dca

import (
	"context"
	"fmt"
)

var (
	Version = "dev"
	Commit  = "dev"
	Date    = "dev"
)

type Order struct {
	AmountInCents int `json:"amount_in_cents"`
}

type Provider interface {
	PlaceOrder(ctx context.Context, order Order) error
}

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
