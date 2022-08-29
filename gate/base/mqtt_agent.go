// Copyright 2014 mqant Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package basegate

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jarekzha/mqant/conf"
	"github.com/jarekzha/mqant/gate"
	"github.com/jarekzha/mqant/gate/base/mqtt"
	"github.com/jarekzha/mqant/module"
	"github.com/jarekzha/mqant/network"
	argsutil "github.com/jarekzha/mqant/rpc/util"
	mqanttools "github.com/jarekzha/mqant/utils"
	"go.uber.org/zap"
)

//type resultInfo struct {
//	Error  string      //错误结果 如果为nil表示请求正确
//	Result interface{} //rpc 返回结果
//}

type agent struct {
	gate.Agent
	module                       module.RPCModule
	session                      gate.Session
	conn                         network.Conn
	r                            *bufio.Reader
	w                            *bufio.Writer
	gate                         gate.Gate
	client                       *mqtt.Client
	ch                           chan int //控制模块可同时开启的最大协程数
	isclose                      bool
	protocol_ok                  bool
	lock                         sync.Mutex
	lastStorageHeartbeatDataTime time.Duration //上一次发送存储心跳时间
	revNum                       int64
	sendNum                      int64
	connTime                     time.Time
}

func NewMqttAgent(module module.RPCModule) *agent {
	a := &agent{
		module: module,
	}
	return a
}
func (age *agent) OnInit(gate gate.Gate, conn network.Conn) error {
	age.ch = make(chan int, gate.Options().ConcurrentTasks)
	age.conn = conn
	age.gate = gate
	age.r = bufio.NewReaderSize(conn, gate.Options().BufSize)
	age.w = bufio.NewWriterSize(conn, gate.Options().BufSize)
	age.isclose = false
	age.protocol_ok = false
	age.revNum = 0
	age.sendNum = 0
	age.lastStorageHeartbeatDataTime = time.Duration(time.Now().UnixNano())
	return nil
}
func (age *agent) IsClosed() bool {
	return age.isclose
}

func (age *agent) ProtocolOK() bool {
	return age.protocol_ok
}

func (age *agent) GetSession() gate.Session {
	return age.session
}

func (age *agent) Wait() error {
	// 如果ch满了则会处于阻塞，从而达到限制最大协程的功能
	select {
	case age.ch <- 1:
	//do nothing
	default:
		//warnning!
		return fmt.Errorf("the work queue is full!")
	}
	return nil
}
func (age *agent) Finish() {
	// 完成则从ch推出数据
	select {
	case <-age.ch:
	default:
	}
}

func (age *agent) Run() (err error) {
	defer func() {
		if err := recover(); err != nil {
			buff := make([]byte, 1024)
			runtime.Stack(buff, false)
			zap.L().Error("conn.serve() panic", zap.Any("err", err), zap.ByteString("info", buff))
		}
		age.Close()

	}()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				buff := make([]byte, 1024)
				runtime.Stack(buff, false)
				zap.L().Error("OverTime panic", zap.Any("err", err), zap.ByteString("info", buff))
			}
		}()
		select {
		case <-time.After(age.gate.Options().OverTime):
			if age.GetSession() == nil {
				//超过一段时间还没有建立mqtt连接则直接关闭网络连接
				age.Close()
			}

		}
	}()

	//握手协议
	var pack *mqtt.Pack
	pack, err = mqtt.ReadPack(age.r, age.gate.Options().MaxPackSize)
	if err != nil {
		zap.L().Error("Read login pack fail", zap.Error(err))
		return
	}
	if pack.GetType() != mqtt.CONNECT {
		zap.L().Error("Recive login pack's type wrong", zap.Int("type", int(pack.GetType())))
		return
	}
	conn, ok := (pack.GetVariable()).(*mqtt.Connect)
	if !ok {
		zap.L().Error("It's not age mqtt connection package")
		return
	}
	//id := info.GetUserName()
	//psw := info.GetPassword()
	//zap.L().Debug("Read login pack %s %s %s %s",*id,*psw,info.GetProtocol(),info.GetVersion())
	c := mqtt.NewClient(conf.Conf.Mqtt, age, age.r, age.w, age.conn, conn.GetKeepAlive(), age.gate.Options().MaxPackSize)
	age.client = c
	addr := age.conn.RemoteAddr()
	age.session, err = NewSessionByMap(age.module.GetApp(), map[string]interface{}{
		"Sessionid": mqanttools.GenerateID().String(),
		"Network":   addr.Network(),
		"IP":        addr.String(),
		"Serverid":  age.module.GetServerID(),
		"Settings":  make(map[string]string),
	})
	if err != nil {
		zap.L().Error("gate create agent fail", zap.Error(err))
		return
	}
	age.session.JudgeGuest(age.gate.GetJudgeGuest())
	age.session.CreateTrace() //代码跟踪
	//回复客户端 CONNECT
	err = mqtt.WritePack(mqtt.GetConnAckPack(0), age.w)
	if err != nil {
		zap.L().Error("ConnAckPack error %v", zap.Error(err))
		return
	}
	age.connTime = time.Now()
	age.protocol_ok = true
	age.gate.GetAgentLearner().Connect(age) //发送连接成功的事件
	c.Listen_loop()                         //开始监听,直到连接中断
	return nil
}

func (age *agent) OnClose() error {
	defer func() {
		if err := recover(); err != nil {
			buff := make([]byte, 1024)
			runtime.Stack(buff, false)
			zap.L().Error("agent OnClose panic", zap.Any("err", err), zap.ByteString("info", buff))
		}
	}()
	age.isclose = true
	age.gate.GetAgentLearner().DisConnect(age) //发送连接断开的事件
	return nil
}

func (age *agent) GetError() error {
	return age.client.GetError()
}

func (age *agent) RevNum() int64 {
	return age.revNum
}
func (age *agent) SendNum() int64 {
	return age.sendNum
}
func (age *agent) ConnTime() time.Time {
	return age.connTime
}
func (age *agent) OnRecover(pack *mqtt.Pack) {
	err := age.Wait()
	if err != nil {
		zap.L().Error("Gate OnRecover fail", zap.Error(err))
		pub := pack.GetVariable().(*mqtt.Publish)
		age.toResult(age, *pub.GetTopic(), nil, err)
	} else {
		go age.recoverworker(pack)
	}
}

func (age *agent) toResult(a *agent, topic string, result interface{}, err error) error {
	switch v2 := result.(type) {
	case module.ProtocolMarshal:
		return a.WriteMsg(topic, v2.GetData())
	}
	b, err := a.module.GetApp().ProtocolMarshal(a.session.TraceID(), result, err)
	if err != nil {
		if b != nil {
			return a.WriteMsg(topic, b.GetData())
		}
		return nil
	}
	br, _ := a.module.GetApp().ProtocolMarshal(a.session.TraceID(), nil, err)
	return a.WriteMsg(topic, br.GetData())
}

func (age *agent) recoverworker(pack *mqtt.Pack) {
	defer func() {
		age.lock.Lock()
		interval := int64(age.lastStorageHeartbeatDataTime) + int64(age.gate.Options().Heartbeat) //单位纳秒
		age.lock.Unlock()
		if interval < time.Now().UnixNano() {
			if age.gate.GetStorageHandler() != nil {
				age.lock.Lock()
				age.lastStorageHeartbeatDataTime = time.Duration(time.Now().UnixNano())
				age.lock.Unlock()
				age.gate.GetStorageHandler().Heartbeat(age.GetSession())
			}
		}
		age.Finish()
		if r := recover(); r != nil {
			buff := make([]byte, 1024)
			runtime.Stack(buff, false)
			zap.L().Error("Gate recoverworker panic", zap.Any("err", r), zap.ByteString("info", buff))
		}
	}()

	toResult := age.toResult
	//路由服务
	switch pack.GetType() {
	case mqtt.PUBLISH:
		age.lock.Lock()
		age.revNum = age.revNum + 1
		age.lock.Unlock()
		pub := pack.GetVariable().(*mqtt.Publish)
		if age.gate.GetRouteHandler() != nil {
			needreturn, result, err := age.gate.GetRouteHandler().OnRoute(age.GetSession(), *pub.GetTopic(), pub.GetMsg())
			if err != nil {
				if needreturn {
					toResult(age, *pub.GetTopic(), result, err)
				}
				return
			}
			if needreturn {
				toResult(age, *pub.GetTopic(), result, nil)
			}
		} else {
			topics := strings.Split(*pub.GetTopic(), "/")
			var msgid string
			if len(topics) < 2 {
				errorstr := "Topic must be [moduleType@moduleID]/[handler]|[moduleType@moduleID]/[handler]/[msgid]"
				zap.L().Error(errorstr)
				toResult(age, *pub.GetTopic(), nil, errors.New(errorstr))
				return
			} else if len(topics) == 3 {
				msgid = topics[2]
			}
			startsWith := strings.HasPrefix(topics[1], "HD_")
			if !startsWith {
				if msgid != "" {
					toResult(age, *pub.GetTopic(), nil, fmt.Errorf("Method(%s) must begin with 'HD_'", topics[1]))
				}
				return
			}
			var ArgsType []string = make([]string, 2)
			var args [][]byte = make([][]byte, 2)
			serverSession, err := age.module.GetRouteServer(topics[0])
			if err != nil {
				if msgid != "" {
					toResult(age, *pub.GetTopic(), nil, fmt.Errorf("Service(type:%s) not found", topics[0]))
				}
				return
			}
			if len(pub.GetMsg()) > 0 && pub.GetMsg()[0] == '{' && pub.GetMsg()[len(pub.GetMsg())-1] == '}' {
				//尝试解析为json为map
				var obj interface{} // var obj map[string]interface{}
				err := json.Unmarshal(pub.GetMsg(), &obj)
				if err != nil {
					if msgid != "" {
						toResult(age, *pub.GetTopic(), nil, errors.New("The JSON format is incorrect"))
					}
					return
				}
				ArgsType[1] = argsutil.MAP
				args[1] = pub.GetMsg()
			} else {
				ArgsType[1] = argsutil.BYTES
				args[1] = pub.GetMsg()
			}
			session := age.GetSession().Clone()
			session.SetTopic(*pub.GetTopic())
			if msgid != "" {
				ArgsType[0] = RPCParamSessionType
				b, err := session.Serializable()
				if err != nil {
					return
				}
				args[0] = b
				ctx, cancel := context.WithTimeout(context.TODO(), age.module.GetApp().Options().RPCExpired)
				defer cancel()
				result, e := serverSession.CallArgs(ctx, topics[1], ArgsType, args)
				toResult(age, *pub.GetTopic(), result, e)
			} else {
				ArgsType[0] = RPCParamSessionType
				b, err := session.Serializable()
				if err != nil {
					return
				}
				args[0] = b

				e := serverSession.CallNRArgs(topics[1], ArgsType, args)
				if e != nil {
					zap.L().Warn("Gate rpc fail", zap.Error(e))
				}
			}
		}
	case mqtt.PINGREQ:
		//客户端发送的心跳包
		//if age.GetSession().GetUserId() != "" {
		//这个链接已经绑定Userid
		//age.lock.Lock()
		//interval := int64(age.lastStorageHeartbeatDataTime) + int64(age.gate.Options().Heartbeat) //单位纳秒
		//age.lock.Unlock()
		//if interval < time.Now().UnixNano() {
		//	if age.gate.GetStorageHandler() != nil {
		//		age.lock.Lock()
		//		age.lastStorageHeartbeatDataTime = time.Duration(time.Now().UnixNano())
		//		age.lock.Unlock()
		//		age.gate.GetStorageHandler().Heartbeat(age.GetSession())
		//	}
		//}
		//}
	}
}

func (age *agent) WriteMsg(topic string, body []byte) error {
	if age.client == nil {
		return errors.New("mqtt.Client nil")
	}
	age.sendNum++
	if age.gate.Options().SendMessageHook != nil {
		bb, err := age.gate.Options().SendMessageHook(age.GetSession(), topic, body)
		if err != nil {
			return err
		}
		body = bb
	}
	return age.client.WriteMsg(topic, body)
}

func (age *agent) Close() {
	go func() {
		//关闭连接部分情况下会阻塞超时，因此放协程去处理
		if age.conn != nil {
			age.conn.Close()
		}
	}()
}

func (age *agent) Destroy() {
	if age.conn != nil {
		age.conn.Destroy()
	}
}
