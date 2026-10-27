package server

import (
	"testing"

	"github.com/jarekzha/mqant/registry/mock"
)

func TestRPCServerInstanceIdentity(t *testing.T) {
	reg := mock.NewRegistry()
	first := NewServer(Name("Gate"), ID("Gate001"), Address("127.0.0.1:9000"), Registry(reg))
	second := NewServer(Name("Gate"), ID("Gate001"), Address("127.0.0.1:9000"), Registry(reg))

	if first.InstanceID() == "" || second.InstanceID() == "" {
		t.Fatal("service instances must have nonempty instance IDs")
	}
	if first.InstanceID() == second.InstanceID() {
		t.Fatal("two instances with the same logical ID must have different instance IDs")
	}

	for _, srv := range []Server{first, second} {
		instanceID := srv.InstanceID()
		for i := 0; i < 2; i++ {
			if err := srv.ServiceRegister(); err != nil {
				t.Fatal(err)
			}
			if srv.ID() != "Gate@Gate001" {
				t.Fatalf("registration changed logical ID: %q", srv.ID())
			}
			if srv.InstanceID() != instanceID {
				t.Fatal("registration or renewal changed instance ID")
			}
		}
		if err := srv.ServiceDeregister(); err != nil {
			t.Fatal(err)
		}
		if srv.InstanceID() != instanceID {
			t.Fatal("deregistration changed instance ID")
		}
	}
}
