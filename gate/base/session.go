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

// Package basegate gate.Session
package basegate

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/jarekzha/mqant/gate"
	"github.com/jarekzha/mqant/log"
	"github.com/jarekzha/mqant/module"
	mqanttools "github.com/jarekzha/mqant/utils"
	"google.golang.org/protobuf/proto"
)

var errorAppIsNil = errors.New("Module.App is nil")

type sessionagent struct {
	app        module.App
	session    *SessionImp
	lock       *sync.RWMutex
	userdata   interface{}
	judgeGuest func(session gate.Session) bool
}

// NewSession NewSession
func NewSession(app module.App, data []byte) (gate.Session, error) {
	agent := &sessionagent{
		app:  app,
		lock: new(sync.RWMutex),
	}
	se := &SessionImp{}
	err := proto.Unmarshal(data, se)
	if err != nil {
		return nil, err
	} // 测试结果
	agent.session = se
	if app != nil {
		agent.judgeGuest = gate.JudgeGuest
	}
	if agent.session.GetSettings() == nil {
		agent.session.Settings = make(map[string]string)
	}
	return agent, nil
}

// NewSessionByMap NewSessionByMap
func NewSessionByMap(app module.App, data map[string]interface{}) (gate.Session, error) {
	agent := &sessionagent{
		app:     app,
		session: new(SessionImp),
		lock:    new(sync.RWMutex),
	}
	err := agent.updateMap(data)
	if err != nil {
		return nil, err
	}
	if agent.session.GetSettings() == nil {
		agent.session.Settings = make(map[string]string)
	}
	if app != nil {
		agent.judgeGuest = gate.JudgeGuest
	}
	return agent, nil
}

func (sesAgent *sessionagent) GetIP() string {
	sesAgent.lock.RLock()
	defer sesAgent.lock.RUnlock()
	return sesAgent.session.GetIP()
}

func (sesAgent *sessionagent) GetTopic() string {
	sesAgent.lock.RLock()
	defer sesAgent.lock.RUnlock()
	return sesAgent.session.GetTopic()
}

func (sesAgent *sessionagent) GetNetwork() string {
	sesAgent.lock.RLock()
	defer sesAgent.lock.RUnlock()
	return sesAgent.session.GetNetwork()
}

func (sesAgent *sessionagent) GetUserId() string {
	return sesAgent.GetUserID()
}

func (sesAgent *sessionagent) GetUserID() string {
	return sesAgent.session.GetUserId()
}

func (sesAgent *sessionagent) GetUserIDInt64() int64 {
	uid64, err := strconv.ParseInt(sesAgent.session.GetUserId(), 10, 64)
	if err != nil {
		return -1
	}
	return uid64
}

func (sesAgent *sessionagent) GetUserIdInt64() int64 {
	return sesAgent.GetUserIDInt64()
}

func (sesAgent *sessionagent) GetSessionID() string {
	return sesAgent.session.GetSessionId()
}

func (sesAgent *sessionagent) GetSessionId() string {
	return sesAgent.GetSessionID()
}

func (sesAgent *sessionagent) GetServerID() string {
	return sesAgent.session.GetServerId()
}

func (sesAgent *sessionagent) GetGateInstanceID() string {
	return sesAgent.session.GetGateInstanceId()
}

func (sesAgent *sessionagent) GetServerId() string {
	return sesAgent.GetServerID()
}

func (sesAgent *sessionagent) SettingsRange(f func(k, v string) bool) {
	sesAgent.lock.Lock()
	defer sesAgent.lock.Unlock()
	if sesAgent.session.GetSettings() == nil {
		return
	}
	for k, v := range sesAgent.session.GetSettings() {
		c := f(k, v)
		if c == false {
			return
		}
	}
}

// ImportSettings 合并两个map 并且以 agent.(Agent).GetSession().Settings 已有的优先
func (sesAgent *sessionagent) ImportSettings(settings map[string]string) error {
	sesAgent.lock.Lock()
	if sesAgent.session.GetSettings() == nil {
		sesAgent.session.Settings = settings
	} else {
		for k, v := range settings {
			if _, ok := sesAgent.session.GetSettings()[k]; ok {
				//不用替换
			} else {
				sesAgent.session.GetSettings()[k] = v
			}
		}
	}
	sesAgent.lock.Unlock()
	return nil
}

func (sesAgent *sessionagent) LocalUserData() interface{} {
	return sesAgent.userdata
}

func (sesAgent *sessionagent) SetIP(ip string) {
	sesAgent.lock.Lock()
	sesAgent.session.IP = ip
	sesAgent.lock.Unlock()
}
func (sesAgent *sessionagent) SetTopic(topic string) {
	sesAgent.lock.Lock()
	sesAgent.session.Topic = topic
	sesAgent.lock.Unlock()
}
func (sesAgent *sessionagent) SetNetwork(network string) {
	sesAgent.lock.Lock()
	sesAgent.session.Network = network
	sesAgent.lock.Unlock()
}
func (sesAgent *sessionagent) SetUserID(userid string) {
	sesAgent.lock.Lock()
	sesAgent.session.UserId = userid
	sesAgent.lock.Unlock()
}
func (sesAgent *sessionagent) SetUserId(userid string) {
	sesAgent.SetUserID(userid)
}
func (sesAgent *sessionagent) SetSessionID(sessionid string) {
	sesAgent.session.SessionId = sessionid
}
func (sesAgent *sessionagent) SetSessionId(sessionid string) {
	sesAgent.SetSessionID(sessionid)
}
func (sesAgent *sessionagent) SetServerID(serverid string) {
	sesAgent.session.ServerId = serverid
}
func (sesAgent *sessionagent) SetGateInstanceID(instanceID string) {
	sesAgent.session.GateInstanceId = instanceID
}

func (sesAgent *sessionagent) SetServerId(serverid string) {
	sesAgent.SetServerID(serverid)
}
func (sesAgent *sessionagent) SetSettings(settings map[string]string) {
	sesAgent.lock.Lock()
	sesAgent.session.Settings = settings
	sesAgent.lock.Unlock()
}
func (sesAgent *sessionagent) CloneSettings() map[string]string {
	sesAgent.lock.Lock()
	defer sesAgent.lock.Unlock()
	tmp := map[string]string{}
	for k, v := range sesAgent.session.Settings {
		tmp[k] = v
	}
	return tmp
}
func (sesAgent *sessionagent) SetLocalKV(key, value string) error {
	sesAgent.lock.Lock()
	sesAgent.session.GetSettings()[key] = value
	sesAgent.lock.Unlock()
	return nil
}
func (sesAgent *sessionagent) RemoveLocalKV(key string) error {
	sesAgent.lock.Lock()
	delete(sesAgent.session.GetSettings(), key)
	sesAgent.lock.Unlock()
	return nil
}
func (sesAgent *sessionagent) SetLocalUserData(data interface{}) error {
	sesAgent.userdata = data
	return nil
}

func (sesAgent *sessionagent) updateMap(s map[string]interface{}) error {
	Userid := s["Userid"]
	if Userid != nil {
		sesAgent.session.UserId = Userid.(string)
	}
	IP := s["IP"]
	if IP != nil {
		sesAgent.session.IP = IP.(string)
	}
	if topic, ok := s["Topic"]; ok {
		sesAgent.session.Topic = topic.(string)
	}
	Network := s["Network"]
	if Network != nil {
		sesAgent.session.Network = Network.(string)
	}
	Sessionid := s["Sessionid"]
	if Sessionid != nil {
		sesAgent.session.SessionId = Sessionid.(string)
	}
	Serverid := s["Serverid"]
	if Serverid != nil {
		sesAgent.session.ServerId = Serverid.(string)
	}
	if instanceID, ok := s["GateInstanceID"]; ok {
		sesAgent.session.GateInstanceId = instanceID.(string)
	}
	Settings := s["Settings"]
	if Settings != nil {
		sesAgent.lock.Lock()
		sesAgent.session.Settings = Settings.(map[string]string)
		sesAgent.lock.Unlock()
	}
	return nil
}

func (sesAgent *sessionagent) update(s gate.Session) error {
	Userid := s.GetUserID()
	sesAgent.session.UserId = Userid
	IP := s.GetIP()
	sesAgent.session.IP = IP
	sesAgent.session.Topic = s.GetTopic()
	Network := s.GetNetwork()
	sesAgent.session.Network = Network
	Sessionid := s.GetSessionID()
	sesAgent.session.SessionId = Sessionid
	Serverid := s.GetServerID()
	sesAgent.session.ServerId = Serverid
	sesAgent.session.GateInstanceId = s.GetGateInstanceID()
	Settings := map[string]string{}
	s.SettingsRange(func(k, v string) bool {
		Settings[k] = v
		return true
	})
	sesAgent.lock.Lock()
	sesAgent.session.Settings = Settings
	sesAgent.lock.Unlock()
	return nil
}

func (sesAgent *sessionagent) Serializable() ([]byte, error) {
	sesAgent.lock.RLock()
	data, err := proto.Marshal(sesAgent.session)
	sesAgent.lock.RUnlock()
	if err != nil {
		return nil, err
	} // 进行解码
	return data, nil
}

func (sesAgent *sessionagent) Marshal() ([]byte, error) {
	sesAgent.lock.RLock()
	data, err := proto.Marshal(sesAgent.session)
	sesAgent.lock.RUnlock()
	if err != nil {
		return nil, err
	} // 进行解码
	return data, nil
}
func (sesAgent *sessionagent) Unmarshal(data []byte) error {
	se := &SessionImp{}
	err := proto.Unmarshal(data, se)
	if err != nil {
		return err
	} // 测试结果
	sesAgent.session = se
	if sesAgent.session.GetSettings() == nil {
		sesAgent.lock.Lock()
		sesAgent.session.Settings = make(map[string]string)
		sesAgent.lock.Unlock()
	}
	return nil
}
func (sesAgent *sessionagent) String() string {
	return "gate.Session"
}

// gateServer 不允许旧会话回退到同名 Gate 的其他实例。
func (sesAgent *sessionagent) gateServer() (module.ServerSession, error) {
	if sesAgent.app == nil {
		return nil, errorAppIsNil
	}
	instanceID := sesAgent.GetGateInstanceID()
	if instanceID == "" {
		return nil, errors.New("session GateInstanceID is empty")
	}
	server, err := sesAgent.app.GetServerByInstanceID(instanceID)
	if err != nil {
		return nil, fmt.Errorf("gate instance not found (%s): %v", instanceID, err)
	}
	return server, nil
}

func (sesAgent *sessionagent) Update() (err error) {
	if sesAgent.app == nil {
		return errorAppIsNil
	}
	server, e := sesAgent.gateServer()
	if e != nil {
		return e
	}
	result, err := server.Call(nil, "Update", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), sesAgent.session.SessionId)
	if err == nil {
		if result != nil {
			//绑定成功,重新更新当前Session
			sesAgent.update(result.(gate.Session))
		}
	}

	return
}

func (sesAgent *sessionagent) Bind(userID string) (err error) {
	if sesAgent.app == nil {
		err = errors.New("Module.App is nil")
		return
	}
	server, e := sesAgent.gateServer()
	if e != nil {
		err = e
		return
	}
	result, err := server.Call(nil, "Bind", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), sesAgent.session.SessionId, userID)
	if err == nil {
		if result != nil {
			//绑定成功,重新更新当前Session
			sesAgent.update(result.(gate.Session))
		}
	}
	return
}

func (sesAgent *sessionagent) UnBind() (err error) {
	if sesAgent.app == nil {
		return errorAppIsNil
	}
	server, e := sesAgent.gateServer()
	if e != nil {
		return e
	}
	result, err := server.Call(nil, "UnBind", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), sesAgent.session.SessionId)
	if err == nil {
		if result != nil {
			//绑定成功,重新更新当前Session
			sesAgent.update(result.(gate.Session))
		}
	}
	return
}

func (sesAgent *sessionagent) Push() (err error) {
	if sesAgent.app == nil {
		return errorAppIsNil
	}
	server, e := sesAgent.gateServer()
	if e != nil {
		return e
	}
	sesAgent.lock.Lock()
	tmp := map[string]string{}
	for k, v := range sesAgent.session.Settings {
		tmp[k] = v
	}
	sesAgent.lock.Unlock()
	result, err := server.Call(nil, "Push", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), sesAgent.session.SessionId, tmp)
	if err == nil {
		if result != nil {
			//绑定成功,重新更新当前Session
			sesAgent.update(result.(gate.Session))
		}
	}
	return
}

func (sesAgent *sessionagent) Set(key string, value string) (err error) {
	if sesAgent.app == nil {
		return errorAppIsNil
	}
	server, err := sesAgent.gateServer()
	if err != nil {
		return err
	}
	result, err := server.Call(nil, "Set", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), sesAgent.session.SessionId, key, value)
	if err == nil {
		if result != nil {
			//绑定成功,重新更新当前Session
			sesAgent.update(result.(gate.Session))
		}
	}
	return
}
func (sesAgent *sessionagent) SetPush(key string, value string) (err error) {
	if sesAgent.app == nil {
		return errorAppIsNil
	}
	if sesAgent.session.Settings == nil {
		sesAgent.session.Settings = map[string]string{}
	}
	sesAgent.lock.Lock()
	sesAgent.session.Settings[key] = value
	sesAgent.lock.Unlock()
	return sesAgent.Push()
}
func (sesAgent *sessionagent) SetBatch(settings map[string]string) (err error) {
	if sesAgent.app == nil {
		return errorAppIsNil
	}
	server, e := sesAgent.gateServer()
	if e != nil {
		return e
	}
	result, err := server.Call(nil, "Push", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), sesAgent.session.SessionId, settings)
	if err == nil {
		if result != nil {
			//绑定成功,重新更新当前Session
			sesAgent.update(result.(gate.Session))
		}
	}
	return
}
func (sesAgent *sessionagent) Get(key string) (result string) {
	sesAgent.lock.RLock()
	if sesAgent.session.Settings == nil {
		sesAgent.lock.RUnlock()
		return
	}
	result = sesAgent.session.Settings[key]
	sesAgent.lock.RUnlock()
	return
}

func (sesAgent *sessionagent) Load(key string) (result string, ok bool) {
	sesAgent.lock.RLock()
	defer sesAgent.lock.RUnlock()
	if sesAgent.session.Settings == nil {
		return "", false
	}
	if result, ok = sesAgent.session.Settings[key]; ok {
		return result, ok
	} else {
		return "", false
	}
}

func (sesAgent *sessionagent) Remove(key string) (err error) {
	if sesAgent.app == nil {
		return errorAppIsNil
	}
	server, err := sesAgent.gateServer()
	if err != nil {
		return err
	}
	result, err := server.Call(nil, "Remove", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), sesAgent.session.SessionId, key)
	if err == nil {
		if result != nil {
			//绑定成功,重新更新当前Session
			sesAgent.update(result.(gate.Session))
		}
	}
	return
}
func (sesAgent *sessionagent) Send(topic string, body []byte) error {
	if sesAgent.app == nil {
		return errorAppIsNil
	}
	server, e := sesAgent.gateServer()
	if e != nil {
		return e
	}
	_, err := server.Call(nil, "Send", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), sesAgent.session.SessionId, topic, body)
	return err
}

func (sesAgent *sessionagent) SendBatch(Sessionids string, topic string, body []byte) (int64, error) {
	if sesAgent.app == nil {
		return 0, errorAppIsNil
	}
	server, e := sesAgent.gateServer()
	if e != nil {
		return 0, e
	}
	count, err := server.Call(nil, "SendBatch", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), Sessionids, topic, body)
	if err != nil {
		return 0, err
	}
	return count.(int64), err
}

func (sesAgent *sessionagent) IsConnect(userId string) (bool, error) {
	if sesAgent.app == nil {
		return false, errorAppIsNil
	}
	server, e := sesAgent.gateServer()
	if e != nil {
		return false, e
	}
	result, err := server.Call(nil, "IsConnect", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), sesAgent.session.SessionId, userId)
	if err != nil {
		return false, err
	}
	return result.(bool), err
}

func (sesAgent *sessionagent) SendNR(topic string, body []byte) error {
	if sesAgent.app == nil {
		return errorAppIsNil
	}
	server, e := sesAgent.gateServer()
	if e != nil {
		return e
	}
	e = server.CallNR("Send", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), sesAgent.session.SessionId, topic, body)
	if e != nil {
		return e
	}
	//span:=sesid.ExtractSpan(topic)
	//if span!=nil{
	//	span.LogEventWithPayload("SendToClient",map[string]string{
	//		"topic":topic,
	//	})
	//	span.Finish()
	//}
	return nil
}

func (sesAgent *sessionagent) Close() (err error) {
	if sesAgent.app == nil {
		return errorAppIsNil
	}
	server, err := sesAgent.gateServer()
	if err != nil {
		return err
	}

	_, err = server.Call(nil, "Close", log.CreateTrace(sesAgent.TraceId(), sesAgent.SpanId()), sesAgent.session.SessionId)
	return
}

/*
*
每次rpc调用都拷贝一份新的Session进行传输
*/
func (sesAgent *sessionagent) Clone() gate.Session {
	sesAgent.lock.Lock()
	tmp := map[string]string{}
	for k, v := range sesAgent.session.Settings {
		tmp[k] = v
	}
	agent := &sessionagent{
		app:      sesAgent.app,
		userdata: sesAgent.userdata,
		lock:     new(sync.RWMutex),
	}
	se := &SessionImp{
		IP:             sesAgent.session.IP,
		Network:        sesAgent.session.Network,
		UserId:         sesAgent.session.UserId,
		SessionId:      sesAgent.session.SessionId,
		ServerId:       sesAgent.session.ServerId,
		GateInstanceId: sesAgent.session.GateInstanceId,
		TraceId:        sesAgent.session.TraceId,
		SpanId:         mqanttools.GenerateID().String(),
		Settings:       tmp,
	}
	agent.session = se
	sesAgent.lock.Unlock()
	return agent
}

func (sesAgent *sessionagent) CreateTrace() {
	sesAgent.session.TraceId = mqanttools.GenerateID().String()
	sesAgent.session.SpanId = mqanttools.GenerateID().String()
}

func (sesAgent *sessionagent) TraceID() string {
	return sesAgent.session.TraceId
}

func (sesAgent *sessionagent) TraceId() string {
	return sesAgent.TraceID()
}

func (sesAgent *sessionagent) SpanID() string {
	return sesAgent.session.SpanId
}

func (sesAgent *sessionagent) SpanId() string {
	return sesAgent.SpanID()
}

func (sesAgent *sessionagent) ExtractSpan() log.TraceSpan {
	a := &sessionagent{
		app:      sesAgent.app,
		userdata: sesAgent.userdata,
		lock:     new(sync.RWMutex),
	}
	se := &SessionImp{
		IP:             sesAgent.session.IP,
		Network:        sesAgent.session.Network,
		UserId:         sesAgent.session.UserId,
		SessionId:      sesAgent.session.SessionId,
		ServerId:       sesAgent.session.ServerId,
		GateInstanceId: sesAgent.session.GateInstanceId,
		TraceId:        sesAgent.session.TraceId,
		SpanId:         mqanttools.GenerateID().String(),
		Settings:       sesAgent.session.Settings,
	}
	a.session = se
	return a
}

// 是否是访客(未登录) ,默认判断规则为 userId==""代表访客
func (sesAgent *sessionagent) IsGuest() bool {
	if sesAgent.judgeGuest != nil {
		return sesAgent.judgeGuest(sesAgent)
	}
	if sesAgent.GetUserId() == "" {
		return true
	}
	return false
}

// 设置自动的访客判断函数,记得一定要在gate模块设置这个值,以免部分模块因为未设置这个判断函数造成错误的判断
func (sesAgent *sessionagent) JudgeGuest(judgeGuest func(session gate.Session) bool) {
	sesAgent.judgeGuest = judgeGuest
}
