package selector

import (
	"testing"

	"github.com/jarekzha/mqant/registry"
	"github.com/jarekzha/mqant/registry/mock"
)

func TestLatestCandidatesBeforeCustomStrategyAndFilters(t *testing.T) {
	reg := mock.NewRegistry()
	reg.Register(&registry.Service{Name: "Gate", Version: "old", Nodes: []*registry.Node{{ServerID: "Gate@1", InstanceID: "A", StartedAt: 1}}})
	reg.Register(&registry.Service{Name: "Gate", Version: "new", Nodes: []*registry.Node{{ServerID: "Gate@1", InstanceID: "B", StartedAt: 2}, {ServerID: "Gate@2", InstanceID: "C", StartedAt: 1}}})
	seen := false
	sel := NewSelector(Registry(reg), SetStrategy(func(services []*registry.Service) Next {
		seen = true
		if len(services) != 1 || len(services[0].Nodes) != 2 || services[0].Version != "new" {
			t.Fatalf("strategy saw stale nodes: %+v", services)
		}
		return Random(services)
	}))
	next, err := sel.Select("Gate")
	if err != nil || !seen {
		t.Fatalf("custom strategy: %v", err)
	}
	for i := 0; i < 20; i++ {
		node, err := next()
		if err != nil || node.InstanceID == "A" {
			t.Fatal("selected stale instance")
		}
	}
	if _, err := sel.Select("Gate", WithFilter(FilterVersion("old"))); err != ErrNoneAvailable {
		t.Fatalf("filter revived old version: %v", err)
	}
	raw, _ := sel.GetService("Gate")
	if len(raw) != 2 {
		t.Fatal("selection mutated registry")
	}
}
