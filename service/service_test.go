package service

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jarekzha/mqant/server"
)

type lifecycleServer struct {
	server.Server
	mu          sync.Mutex
	events      []string
	registerErr error
}

func (s *lifecycleServer) record(event string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}
func (s *lifecycleServer) Start() error             { s.record("start"); return nil }
func (s *lifecycleServer) Stop() error              { s.record("stop"); return nil }
func (s *lifecycleServer) ServiceRegister() error   { s.record("register"); return s.registerErr }
func (s *lifecycleServer) ServiceDeregister() error { s.record("deregister"); return nil }

func TestStartThenWaitDoesNotRegisterTwice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := &lifecycleServer{}
	s := NewService(Server(server), Context(ctx), RegisterInterval(time.Hour))
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(server.events, []string{"start", "register"}) {
		t.Fatalf("Start did not synchronously register: %v", server.events)
	}
	cancel()
	if err := s.Wait(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(server.events, []string{"start", "register", "deregister", "stop"}) {
		t.Fatalf("Wait repeated registration or missed cleanup: %v", server.events)
	}
}

func TestRunDoesNotWaitAfterRegistrationFailure(t *testing.T) {
	failure := errors.New("Consul unavailable")
	server := &lifecycleServer{registerErr: failure}
	s := NewService(Server(server))
	if err := s.Run(); err != failure {
		t.Fatalf("registration failure lost: %v", err)
	}
	if !reflect.DeepEqual(server.events, []string{"start", "register"}) {
		t.Fatalf("continued after failed registration: %v", server.events)
	}
}

type renewalServer struct {
	lifecycleServer
	calls        int
	entered      chan struct{}
	release      chan struct{}
	deregistered chan struct{}
}

func (s *renewalServer) ServiceRegister() error {
	s.calls++
	if s.calls == 2 {
		close(s.entered)
		<-s.release
	}
	return nil
}
func (s *renewalServer) ServiceDeregister() error { close(s.deregistered); return nil }

func TestWaitFinishesRenewalBeforeDeregistering(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := &renewalServer{entered: make(chan struct{}), release: make(chan struct{}), deregistered: make(chan struct{})}
	defer close(server.release)
	s := NewService(Server(server), Context(ctx), RegisterInterval(time.Millisecond))
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Wait() }()
	select {
	case <-server.entered:
	case <-time.After(time.Second):
		t.Fatal("renewal did not start")
	}
	cancel()
	select {
	case <-server.deregistered:
		t.Fatal("deregistered while renewal could still re-register the service")
	case <-time.After(20 * time.Millisecond):
	}
	// 放行续期后，等待循环退出，再注销；此后不能再有注册。
	server.release <- struct{}{}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not stop after renewal finished")
	}
	select {
	case <-server.deregistered:
	default:
		t.Fatal("Wait returned before deregistration")
	}
}
