// Copyright 2014 mqant Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package app mqant默认应用实现
package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jarekzha/mqant/conf"
	"github.com/jarekzha/mqant/log"
	"github.com/jarekzha/mqant/module"
	basemodule "github.com/jarekzha/mqant/module/base"
	"github.com/jarekzha/mqant/module/modules"
	"github.com/jarekzha/mqant/registry"
	mqrpc "github.com/jarekzha/mqant/rpc"
	"github.com/jarekzha/mqant/selector"
	"github.com/jarekzha/mqant/selector/cache"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
)

type resultInfo struct {
	Trace  string
	Error  string      // 错误结果 如果为nil表示请求正确
	Result interface{} // 结果
}

type protocolMarshalImp struct {
	data []byte
}

func (p *protocolMarshalImp) GetData() []byte {
	return p.data
}

func newOptions(opts ...module.Option) module.Options {
	opt := module.Options{
		Registry:         registry.DefaultRegistry,
		Selector:         cache.NewSelector(),
		RegisterInterval: time.Second * time.Duration(10),
		RegisterTTL:      time.Second * time.Duration(20),
		KillWaitTTL:      time.Second * time.Duration(60),
		RPCExpired:       time.Second * time.Duration(10),
		RPCMaxCoroutine:  0, //不限制
		Debug:            true,
		Parse:            true,
		LogFileName: func(logdir, prefix, processID, suffix string) string {
			return fmt.Sprintf("%s/%v%s%s", logdir, prefix, processID, suffix)
		},
	}

	for _, o := range opts {
		o(&opt)
	}

	if opt.Parse {
		parseFlag(&opt)
	}

	if opt.Nats == nil {
		nc, err := nats.Connect(nats.DefaultURL)
		if err != nil {
			log.DPanic("Nats connect fail", log.Err(err))
		}
		opt.Nats = nc
	}

	if opt.ProcessID == "" {
		opt.ProcessID = "development"
	}
	appDir := ""
	if opt.WorkDir != "" {
		_, err := os.Open(opt.WorkDir)
		if err != nil {
			panic(err)
		}
		os.Chdir(opt.WorkDir)
		appDir, err = os.Getwd()
		if err != nil {
			panic(err)
		}
	} else {
		var err error
		appDir, err = os.Getwd()
		if err != nil {
			file, _ := exec.LookPath(os.Args[0])
			appPath, _ := filepath.Abs(file)
			appDir, _ = filepath.Split(appPath)
		}
	}
	opt.WorkDir = appDir

	if opt.ConfPath == "" {
		opt.ConfPath = fmt.Sprintf("%s/bin/conf/server.yaml", appDir)
	}
	if opt.LogDir == "" {
		opt.LogDir = fmt.Sprintf("%s/bin/logs", appDir)
	}

	_, err := os.Open(opt.ConfPath)
	if err != nil {
		//文件不存在
		panic(fmt.Sprintf("config path error %v", err))
	}
	_, err = os.Open(opt.LogDir)
	if err != nil {
		//文件不存在
		err := os.Mkdir(opt.LogDir, os.ModePerm) //
		if err != nil {
			fmt.Println(err)
		}
	}

	return opt
}

// 解析命令行参数
func parseFlag(opt *module.Options) {
	workDir := flag.String("wd", "", "Server work directory")
	confPath := flag.String("conf", "", "Server configuration file path(json,yaml,toml)")
	processID := flag.String("pid", "development", "Server ProcessID?")
	logDir := flag.String("log", "", "Log file directory?")
	flag.Parse() // 解析输入的参数

	if *workDir != "" {
		opt.WorkDir = *workDir
	}
	if *confPath != "" {
		opt.ConfPath = *confPath
	}
	if *processID != "" {
		opt.ProcessID = *processID
	}
	if *logDir != "" {
		opt.LogDir = *logDir
	}
}

// NewApp 创建app
func NewApp(opts ...module.Option) module.App {
	options := newOptions(opts...)
	app := new(DefaultApp)
	app.opts = options

	if !options.Debug {
		options.Selector.Init(selector.SetWatcher(app.Watcher))
	}

	app.rpcserializes = map[string]module.RPCSerialize{}
	return app
}

// DefaultApp 默认应用
type DefaultApp struct {
	//module.App
	settings   conf.Config
	serverList sync.Map
	opts       module.Options
	//将一个RPC调用路由到新的路由上
	mapRoute            func(app module.App, route string) string
	rpcserializes       map[string]module.RPCSerialize
	configurationLoaded func(app module.App)
	startup             func(app module.App)
	moduleInited        func(app module.App, module module.Module)
	protocolMarshal     func(trace string, result interface{}, err error) (module.ProtocolMarshal, error)
}

// Run 运行应用
func (app *DefaultApp) Run(mods ...module.Module) error {
	// 加载配置文件
	fmt.Println("Server configuration path:", app.opts.ConfPath)
	conf.LoadConfig(app.opts.ConfPath)
	// 解析配置信息
	app.Configure(conf.Conf)
	// 配置解析
	if app.configurationLoaded != nil {
		app.configurationLoaded(app)
	}

	log.Init(log.WithDebug(app.opts.Debug),
		log.WithProcessID(app.opts.ProcessID),
		log.WithLogDir(app.opts.LogDir),
		log.WithLogFileName(app.opts.LogFileName),
		log.WithLogSetting(conf.Conf.Log))
	log.Info("Framework mqant starting up", log.String("version", app.opts.Version))

	manager := basemodule.NewModuleManager()
	manager.RegisterRunMod(modules.TimerModule()) //注册时间轮模块 每一个进程都默认运行
	// module
	for i := 0; i < len(mods); i++ {
		mods[i].OnAppConfigurationLoaded(app)
		manager.Register(mods[i])
	}
	app.OnInit(app.settings)
	manager.Init(app, app.opts.ProcessID)
	if app.startup != nil {
		app.startup(app)
	}
	log.Info("Framework mqant started", log.String("version", app.opts.Version))
	// close
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	sig := <-c
	log.Sync()
	//如果一分钟都关不了则强制关闭
	timeout := time.NewTimer(app.opts.KillWaitTTL)
	wait := make(chan struct{})
	go func() {
		manager.Destroy()
		app.OnDestroy()
		wait <- struct{}{}
	}()
	select {
	case <-timeout.C:
		log.DPanic("mqant close timeout", log.Any("singal", sig))
	case <-wait:
		log.Info("mqant closing down", log.Any("signal", sig))
	}
	log.Sync()
	return nil
}

func (app *DefaultApp) UpdateOptions(opts ...module.Option) error {
	for _, o := range opts {
		o(&app.opts)
	}
	return nil
}

// SetMapRoute 设置路由器
func (app *DefaultApp) SetMapRoute(fn func(app module.App, route string) string) error {
	app.mapRoute = fn
	return nil
}

// AddRPCSerialize AddRPCSerialize
func (app *DefaultApp) AddRPCSerialize(name string, Interface module.RPCSerialize) error {
	if _, ok := app.rpcserializes[name]; ok {
		return fmt.Errorf("rpc serialize name(%s) has been occupied", name)
	}
	app.rpcserializes[name] = Interface
	return nil
}

// Options 应用配置
func (app *DefaultApp) Options() module.Options {
	return app.opts
}

// Transport Transport
func (app *DefaultApp) Transport() *nats.Conn {
	return app.opts.Nats
}

// Registry Registry
func (app *DefaultApp) Registry() registry.Registry {
	return app.opts.Registry
}

// GetRPCSerialize GetRPCSerialize
func (app *DefaultApp) GetRPCSerialize() map[string]module.RPCSerialize {
	return app.rpcserializes
}

// Watcher Watcher
func (app *DefaultApp) Watcher(node *registry.Node) {
	//把注销的服务ServerSession删除掉
	session, ok := app.serverList.Load(node.ID)
	if ok && session != nil {
		log.Info("Watcher delete node", log.String("id", node.ID))
		session.(module.ServerSession).GetRPC().Done()
		app.serverList.Delete(node.ID)
	}
}

// Configure 重设应用配置
func (app *DefaultApp) Configure(settings conf.Config) error {
	app.settings = settings

	// RPC 设置
	app.opts.RPCExpired = time.Second * time.Duration(settings.RPC.RPCExpired)
	app.opts.RPCMaxCoroutine = settings.RPC.MaxCoroutine

	// debug模式下不超时
	if app.opts.Debug {
		app.opts.RPCExpired = time.Hour
	}

	return nil
}

// OnInit 初始化
func (app *DefaultApp) OnInit(settings conf.Config) error {

	return nil
}

// OnDestroy 应用退出
func (app *DefaultApp) OnDestroy() error {

	return nil
}

// GetServerByID 通过服务ID获取服务实例
func (app *DefaultApp) GetServerByID(serverID string) (module.ServerSession, error) {
	session, ok := app.serverList.Load(serverID)
	if !ok {
		serviceName := serverID
		s := strings.Split(serverID, "@")
		if len(s) == 2 {
			serviceName = s[0]
		} else {
			return nil, errors.Errorf("serverID is error %v", serverID)
		}
		sessions := app.GetServersByType(serviceName)
		for _, s := range sessions {
			if s.GetNode().ID == serverID {
				return s, nil
			}
		}
	} else {
		return session.(module.ServerSession), nil
	}
	return nil, errors.Errorf("nofound %v", serverID)
}

// GetServerBySelector 获取服务实例,可设置选择器
func (app *DefaultApp) GetServerBySelector(serviceName string, opts ...selector.SelectOption) (module.ServerSession, error) {
	next, err := app.opts.Selector.Select(serviceName, opts...)
	if err != nil {
		return nil, err
	}
	node, err := next()
	if err != nil {
		return nil, err
	}
	session, ok := app.serverList.Load(node.ID)
	if !ok {
		s, err := basemodule.NewServerSession(app, serviceName, node)
		if err != nil {
			return nil, err
		}
		app.serverList.Store(node.ID, s)
		return s, nil
	}
	session.(module.ServerSession).SetNode(node)
	return session.(module.ServerSession), nil

}

// GetServersByType 通过服务类型获取服务实例列表
func (app *DefaultApp) GetServersByType(serviceName string) []module.ServerSession {
	sessions := make([]module.ServerSession, 0)
	services, err := app.opts.Selector.GetService(serviceName)
	if err != nil {
		log.Warn("GetServersByType fail", log.Err(err))
		return sessions
	}
	for _, service := range services {
		//log.TInfo(nil,"GetServersByType3 %v %v",Type,service.Nodes)
		for _, node := range service.Nodes {
			session, ok := app.serverList.Load(node.ID)
			if !ok {
				s, err := basemodule.NewServerSession(app, serviceName, node)
				if err != nil {
					log.Warn("NewServerSession fail", log.Err(err))
				} else {
					app.serverList.Store(node.ID, s)
					sessions = append(sessions, s)
				}
			} else {
				session.(module.ServerSession).SetNode(node)
				sessions = append(sessions, session.(module.ServerSession))
			}
		}
	}
	return sessions
}

// GetRouteServer 通过选择器过滤服务实例
func (app *DefaultApp) GetRouteServer(filter string, opts ...selector.SelectOption) (s module.ServerSession, err error) {
	if app.mapRoute != nil {
		//进行一次路由转换
		filter = app.mapRoute(app, filter)
	}
	sl := strings.Split(filter, "@")
	if len(sl) == 2 {
		moduleID := sl[1]
		if moduleID != "" {
			return app.GetServerByID(filter)
		}
	}
	moduleType := sl[0]
	return app.GetServerBySelector(moduleType, opts...)
}

// GetSettings 获取配置
func (app *DefaultApp) GetSettings() conf.Config {
	return app.settings
}

// GetProcessID 获取应用分组ID
func (app *DefaultApp) GetProcessID() string {
	return app.opts.ProcessID
}

// WorkDir 获取进程工作目录
func (app *DefaultApp) WorkDir() string {
	return app.opts.WorkDir
}

// Invoke Invoke
func (app *DefaultApp) Invoke(module module.RPCModule, moduleType string, _func string, params ...interface{}) (result interface{}, err error) {
	server, e := app.GetRouteServer(moduleType)
	if e != nil {
		err = e
		return
	}
	return server.Call(context.TODO(), _func, params...)
}

// InvokeNR InvokeNR
func (app *DefaultApp) InvokeNR(module module.RPCModule, moduleType string, _func string, params ...interface{}) (err error) {
	server, e := app.GetRouteServer(moduleType)
	if e != nil {
		err = e
		return
	}
	return server.CallNR(_func, params...)
}

// Call Call
func (app *DefaultApp) Call(ctx context.Context, moduleType, _func string, param mqrpc.ParamOption, opts ...selector.SelectOption) (result interface{}, err error) {
	server, e := app.GetRouteServer(moduleType, opts...)
	if e != nil {
		err = e
		return
	}
	return server.Call(ctx, _func, param()...)
}

// GetModuleInited GetModuleInited
func (app *DefaultApp) GetModuleInited() func(app module.App, module module.Module) {
	return app.moduleInited
}

// OnConfigurationLoaded 设置配置初始化完成后回调
func (app *DefaultApp) OnConfigurationLoaded(_func func(app module.App)) error {
	app.configurationLoaded = _func
	return nil
}

// OnModuleInited 设置模块初始化完成后回调
func (app *DefaultApp) OnModuleInited(_func func(app module.App, module module.Module)) error {
	app.moduleInited = _func
	return nil
}

// OnStartup 设置应用启动完成后回调
func (app *DefaultApp) OnStartup(_func func(app module.App)) error {
	app.startup = _func
	return nil
}

// SetProtocolMarshal 设置RPC数据包装器
func (app *DefaultApp) SetProtocolMarshal(protocolMarshal func(trace string, result interface{}, err error) (module.ProtocolMarshal, error)) error {
	app.protocolMarshal = protocolMarshal
	return nil
}

// ProtocolMarshal RPC数据包装器
func (app *DefaultApp) ProtocolMarshal(trace string, result interface{}, err error) (module.ProtocolMarshal, error) {
	if app.protocolMarshal != nil {
		return app.protocolMarshal(trace, result, err)
	}
	r := &resultInfo{
		Trace:  trace,
		Result: result,
	}
	if err != nil {
		r.Error = err.Error()
	}

	b, err := json.Marshal(r)
	if err == nil {
		return app.NewProtocolMarshal(b), nil
	}
	return nil, err
}

// NewProtocolMarshal 创建RPC数据包装器
func (app *DefaultApp) NewProtocolMarshal(data []byte) module.ProtocolMarshal {
	return &protocolMarshalImp{
		data: data,
	}
}
