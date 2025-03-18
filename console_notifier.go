package dca

import (
	"context"
	"fmt"
	"io"
	"os"
)

type Notifier interface {
	Notify(_ context.Context, response ExecuteOrderResponse) (err error)
	NotifyFailure(_ context.Context, f error) (err error)
}

var _ Notifier = (*ConsoleNotifier)(nil)

// ConsoleNotifier is a stub implementation of a notifier used for local development.
type ConsoleNotifier struct {
	Destination io.Writer
}

// NewConsoleNotifier creates a ConsoleNotifier configured to write output to stdout.
func NewConsoleNotifier() *ConsoleNotifier {
	return &ConsoleNotifier{
		Destination: os.Stdout,
	}
}

func (n *ConsoleNotifier) Notify(_ context.Context, response ExecuteOrderResponse) (err error) {
	defer AddErr(&err, "ConsoleNotifier.Notify")
	msg := fmt.Sprintf("bought %.8f bitcoin for %.2f (paid %.2f fee)", response.VolumePurchased, response.Cost, response.Fee)
	if _, err = fmt.Fprintf(n.Destination, msg); err != nil {
		return err
	}
	return nil
}

func (n *ConsoleNotifier) NotifyFailure(_ context.Context, f error) (err error) {
	defer AddErr(&err, "ConsoleNotifier.NotifyFailure")
	if _, err = fmt.Fprintf(n.Destination, fmt.Sprintf("failed to buy bitcoin: %v", f)); err != nil {
		return err
	}
	return nil
}
