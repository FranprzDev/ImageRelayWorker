//go:build windows

package platform

import (
	"context"

	"golang.org/x/sys/windows/svc"
)

func IsWindowsService() bool { ok, _ := svc.IsWindowsService(); return ok }

func RunService(run func(context.Context)) error {
	if !IsWindowsService() {
		return nil
	}
	return svc.Run("ImageRelayWorker", &handler{run: run})
}

type handler struct{ run func(context.Context) }

func (h *handler) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { h.run(ctx); close(done) }()
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for request := range requests {
		switch request.Cmd {
		case svc.Interrogate:
			changes <- request.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			cancel()
			<-done
			return false, 0
		default:
			return false, 1
		}
	}
	return false, 0
}
