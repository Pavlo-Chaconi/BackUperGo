//go:build windows

package agent

import "golang.org/x/sys/windows/svc"

type serviceHandler struct {
	opts Options
}

func RunService(opts Options) error {
	return svc.Run("BackUperAgent", &serviceHandler{opts: opts})
}

func (h *serviceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	stop := make(chan struct{})

	s <- svc.Status{State: svc.StartPending}
	go runPolling(normalizeOptions(h.opts), stop)
	s <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			s <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			close(stop)
			s <- svc.Status{State: svc.StopPending}
			return false, 0
		default:
			continue
		}
	}
	return false, 0
}
