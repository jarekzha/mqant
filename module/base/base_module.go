// Copyright 2014 mqantserver Author. All Rights Reserved.
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

// Package basemodule BaseModule定义
package basemodule

import (
	"context"
	"fmt"
	"os"

	"github.com/jarekzha/mqant/conf"
	"github.com/jarekzha/mqant/log"
	"github.com/jarekzha/mqant/module"
	mqrpc "github.com/jarekzha/mqant/rpc"
	rpcpb "github.com/jarekzha/mqant/rpc/pb"
	"github.com/jarekzha/mqant/selector"
	"github.com/jarekzha/mqant/server"
	"github.com/jarekzha/mqant/service"
	mqanttools "github.com/jarekzha/mqant/utils"
	"github.com/pkg/errors"
)

// BaseModule 默认的RPCModule实现
type BaseModule struct {
	context.Context
	serviceStopeds chan bool
	exit           context.CancelFunc
	App            module.App
	subclass       module.RPCModule
	settings       *conf.ModuleSettings
	service        service.Service
	listener       mqrpc.RPCListener
}

// GetServerID 节点ID
func (m *BaseModule) GetServerID() string {
	//很关键,需要与配置文件中的Module配置对应
	if m.service != nil && m.service.Server() != nil {
		return m.service.Server().ID()
	}
	return "no server"
}

// GetApp module.App
func (m *BaseModule) GetApp() module.App {
	return m.App
}

// GetSubclass 子类
func (m *BaseModule) GetSubclass() module.RPCModule {
	return m.subclass
}

// GetServer server.Server
func (m *BaseModule) GetServer() server.Server {
	return m.service.Server()
}

// OnConfChanged 当配置变更时调用
func (m *BaseModule) OnConfChanged(settings *conf.ModuleSettings) {

}

// OnAppConfigurationLoaded 当应用配置加载完成时调用
func (m *BaseModule) OnAppConfigurationLoaded(app module.App) {
	m.App = app
	//当App初始化时调用，这个接口不管这个模块是否在这个进程运行都会调用
}

// OnInit 当模块初始化时调用
func (m *BaseModule) OnInit(subclass module.RPCModule, app module.App, settings *conf.ModuleSettings, opt ...server.Option) {
	//初始化模块
	m.App = app
	m.subclass = subclass
	m.settings = settings
	//创建一个远程调用的RPC

	opts := server.Options{
		Metadata: map[string]string{},
	}
	for _, o := range opt {
		o(&opts)
	}
	if opts.Registry == nil {
		opt = append(opt, server.Registry(app.Registry()))
	}

	if opts.RegisterInterval == 0 {
		opt = append(opt, server.RegisterInterval(app.Options().RegisterInterval))
	}

	if opts.RegisterTTL == 0 {
		opt = append(opt, server.RegisterTTL(app.Options().RegisterTTL))
	}

	if len(opts.Name) == 0 {
		opt = append(opt, server.Name(subclass.GetType()))
	}

	if len(opts.ID) == 0 {
		if len(settings.ID) > 0 {
			opt = append(opt, server.ID(settings.ID))
		} else {
			opt = append(opt, server.ID(mqanttools.GenerateID().String()))
		}
	}

	if len(opts.Version) == 0 {
		opt = append(opt, server.Version(subclass.Version()))
	}
	server := server.NewServer(opt...)
	err := server.OnInit(subclass, app, settings)
	if err != nil {
		log.Warn("Server OnInit fail", log.String("serverID", m.GetServerID()), log.Err(err))
	}
	hostname, _ := os.Hostname()
	server.Options().Metadata["hostname"] = hostname
	server.Options().Metadata["pid"] = fmt.Sprintf("%v", os.Getpid())
	ctx, cancel := context.WithCancel(context.Background())
	m.exit = cancel
	m.serviceStopeds = make(chan bool)
	m.service = service.NewService(
		service.Server(server),
		service.RegisterTTL(app.Options().RegisterTTL),
		service.RegisterInterval(app.Options().RegisterInterval),
		service.Context(ctx),
	)

	go func() {
		err := m.service.Run()
		if err != nil {
			log.Warn("service run fail", log.String("serverID", m.GetServerID()), log.Err(err))
		}
		close(m.serviceStopeds)
	}()
	m.GetServer().SetListener(m)
}

// OnDestroy 当模块注销时调用
func (m *BaseModule) OnDestroy() {
	//注销模块
	//一定别忘了关闭RPC
	m.exit()
	select {
	case <-m.serviceStopeds:
		//等待注册中心注销完成
	}
	_ = m.GetServer().OnDestroy()
}

// SetListener  mqrpc.RPCListener
func (m *BaseModule) SetListener(listener mqrpc.RPCListener) {
	m.listener = listener
}

// GetModuleSettings  GetModuleSettings
func (m *BaseModule) GetModuleSettings() *conf.ModuleSettings {
	return m.settings
}

// GetRouteServer  GetRouteServer
func (m *BaseModule) GetRouteServer(moduleType string, opts ...selector.SelectOption) (s module.ServerSession, err error) {
	return m.App.GetRouteServer(moduleType, opts...)
}

// Invoke  Invoke
func (m *BaseModule) Invoke(moduleType string, _func string, params ...interface{}) (result interface{}, err error) {
	return m.App.Invoke(m.GetSubclass(), moduleType, _func, params...)
}

// InvokeNR  InvokeNR
func (m *BaseModule) InvokeNR(moduleType string, _func string, params ...interface{}) (err error) {
	return m.App.InvokeNR(m.GetSubclass(), moduleType, _func, params...)
}

// InvokeArgs  InvokeArgs
func (m *BaseModule) InvokeArgs(moduleType string, _func string, ArgsType []string, args [][]byte) (result interface{}, err error) {
	server, e := m.App.GetRouteServer(moduleType)
	if e != nil {
		err = e
		return
	}
	return server.CallArgs(nil, _func, ArgsType, args)
}

// InvokeNRArgs  InvokeNRArgs
func (m *BaseModule) InvokeNRArgs(moduleType string, _func string, ArgsType []string, args [][]byte) (err error) {
	server, err := m.App.GetRouteServer(moduleType)
	if err != nil {
		return
	}
	return server.CallNRArgs(_func, ArgsType, args)
}

// Call  Call
func (m *BaseModule) Call(ctx context.Context, moduleType, _func string, param mqrpc.ParamOption, opts ...selector.SelectOption) (interface{}, error) {
	return m.App.Call(ctx, moduleType, _func, param, opts...)
}

// NoFoundFunction  当hander未找到时调用
func (m *BaseModule) NoFoundFunction(fn string) (*mqrpc.FunctionInfo, error) {
	if m.listener != nil {
		return m.listener.NoFoundFunction(fn)
	}
	return nil, errors.Errorf("Remote function(%s) not found", fn)
}

// BeforeHandle  hander执行前调用
func (m *BaseModule) BeforeHandle(fn string, callInfo *mqrpc.CallInfo) error {
	if m.listener != nil {
		return m.listener.BeforeHandle(fn, callInfo)
	}
	return nil
}

// OnTimeOut  hander执行超时调用
func (m *BaseModule) OnTimeOut(fn string, Expired int64) {
	if m.listener != nil {
		m.listener.OnTimeOut(fn, Expired)
	}
}

// OnError  hander执行错误调用
func (m *BaseModule) OnError(fn string, callInfo *mqrpc.CallInfo, err error) {
	if m.listener != nil {
		m.listener.OnError(fn, callInfo, err)
	}
}

// OnComplete hander成功执行完成时调用
// fn 		方法名
// params		参数
// result		执行结果
// exec_time 	方法执行时间 单位为 Nano 纳秒  1000000纳秒等于1毫秒
func (m *BaseModule) OnComplete(fn string, callInfo *mqrpc.CallInfo, result *rpcpb.ResultInfo, execTime int64) {
	if m.listener != nil {
		m.listener.OnComplete(fn, callInfo, result, execTime)
	}
}

// GetExecuting GetExecuting
func (m *BaseModule) GetExecuting() int64 {
	return 0
	//return m.GetServer().GetRPCServer().GetExecuting()
}
