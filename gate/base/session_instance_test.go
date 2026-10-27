package basegate

import (
	"context"
	"errors"
	"testing"

	"github.com/jarekzha/mqant/gate"
	"github.com/jarekzha/mqant/module"
)

type sessionInstanceApp struct {
	module.App
	instances      map[string]module.ServerSession
	lookups        []string
	logicalLookups int
}

func (app *sessionInstanceApp) GetServerByInstanceID(id string) (module.ServerSession, error) {
	app.lookups = append(app.lookups, id)
	if server, ok := app.instances[id]; ok {
		return server, nil
	}
	return nil, errors.New("instance unavailable")
}

func (app *sessionInstanceApp) GetServerByID(id string) (module.ServerSession, error) {
	app.logicalLookups++
	return nil, errors.New("logical lookup must not be used for a session")
}

type sessionInstanceServer struct {
	module.ServerSession
	calls  []string
	result gate.Session
}

func (server *sessionInstanceServer) Call(_ context.Context, method string, params ...interface{}) (interface{}, error) {
	server.calls = append(server.calls, method)
	switch method {
	case "IsConnect":
		return true, nil
	case "SendBatch":
		return int64(1), nil
	case "Send", "Close":
		return nil, nil
	default:
		return server.result, nil
	}
}

func (server *sessionInstanceServer) CallNR(method string, params ...interface{}) error {
	server.calls = append(server.calls, method)
	return nil
}

func instanceSession(t *testing.T, app module.App, instanceID string) gate.Session {
	t.Helper()
	session, err := NewSessionByMap(app, map[string]interface{}{
		"Serverid": "Gate@Gate001", "GateInstanceID": instanceID,
		"Sessionid": "session-A", "Settings": map[string]string{"test": "value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestSessionGateInstanceSurvivesSerializationAndCopies(t *testing.T) {
	original := instanceSession(t, nil, "Gate@Gate001#A")
	check := func(name string, session gate.Session) {
		t.Helper()
		if session.GetServerID() != original.GetServerID() || session.GetGateInstanceID() != original.GetGateInstanceID() {
			t.Fatalf("%s lost Gate identity: %s / %s", name, session.GetServerID(), session.GetGateInstanceID())
		}
	}
	check("clone", original.Clone())
	check("trace copy", original.ExtractSpan().(gate.Session))
	for _, marshal := range []func() ([]byte, error){original.Serializable, original.(*sessionagent).Marshal} {
		data, err := marshal()
		if err != nil {
			t.Fatal(err)
		}
		restored, err := NewSession(nil, data)
		if err != nil {
			t.Fatal(err)
		}
		check("protobuf round trip", restored)
		target := instanceSession(t, nil, "Gate@Gate001#B").(*sessionagent)
		if err := target.Unmarshal(data); err != nil {
			t.Fatal(err)
		}
		check("unmarshal", target)
		target = instanceSession(t, nil, "Gate@Gate001#B").(*sessionagent)
		if err := target.update(restored); err != nil {
			t.Fatal(err)
		}
		check("update", target)
	}
}

func TestSessionOperationsUseOnlyOwningGateInstance(t *testing.T) {
	operations := []struct {
		name string
		call func(gate.Session) error
	}{
		{"Update", func(s gate.Session) error { return s.Update() }},
		{"Bind", func(s gate.Session) error { return s.Bind("user") }},
		{"UnBind", func(s gate.Session) error { return s.UnBind() }},
		{"Push", func(s gate.Session) error { return s.Push() }},
		{"Set", func(s gate.Session) error { return s.Set("test", "value") }},
		{"SetPush", func(s gate.Session) error { return s.SetPush("test", "value") }},
		{"SetBatch", func(s gate.Session) error { return s.SetBatch(map[string]string{"test": "value"}) }},
		{"Remove", func(s gate.Session) error { return s.Remove("test") }},
		{"Send", func(s gate.Session) error { return s.Send("topic", []byte("body")) }},
		{"SendNR", func(s gate.Session) error { return s.SendNR("topic", []byte("body")) }},
		{"SendBatch", func(s gate.Session) error { _, err := s.SendBatch("session-A", "topic", []byte("body")); return err }},
		{"IsConnect", func(s gate.Session) error { _, err := s.IsConnect("user"); return err }},
		{"Close", func(s gate.Session) error { return s.Close() }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			first, second := &sessionInstanceServer{}, &sessionInstanceServer{}
			app := &sessionInstanceApp{instances: map[string]module.ServerSession{"Gate@Gate001#A": first, "Gate@Gate001#B": second}}
			session := instanceSession(t, app, "Gate@Gate001#A")
			first.result = session.Clone()
			if err := operation.call(session); err != nil {
				t.Fatal(err)
			}
			if len(first.calls) != 1 || len(second.calls) != 0 || len(app.lookups) != 1 || app.lookups[0] != "Gate@Gate001#A" || app.logicalLookups != 0 {
				t.Fatalf("operation did not reach only A: A=%v B=%v lookups=%v logical=%d", first.calls, second.calls, app.lookups, app.logicalLookups)
			}
			// A 退出后 B 仍然在线，旧会话必须失败，不能切换到 B。
			delete(app.instances, "Gate@Gate001#A")
			if err := operation.call(session); err == nil {
				t.Fatal("missing owner did not fail")
			}
			if len(second.calls) != 0 || app.logicalLookups != 0 {
				t.Fatal("missing owner fell back to another instance")
			}
			// 没有 InstanceID 的旧 Session 不进行任何服务发现或逻辑 ID 回退。
			missing := instanceSession(t, app, "")
			lookups := len(app.lookups)
			if err := operation.call(missing); err == nil {
				t.Fatal("session without instance ID did not fail")
			}
			if len(app.lookups) != lookups || app.logicalLookups != 0 {
				t.Fatal("session without instance ID attempted routing")
			}
		})
	}
}

func TestNewGateSessionDoesNotRouteToOldInstance(t *testing.T) {
	first, second := &sessionInstanceServer{}, &sessionInstanceServer{}
	app := &sessionInstanceApp{instances: map[string]module.ServerSession{
		"Gate@Gate001#A": first, "Gate@Gate001#B": second,
	}}
	session := instanceSession(t, app, "Gate@Gate001#B")
	second.result = session.Clone()
	if err := session.Bind("user"); err != nil {
		t.Fatal(err)
	}
	if len(first.calls) != 0 || len(second.calls) != 1 || len(app.lookups) != 1 || app.lookups[0] != "Gate@Gate001#B" || app.logicalLookups != 0 {
		t.Fatal("new Gate session was sent to the old instance")
	}
}
