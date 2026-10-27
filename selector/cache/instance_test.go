package cache

import (
	"testing"
	"time"

	"github.com/jarekzha/mqant/registry"
	"github.com/jarekzha/mqant/selector"
)

func TestInstanceDeleteEvents(t *testing.T) {
	for _, tc := range []struct {
		name        string
		event       *registry.Service
		wantDeleted int
		wantNodes   int
	}{
		{"one instance", &registry.Service{Name: "Gate", Version: "v1", Nodes: []*registry.Node{{InstanceID: "Gate@Gate001#A"}}}, 1, 2},
		{"delete without nodes retains service-wide scope", &registry.Service{Name: "Gate", Version: "v1"}, 3, 0},
		{"whole service without nodes or version", &registry.Service{Name: "Gate"}, 3, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deleted := map[string]bool{}
			c := &cacheSelector{
				so:   selector.Options{Watcher: func(node *registry.Node) { deleted[node.InstanceID] = true }},
				ttls: make(map[string]time.Time),
				cache: map[string][]*registry.Service{"Gate": {
					{Name: "Gate", Version: "v1", Nodes: []*registry.Node{{InstanceID: "Gate@Gate001#A"}, {InstanceID: "Gate@Gate001#B"}}},
					{Name: "Gate", Version: "v2", Nodes: []*registry.Node{{InstanceID: "Gate@Gate001#C"}}},
				}},
			}
			c.update(&registry.Result{Action: "delete", Service: tc.event})
			if len(deleted) != tc.wantDeleted || !deleted["Gate@Gate001#A"] {
				t.Fatalf("wrong invalidated instances: %v", deleted)
			}
			nodes := 0
			for _, service := range c.cache["Gate"] {
				nodes += len(service.Nodes)
			}
			if nodes != tc.wantNodes {
				t.Fatalf("remaining nodes = %d, want %d", nodes, tc.wantNodes)
			}
		})
	}
}
