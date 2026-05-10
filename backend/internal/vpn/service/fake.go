package service

import (
	"context"
	"sync"
)

// FakeServiceManager is a test double for ServiceManager.
// All operations are thread-safe.
type FakeServiceManager struct {
	mu       sync.Mutex
	status   ServiceStatus
	enabled  bool
	err      error
	starts   int
	stops    int
	restarts int
}

// Compile-time interface satisfaction check.
var _ ServiceManager = (*FakeServiceManager)(nil)

// NewFakeServiceManager returns a FakeServiceManager with the given initial status.
// The unit is enabled by default.
func NewFakeServiceManager(status ServiceStatus) *FakeServiceManager {
	return &FakeServiceManager{status: status, enabled: true}
}

// Status returns the configured status.
func (f *FakeServiceManager) Status(ctx context.Context) (ServiceStatus, error) {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status, f.err
}

// Start transitions the status to ServiceActive and increments the start counter.
func (f *FakeServiceManager) Start(ctx context.Context) error {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	if f.err != nil {
		return f.err
	}
	f.status = ServiceActive
	return nil
}

// Stop transitions the status to ServiceInactive and increments the stop counter.
func (f *FakeServiceManager) Stop(ctx context.Context) error {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	if f.err != nil {
		return f.err
	}
	f.status = ServiceInactive
	return nil
}

// Restart transitions the status to ServiceActive and increments the restart counter.
func (f *FakeServiceManager) Restart(ctx context.Context) error {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restarts++
	if f.err != nil {
		return f.err
	}
	f.status = ServiceActive
	return nil
}

// IsEnabled returns the configured enabled state.
func (f *FakeServiceManager) IsEnabled(ctx context.Context) (bool, error) {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.enabled, f.err
}

// SetError configures the error returned by all operations.
func (f *FakeServiceManager) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// SetStatus directly overrides the current status.
func (f *FakeServiceManager) SetStatus(status ServiceStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = status
}

// SetEnabled overrides the enabled state.
func (f *FakeServiceManager) SetEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enabled = enabled
}

// StartCount returns the number of Start calls made.
func (f *FakeServiceManager) StartCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts
}

// StopCount returns the number of Stop calls made.
func (f *FakeServiceManager) StopCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stops
}

// RestartCount returns the number of Restart calls made.
func (f *FakeServiceManager) RestartCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.restarts
}
