//go:build !windows

package platform

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func IsWindowsService() bool { return false }

func RunService(run func(context.Context)) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	run(ctx)
	return nil
}
