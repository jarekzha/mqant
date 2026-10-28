package cache

import (
	"testing"

	"github.com/jarekzha/mqant/registry"
	"github.com/jarekzha/mqant/registry/mock"
	"github.com/jarekzha/mqant/selector"
)

func TestLatestSelectionAndRegistrationReset(t *testing.T) {
	reg := mock.NewRegistry()
	a := &registry.Service{Name: "Gate", Version: "v1", Nodes: []*registry.Node{{ServerID: "Gate@1", InstanceID: "A", StartedAt: 1}}}
	b := &registry.Service{Name: "Gate", Version: "v1", Nodes: []*registry.Node{{ServerID: "Gate@1", InstanceID: "B", StartedAt: 2}}}
	reg.Register(a)
	deleted := 0
	strategyCalls := 0
	c := NewSelector(selector.Registry(reg), selector.SetWatcher(func(*registry.Node) { deleted++ }), selector.SetStrategy(func(services []*registry.Service) selector.Next {
		strategyCalls++
		return selector.Random(services)
	})).(*cacheSelector)
	c.watched["Gate"] = true // 下方手动投递事件，避免后台 Watch 影响测试结果
	defer c.Close()
	next, err := c.Select("Gate")
	if err != nil {
		t.Fatal(err)
	}
	node, _ := next()
	if node.InstanceID != "A" {
		t.Fatal("initial lookup")
	}
	reg.Register(b)
	c.Reset("Gate") // 模拟 OnReady 在新实例注册成功后清除候选缓存。
	next, err = c.Select("Gate")
	if err != nil {
		t.Fatal(err)
	}
	node, _ = next()
	if node.InstanceID != "B" || node.StartedAt != 2 {
		t.Fatalf("stale registration snapshot: %+v", node)
	}
	raw, _ := c.GetService("Gate")
	if len(raw[0].Nodes) != 2 || deleted != 0 {
		t.Fatal("reset removed raw nodes or invalidated App bindings")
	}
	c.update(&registry.Result{Action: "delete", Service: a})
	next, err = c.Select("Gate")
	if err != nil {
		t.Fatal(err)
	}
	node, _ = next()
	if node.InstanceID != "B" || deleted != 1 || strategyCalls != 3 {
		t.Fatal("old deletion affected new instance")
	}
}
