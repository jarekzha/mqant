package registry

import "testing"

func TestLatestServicesAcrossVersionsAndLegacyNodes(t *testing.T) {
	old := &Node{ServerID: "Zone@1", InstanceID: "old", StartedAt: 10}
	latest := &Node{ServerID: "Zone@1", InstanceID: "new", StartedAt: 20}
	other := &Node{ServerID: "Zone@2", InstanceID: "other", StartedAt: 1}
	legacy := &Node{InstanceID: "legacy"}
	services := []*Service{
		{Name: "Zone", Version: "old", Nodes: []*Node{old, other, legacy}},
		{Name: "Zone", Version: "new", Nodes: []*Node{latest}},
	}
	for _, input := range [][]*Service{services, {services[1], services[0]}} {
		got := LatestServices(input)
		nodes := map[string]bool{}
		for _, service := range got {
			for _, node := range service.Nodes {
				nodes[node.InstanceID] = true
			}
		}
		if len(nodes) != 3 || nodes["old"] || !nodes["new"] || !nodes["other"] || !nodes["legacy"] {
			t.Fatalf("wrong latest candidates: %v", nodes)
		}
		got[0].Nodes = nil
		if len(services[0].Nodes) != 3 || len(services[1].Nodes) != 1 {
			t.Fatal("mutated raw discovery snapshot")
		}
	}
}

func TestLatestServicesMissingTimeAndTie(t *testing.T) {
	for _, nodes := range [][]*Node{
		{{ServerID: "Gate@1", InstanceID: "z"}, {ServerID: "Gate@1", InstanceID: "a", StartedAt: 1}},
		{{ServerID: "Gate@1", InstanceID: "a", StartedAt: 1}, {ServerID: "Gate@1", InstanceID: "b", StartedAt: 1}},
	} {
		want := nodes[1]
		for i := 0; i < 2; i++ {
			got := LatestServices([]*Service{{Name: "Gate", Nodes: nodes}})
			if len(got) != 1 || len(got[0].Nodes) != 1 || got[0].Nodes[0] != want {
				t.Fatalf("unstable winner: %+v", got)
			}
			nodes[0], nodes[1] = nodes[1], nodes[0]
		}
	}
}

func TestDecodeStartedAtCompatibility(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int64
	}{{"", 0}, {"123", 123}, {"invalid", 0}, {"-1", 0}, {"99999999999999999999", 0}} {
		if got := decodeStartedAt(map[string]string{consulStartedAtKey: tc.value}); got != tc.want {
			t.Fatalf("%q: %d", tc.value, got)
		}
	}
}
