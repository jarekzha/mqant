package server

import (
	"testing"

	"github.com/jarekzha/mqant/registry/mock"
)

func TestRPCServerInstanceIdentity(t *testing.T) {
	reg := mock.NewRegistry()
	first := NewServer(Name("Gate"), ID("Gate001"), Address("127.0.0.1:9000"), Registry(reg))
	second := NewServer(Name("Gate"), ID("Gate001"), Address("127.0.0.1:9000"), Registry(reg))

	if first.ID() != "Gate@Gate001" || second.ID() != "Gate@Gate001" {
		t.Fatal("logical ID must be available before registration")
	}

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
	}
	services, err := reg.GetService("Gate")
	if err != nil || len(services) != 1 || len(services[0].Nodes) != 2 {
		t.Fatalf("same logical ID should register two instances: %v, %v", services, err)
	}
	seen := map[string]bool{}
	for _, node := range services[0].Nodes {
		seen[node.InstanceID] = true
		if node.ServerID != "Gate@Gate001" {
			t.Fatalf("logical ID missing from registration: %v", node.Metadata)
		}
	}
	if !seen[first.InstanceID()] || !seen[second.InstanceID()] {
		t.Fatal("registration IDs must be the instance IDs")
	}
	instanceID := first.InstanceID()
	if err := first.ServiceDeregister(); err != nil {
		t.Fatal(err)
	}
	if first.InstanceID() != instanceID {
		t.Fatal("deregistration changed instance ID")
	}
	services, err = reg.GetService("Gate")
	if err != nil || len(services) != 1 || len(services[0].Nodes) != 1 ||
		services[0].Nodes[0].InstanceID != second.InstanceID() {
		t.Fatalf("old instance removed the new registration: %v, %v", services, err)
	}
	if err := second.ServiceDeregister(); err != nil {
		t.Fatal(err)
	}
}
