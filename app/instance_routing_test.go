package app

import (
	"testing"

	"github.com/jarekzha/mqant/module"
	"github.com/jarekzha/mqant/registry"
	mqrpc "github.com/jarekzha/mqant/rpc"
	"github.com/jarekzha/mqant/selector"
)

type instanceSelector struct {
	selector.Selector
	nodes []*registry.Node
}

func (s *instanceSelector) GetService(name string) ([]*registry.Service, error) {
	return []*registry.Service{{Name: name, Nodes: s.nodes}}, nil
}

type instanceRPC struct {
	mqrpc.RPCClient
	closed int
}

func (c *instanceRPC) Done() error {
	c.closed++
	return nil
}

type instanceSession struct {
	module.ServerSession
	node *registry.Node
	rpc  *instanceRPC
}

func (s *instanceSession) GetID() string           { return s.node.ServerID }
func (s *instanceSession) GetInstanceID() string   { return s.node.InstanceID }
func (s *instanceSession) GetNode() *registry.Node { return s.node }
func (s *instanceSession) GetRPC() mqrpc.RPCClient { return s.rpc }

func TestInstanceRoutingAndOldInstanceDeletion(t *testing.T) {
	const logicalID = "Gate@Gate001"
	first := &instanceSession{node: &registry.Node{
		InstanceID: logicalID + "#A", ServerID: logicalID, StartedAt: 20,
	}, rpc: &instanceRPC{}}
	second := &instanceSession{node: &registry.Node{
		InstanceID: logicalID + "#B", ServerID: logicalID, StartedAt: 10,
	}, rpc: &instanceRPC{}}
	sel := &instanceSelector{nodes: []*registry.Node{first.node, second.node}}
	app := &DefaultApp{opts: module.Options{Selector: sel}}
	app.serverList.Store(first.GetInstanceID(), first)
	app.serverList.Store(second.GetInstanceID(), second)

	got, err := app.GetServerByID(logicalID)
	if err != nil || got != first {
		t.Fatalf("initial logical lookup: got %v, err %v", got, err)
	}
	sel.nodes = []*registry.Node{second.node, first.node}
	got, err = app.GetServerByID(logicalID)
	if err != nil || got != first {
		t.Fatal("logical lookup switched a live instance")
	}
	got, err = app.GetServerByInstanceID(second.GetInstanceID())
	if err != nil || got != second {
		t.Fatal("exact lookup did not reach the second instance")
	}
	if _, err := app.GetServerByInstanceID(logicalID); err == nil {
		t.Fatal("exact lookup accepted a logical ID alias")
	}
	if got := app.GetServersByType("Gate"); len(got) != 2 {
		t.Fatal("discovery merged two instances with the same logical ID")
	}

	if _, ok := app.serverList.Load(logicalID); ok {
		t.Fatal("logical ID leaked into the instance cache")
	}
	if route, ok := app.logicalRoutes.Load(logicalID); !ok || route != first.GetInstanceID() {
		t.Fatal("logical route does not point to the selected instance ID")
	}
	rediscovered := *first.node
	rediscovered.Address = "changed-address"
	got, err = app.getServerSession("Gate", &rediscovered)
	if err != nil || got != first || got.GetNode() != first.node {
		t.Fatal("rediscovery replaced an existing proxy's immutable node")
	}

	sel.nodes = []*registry.Node{second.node}
	app.Watcher(first.node)
	if first.rpc.closed != 1 || second.rpc.closed != 0 {
		t.Fatal("deletion closed the wrong RPC client")
	}
	got, err = app.GetServerByID(logicalID)
	if err != nil || got != second {
		t.Fatal("logical alias was not rebuilt after its old instance left")
	}
	app.Watcher(first.node)
	got, err = app.GetServerByID(logicalID)
	if err != nil || got != second || second.rpc.closed != 0 {
		t.Fatal("a repeated old deletion removed the new instance alias")
	}
	if _, err := app.GetServerByInstanceID(first.GetInstanceID()); err == nil {
		t.Fatal("exact lookup fell back from a missing instance to the new one")
	}
}

// 即使发现更新的实例，已有逻辑绑定仍保持不变。
func TestLatestInitialRouteAndStickyReplacement(t *testing.T) {
	const id = "Zone@Zone001"
	a := &instanceSession{node: &registry.Node{ServerID: id, InstanceID: id + "#A", StartedAt: 10}, rpc: &instanceRPC{}}
	b := &instanceSession{node: &registry.Node{ServerID: id, InstanceID: id + "#B", StartedAt: 20}, rpc: &instanceRPC{}}
	sel := &instanceSelector{nodes: []*registry.Node{a.node}}
	app := &DefaultApp{opts: module.Options{Selector: sel}}
	app.serverList.Store(a.GetInstanceID(), a)
	app.serverList.Store(b.GetInstanceID(), b)
	got, err := app.GetServerByID(id)
	if err != nil || got != a {
		t.Fatalf("initial A: %v, %v", got, err)
	}
	sel.nodes = []*registry.Node{a.node, b.node}
	got, err = app.GetServerByID(id)
	if err != nil || got != a {
		t.Fatal("new discovery replaced a live binding")
	}
	app.logicalRoutes.Delete(id)
	got, err = app.GetServerByID(id)
	if err != nil || got != b {
		t.Fatalf("unbound route did not select latest B: %v, %v", got, err)
	}
	exact, err := app.GetServerByInstanceID(a.GetInstanceID())
	if err != nil || exact != a {
		t.Fatal("latest selection broke exact old lookup")
	}
	if got := app.GetServersByType("Zone"); len(got) != 2 {
		t.Fatal("latest selection removed enumeration nodes")
	}
	app.Watcher(a.node)
	got, err = app.GetServerByID(id)
	if err != nil || got != b || b.rpc.closed != 0 {
		t.Fatal("old deletion damaged B binding")
	}
	// 模拟另一个已绑定 A 的调用者，仅在 A 被删除后重新选择。
	app.logicalRoutes.Store(id, a.GetInstanceID())
	app.serverList.Store(a.GetInstanceID(), a)
	sel.nodes = []*registry.Node{b.node}
	app.Watcher(a.node)
	got, err = app.GetServerByID(id)
	if err != nil || got != b {
		t.Fatalf("did not replace deleted A: %v, %v", got, err)
	}
}
