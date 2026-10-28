package defaultrpc

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jarekzha/mqant/module"
	"github.com/jarekzha/mqant/registry"
	mqrpc "github.com/jarekzha/mqant/rpc"
	rpcpb "github.com/jarekzha/mqant/rpc/pb"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/protobuf/proto"
)

type lifecycleApp struct {
	module.App
	conn *nats.Conn
}

func (a *lifecycleApp) Transport() *nats.Conn   { return a.conn }
func (a *lifecycleApp) Options() module.Options { return module.Options{RPCExpired: time.Second} }

type lifecycleSession struct {
	module.ServerSession
	node *registry.Node
}

func (s *lifecycleSession) GetNode() *registry.Node { return s.node }
func (s *lifecycleSession) GetInstanceID() string   { return s.node.InstanceID }

func lifecycleConnection(t *testing.T) *nats.Conn {
	t.Helper()
	server, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go server.Start()
	t.Cleanup(server.Shutdown)
	if !server.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS did not start")
	}
	conn, err := nats.Connect(server.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(conn.Close)
	return conn
}
func lifecycleClient(t *testing.T, conn *nats.Conn) *NatsClient {
	t.Helper()
	client, err := NewNatsClient(&lifecycleApp{conn: conn}, &lifecycleSession{node: &registry.Node{InstanceID: "Test@instance", Address: "lifecycle.requests"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Done() })
	return client
}
func lifecycleCall(id string) *mqrpc.CallInfo {
	return &mqrpc.CallInfo{RPCInfo: &rpcpb.RPCInfo{Cid: id, Fn: "test"}}
}
func lifecycleWait(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("client operation did not finish")
	}
}

func TestNatsClientConcurrentDone(t *testing.T) {
	core, observed := observer.New(zap.ErrorLevel)
	restore := zap.ReplaceGlobals(zap.New(core))
	defer restore()
	defer func() {
		if observed.Len() != 0 {
			t.Errorf("normal close logged errors: %v", observed.All())
		}
	}()
	conn := lifecycleConnection(t)
	for round := 0; round < 50; round++ {
		client := lifecycleClient(t, conn)
		pending := make(chan *rpcpb.ResultInfo, 1)
		if err := client.Call(lifecycleCall("pending"), pending); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		for worker := 0; worker < 8; worker++ {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				<-start
				for i := 0; i < 20; i++ {
					id := fmt.Sprintf("%d-%d", worker, i)
					_ = client.Call(lifecycleCall(id), make(chan *rpcpb.ResultInfo, 1))
					client.Delete(id)
					_ = client.CallNR(lifecycleCall(id))
				}
				_ = client.Done()
			}(worker)
		}
		close(start)
		done := make(chan struct{})
		go func() { _ = client.Done(); wg.Wait(); close(done) }()
		lifecycleWait(t, done)
		select {
		case _, ok := <-pending:
			if ok {
				t.Fatal("pending call received unexpected reply")
			}
		default:
			t.Fatal("pending call was not released")
		}
		if err := client.Call(lifecycleCall("closed"), make(chan *rpcpb.ResultInfo, 1)); err == nil {
			t.Fatal("closed client accepted call")
		}
		if err := client.CallNR(lifecycleCall("closed")); err == nil {
			t.Fatal("closed client accepted notification")
		}
		if n := conn.NumSubscriptions(); n != 0 {
			t.Fatalf("subscriptions leaked after Done: %d", n)
		}
	}
}

func TestNatsClientReplySubscriptionRecovery(t *testing.T) {
	conn := lifecycleConnection(t)
	client := lifecycleClient(t, conn)
	client.cmutex.Lock()
	old := client.subs
	_ = old.Unsubscribe()
	client.cmutex.Unlock()
	deadline := time.Now().Add(5 * time.Second)
	for {
		client.cmutex.Lock()
		recovered := client.subs != old && client.subs.IsValid()
		client.cmutex.Unlock()
		if recovered {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reply subscription did not recover")
		}
		time.Sleep(time.Millisecond)
	}
	callback := make(chan *rpcpb.ResultInfo, 1)
	if err := client.Call(lifecycleCall("recovered"), callback); err != nil {
		t.Fatal(err)
	}
	body, _ := proto.Marshal(&rpcpb.ResultInfo{Cid: "recovered"})
	if err := conn.Publish(client.callbackqueueName, body); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-callback:
		if result == nil || result.Cid != "recovered" {
			t.Fatal("wrong reply")
		}
	case <-time.After(time.Second):
		t.Fatal("recovered client missed reply")
	}
	_ = client.Done()
	if n := conn.NumSubscriptions(); n != 0 {
		t.Fatalf("recovered subscription leaked: %d", n)
	}
}

func TestNatsClientDoneCancelsBlockedDelivery(t *testing.T) {
	conn := lifecycleConnection(t)
	client := lifecycleClient(t, conn)
	callback := make(chan *rpcpb.ResultInfo)
	if err := client.Call(lifecycleCall("blocked"), callback); err != nil {
		t.Fatal(err)
	}
	body, _ := proto.Marshal(&rpcpb.ResultInfo{Cid: "blocked"})
	_ = conn.Publish(client.callbackqueueName, body)
	// 等待接收协程取走回调，确保关闭能打断无接收方的发送。
	deadline := time.Now().Add(time.Second)
	for client.callinfos.Get("blocked") != nil {
		if time.Now().After(deadline) {
			t.Fatal("reply was not processed")
		}
		time.Sleep(time.Millisecond)
	}
	done := make(chan struct{})
	go func() { _ = client.Done(); close(done) }()
	lifecycleWait(t, done)
	if _, ok := <-callback; ok {
		t.Fatal("blocked callback was not closed")
	}
}

func TestNatsClientTimeoutReplyRace(t *testing.T) {
	conn := lifecycleConnection(t)
	client := lifecycleClient(t, conn)
	sub, err := conn.Subscribe("lifecycle.requests", func(msg *nats.Msg) {
		var request rpcpb.RPCInfo
		if proto.Unmarshal(msg.Data, &request) != nil {
			return
		}
		body, _ := proto.Marshal(&rpcpb.ResultInfo{Cid: request.Cid, Error: "test reply"})
		_ = conn.Publish(request.ReplyTo, body)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()
	if err = conn.Flush(); err != nil {
		t.Fatal(err)
	}
	rpc := &RPCClient{app: client.app, nats_client: client}
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			defer cancel()
			_, err := rpc.CallArgs(ctx, "test", nil, nil)
			if err == nil || (err.Error() != "test reply" && err.Error() != "deadline exceeded") {
				t.Errorf("unexpected RPC result: %v", err)
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	lifecycleWait(t, done)
}

func TestNatsClientTransportClosed(t *testing.T) {
	conn := lifecycleConnection(t)
	client := lifecycleClient(t, conn)
	pending := make(chan *rpcpb.ResultInfo, 1)
	if err := client.Call(lifecycleCall("pending"), pending); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	lifecycleWait(t, client.stopped)
	if _, ok := <-pending; ok {
		t.Fatal("pending callback not released")
	}
	if err := client.CallNR(lifecycleCall("closed")); err == nil {
		t.Fatal("transport-closed client accepted call")
	}
}

func TestNatsClientRecoveryConcurrentDone(t *testing.T) {
	conn := lifecycleConnection(t)
	for round := 0; round < 100; round++ {
		client := lifecycleClient(t, conn)
		client.cmutex.Lock()
		old := client.subs
		_ = old.Unsubscribe()
		client.cmutex.Unlock()
		done := make(chan struct{})
		go func() { _ = client.Done(); close(done) }()
		lifecycleWait(t, done)
		if n := conn.NumSubscriptions(); n != 0 {
			t.Fatalf("subscription leaked during recovery/close: %d", n)
		}
	}
}

func TestNatsClientPublishFailureReleasesPendingCall(t *testing.T) {
	conn := lifecycleConnection(t)
	client := lifecycleClient(t, conn)
	client.session.GetNode().Address = ""
	if err := client.Call(lifecycleCall("failed"), make(chan *rpcpb.ResultInfo, 1)); err == nil {
		t.Fatal("invalid subject accepted")
	}
	if len(client.callinfos.Items()) != 0 {
		t.Fatal("failed publish retained callback")
	}
}
