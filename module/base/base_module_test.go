package basemodule

import (
	"context"
	"errors"
	"testing"

	"github.com/jarekzha/mqant/server"
	"github.com/jarekzha/mqant/service"
)

type readinessServer struct {
	server.Server
	registerErr error
	registered  bool
	stopped     bool
}

func (s *readinessServer) ID() string         { return "Activity@Activity" }
func (s *readinessServer) InstanceID() string { return "Activity@Activity#test" }
func (s *readinessServer) Start() error       { return nil }
func (s *readinessServer) ServiceRegister() error {
	if s.registerErr != nil {
		return s.registerErr
	}
	s.registered = true
	return nil
}
func (s *readinessServer) ServiceDeregister() error { s.registered = false; return nil }
func (s *readinessServer) Stop() error              { s.stopped = true; return nil }
func (s *readinessServer) OnDestroy() error         { return s.Stop() }

func TestBaseModulePublishesSynchronouslyAndCleansUp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := &readinessServer{}
	m := &BaseModule{service: service.NewService(service.Server(server), service.Context(ctx)), exit: cancel, serviceStopped: make(chan bool)}
	if server.registered {
		t.Fatal("service registered before OnReady")
	}
	m.OnReady()
	if !server.registered || !m.serviceStarted {
		t.Fatal("OnReady returned before registration")
	}
	m.OnDestroy()
	if server.registered || !server.stopped {
		t.Fatal("OnDestroy did not stop registration and RPC")
	}
}

func TestBaseModuleRegistrationFailurePanics(t *testing.T) {
	server := &readinessServer{registerErr: errors.New("Consul unavailable")}
	m := &BaseModule{service: service.NewService(service.Server(server))}
	defer func() {
		if recover() == nil {
			t.Fatal("registration failure did not panic")
		}
		if m.serviceStarted || server.registered {
			t.Fatal("failed registration marked module ready")
		}
	}()
	m.OnReady()
}
