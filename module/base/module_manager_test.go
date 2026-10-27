package basemodule

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jarekzha/mqant/conf"
	"github.com/jarekzha/mqant/module"
)

type lifecycleApp struct {
	module.App
	config conf.Config
}

func (a *lifecycleApp) GetSettings() conf.Config                         { return a.config }
func (a *lifecycleApp) GetModuleInited() func(module.App, module.Module) { return nil }

type lifecycleModule struct {
	module.Module
	name  string
	init  func()
	ready func()
	run   func()
}

func (m *lifecycleModule) GetType() string { return m.name }
func (m *lifecycleModule) OnInit(module.App, *conf.ModuleSettings) {
	if m.init != nil {
		m.init()
	}
}
func (m *lifecycleModule) OnReady() {
	if m.ready != nil {
		m.ready()
	}
}
func (m *lifecycleModule) Run(chan bool) {
	if m.run != nil {
		m.run()
	}
}

func lifecycleConfig(t *testing.T, names ...string) *lifecycleApp {
	t.Helper()
	old := conf.Conf
	t.Cleanup(func() { conf.Conf = old })
	config := conf.Config{Module: map[string][]*conf.ModuleSettings{}}
	for _, name := range names {
		config.Module[name] = []*conf.ModuleSettings{{ID: name, ProcessID: "world"}}
	}
	conf.Conf = config
	return &lifecycleApp{config: config}
}

func TestModulesPublishInOrderBeforeOpeningIngress(t *testing.T) {
	app := lifecycleConfig(t, "DBProxy", "Activity", "Gate")
	manager := NewModuleManager()
	var events []string
	runs := make(chan string, 3)
	for _, name := range []string{"DBProxy", "Activity", "Gate"} {
		name := name
		manager.Register(&lifecycleModule{name: name,
			init:  func() { events = append(events, name+" init") },
			ready: func() { events = append(events, name+" ready") },
			run: func() {
				if len(events) != 6 {
					runs <- "entry opened before all modules were ready"
					return
				}
				runs <- name
			},
		})
	}
	manager.Init(app, "world")
	want := []string{"DBProxy init", "DBProxy ready", "Activity init", "Activity ready", "Gate init", "Gate ready"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("lifecycle order: %v", events)
	}
	for i := 0; i < 3; i++ {
		select {
		case name := <-runs:
			if name == "entry opened before all modules were ready" {
				t.Fatal(name)
			}
		case <-time.After(time.Second):
			t.Fatal("module Run did not start")
		}
	}
	for _, m := range manager.runMods {
		m.wg.Wait()
	}
}

func TestInitializationFailureDoesNotPublishOrOpenGate(t *testing.T) {
	for _, failedPhase := range []string{"init", "ready"} {
		t.Run(failedPhase, func(t *testing.T) {
			app := lifecycleConfig(t, "Activity", "Gate")
			manager := NewModuleManager()
			gateInitialized := false
			activityReady := false
			fail := func() { panic("required initialization failed") }
			activity := &lifecycleModule{name: "Activity", ready: func() { activityReady = true }}
			if failedPhase == "init" {
				activity.init = fail
			} else {
				activity.ready = fail
			}
			manager.Register(activity)
			manager.Register(&lifecycleModule{name: "Gate", init: func() { gateInitialized = true }})
			defer func() {
				if r := recover(); r != "required initialization failed" {
					t.Fatalf("startup panic was swallowed: %v", r)
				}
				if gateInitialized || activityReady {
					t.Fatal("failed initialization continued publishing or opening Gate")
				}
			}()
			manager.Init(app, "world")
		})
	}
}

func TestBootstrapRunsDuringBusinessInitialization(t *testing.T) {
	app := lifecycleConfig(t, "Gate")
	manager := NewModuleManager()
	started := make(chan struct{})
	manager.RegisterRunMod(&lifecycleModule{name: "Timer", run: func() { close(started) }})
	manager.Register(&lifecycleModule{name: "Gate", init: func() {
		select {
		case <-started:
		case <-time.After(time.Second):
			panic("bootstrap not running during OnInit")
		}
	}})
	manager.Init(app, "world")
	for _, m := range manager.runMods {
		m.wg.Wait()
	}
}

func TestRunRecoverLogsAndRethrows(t *testing.T) {
	m := &ModuleAgent{mi: &lifecycleModule{run: func() { panic("run failed") }}}
	m.wg.Add(1)
	defer func() {
		if r := recover(); fmt.Sprint(r) != "run failed" {
			t.Fatalf("Run panic was swallowed: %v", r)
		}
		m.wg.Wait()
	}()
	run(m)
}
