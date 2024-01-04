package signal

import (
	"context"
	"errors"
	"os"
	"os/signal"

	"k8s.io/klog/v2"
)

var registered = make(chan struct{})

// Context registers for signals and returns a context.
func Context() context.Context {
	close(registered) // Panics when called twice.

	ctx, cancel := context.WithCancelCause(context.Background())

	// Register for signals.
	ch := make(chan os.Signal, len(shutdownSignals))
	signal.Notify(ch, shutdownSignals...)

	// Process signals.
	go func() {
		var shutdown bool

		for s := range ch {
			klog.V(4).Infof("received signal %q", s)

			if shutdown {
				os.Exit(1)
			}

			klog.V(4).Info("exiting")
			cancel(errors.New("received shutdown signal"))
			shutdown = true
		}
	}()

	return ctx
}
