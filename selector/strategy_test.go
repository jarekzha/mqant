package selector

import (
	"testing"

	"github.com/jarekzha/mqant/registry"
)

func TestStrategies(t *testing.T) {
	testData := []*registry.Service{
		{
			Name:    "test1",
			Version: "latest",
			Nodes: []*registry.Node{
				{
					InstanceID: "test1-1",
					Address:    "10.0.0.1",
					Port:       1001,
				}, {
					InstanceID: "test1-2",
					Address:    "10.0.0.2",
					Port:       1002,
				},
			},
		}, {
			Name:    "test1",
			Version: "default",
			Nodes: []*registry.Node{
				{
					InstanceID: "test1-3",
					Address:    "10.0.0.3",
					Port:       1003,
				}, {
					InstanceID: "test1-4",
					Address:    "10.0.0.4",
					Port:       1004,
				},
			},
		},
	}

	for name, strategy := range map[string]Strategy{"random": Random, "roundrobin": RoundRobin} {
		next := strategy(testData)
		counts := make(map[string]int)

		for i := 0; i < 100; i++ {
			node, err := next()
			if err != nil {
				t.Fatal(err)
			}
			counts[node.InstanceID]++
		}

		t.Logf("%s: %+v\n", name, counts)
	}
}
