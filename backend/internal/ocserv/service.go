package ocserv

import "context"

// ServiceStatus represents the runtime state of a systemd service.
type ServiceStatus int

const (
	ServiceActive   ServiceStatus = iota // service is running
	ServiceInactive                       // service is stopped
	ServiceFailed                         // service unit has failed
	ServiceNotFound                       // service unit does not exist
)

// ServiceManager manages a systemd service lifecycle.
type ServiceManager interface {
	Status(ctx context.Context) (ServiceStatus, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	IsEnabled(ctx context.Context) (bool, error)
}
