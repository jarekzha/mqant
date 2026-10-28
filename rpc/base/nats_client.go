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
package defaultrpc

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/jarekzha/mqant/log"
	"github.com/jarekzha/mqant/module"
	mqrpc "github.com/jarekzha/mqant/rpc"
	rpcpb "github.com/jarekzha/mqant/rpc/pb"
	mqanttools "github.com/jarekzha/mqant/utils"
	"github.com/nats-io/nats.go"

	"google.golang.org/protobuf/proto"
)

type NatsClient struct {
	//callinfos map[string]*ClinetCallInfo
	callinfos         *mqanttools.BeeMap
	cmutex            sync.Mutex // 保护关闭状态、订阅和回调归属
	callbackqueueName string
	app               module.App
	done              chan struct{}
	stopped           chan struct{}
	subs              *nats.Subscription
	isClose           bool
	session           module.ServerSession
}

func NewNatsClient(app module.App, session module.ServerSession) (client *NatsClient, err error) {
	client = new(NatsClient)
	client.session = session
	client.app = app
	client.callinfos = mqanttools.NewBeeMap()
	client.callbackqueueName = nats.NewInbox()
	client.done = make(chan struct{})
	client.stopped = make(chan struct{})
	client.isClose = false
	client.subs, err = subscribeNatsReady(app.Transport(), client.callbackqueueName)
	if err != nil {
		return nil, fmt.Errorf("initialize RPC reply subscription: %v", err)
	}
	go client.on_request_handle()
	return client, nil
}

func (c *NatsClient) Delete(key string) (err error) {
	c.cmutex.Lock()
	defer c.cmutex.Unlock()
	c.callinfos.Delete(key)
	return nil
}
func (c *NatsClient) CloseFch(fch chan *rpcpb.ResultInfo) {
	defer func() {
		if recover() != nil {
			// 保留兼容保护；内部通过回调归属保证只关闭一次。
		}
	}()
	close(fch)
}

// stop 先发布关闭状态，再取消订阅，禁止接收循环在退出时重建订阅。
func (c *NatsClient) stop() {
	c.cmutex.Lock()
	defer c.cmutex.Unlock()
	if c.isClose {
		return
	}
	c.isClose = true
	close(c.done)
	_ = c.subs.Unsubscribe()
}

func (c *NatsClient) Done() error {
	c.stop()
	<-c.stopped
	return nil
}

// finish 在接收协程退出时释放剩余调用，Done 返回后不再有回调写入。
func (c *NatsClient) finish() {
	c.stop()
	c.cmutex.Lock()
	for key, info := range c.callinfos.Items() {
		if info != nil {
			c.CloseFch(info.(ClinetCallInfo).call)
			c.callinfos.Delete(key)
		}
	}
	c.cmutex.Unlock()
	close(c.stopped)
}

/*
*
消息请求
*/
func (c *NatsClient) Call(callInfo *mqrpc.CallInfo, callback chan *rpcpb.ResultInfo) error {
	c.cmutex.Lock()
	defer c.cmutex.Unlock()
	if c.isClose {
		return fmt.Errorf("NatsClient is closed")
	}
	callInfo.RPCInfo.ReplyTo = c.callbackqueueName
	body, err := c.Marshal(callInfo.RPCInfo)
	if err != nil {
		return err
	}
	correlationID := callInfo.RPCInfo.Cid
	c.callinfos.Set(correlationID, ClinetCallInfo{
		correlation_id: correlationID,
		call:           callback,
		timeout:        callInfo.RPCInfo.Expired,
	})
	if err = c.app.Transport().Publish(c.session.GetNode().Address, body); err != nil {
		c.callinfos.Delete(correlationID)
	}
	return err
}

/*
*
消息请求 不需要回复
*/
func (c *NatsClient) CallNR(callInfo *mqrpc.CallInfo) error {
	c.cmutex.Lock()
	defer c.cmutex.Unlock()
	if c.isClose {
		return fmt.Errorf("NatsClient is closed")
	}
	body, err := c.Marshal(callInfo.RPCInfo)
	if err != nil {
		return err
	}
	return c.app.Transport().Publish(c.session.GetNode().Address, body)
}

/*
*
接收应答信息
*/
func (c *NatsClient) on_request_handle() (err error) {
	defer c.finish()
	defer func() {
		if r := recover(); r != nil {
			var rn = ""
			switch r.(type) {

			case string:
				rn = r.(string)
			case error:
				rn = r.(error).Error()
			}
			buf := make([]byte, 1024)
			l := runtime.Stack(buf, false)
			errstr := string(buf[:l])
			log.Errorf("%s\n ----Stack----\n%s", rn, errstr)
			fmt.Println(errstr)
		}
	}()
	for {
		select {
		case <-c.done:
			return nil
		default:
		}
		// 只有此协程重建订阅；stop 在同一把锁下取消当前订阅。
		c.cmutex.Lock()
		sub := c.subs
		c.cmutex.Unlock()
		m, err := sub.NextMsg(time.Minute)
		if err != nil {
			c.cmutex.Lock()
			if c.isClose {
				c.cmutex.Unlock()
				return nil
			}
			if err != nats.ErrTimeout {
				log.Error("NatsClient receive reply fail",
					log.String("instanceID", c.session.GetInstanceID()),
					log.String("subscription", c.callbackqueueName), log.Err(err))
			}
			if c.app.Transport().IsClosed() {
				c.cmutex.Unlock()
				return err
			}
			if !sub.IsValid() {
				var replacement *nats.Subscription
				replacement, err = subscribeNatsReady(c.app.Transport(), c.callbackqueueName)
				if err == nil {
					c.subs = replacement
				} else {
					log.Error("NatsClient restore reply subscription fail",
						log.String("instanceID", c.session.GetInstanceID()),
						log.String("subscription", c.callbackqueueName), log.Err(err))
				}
			}
			c.cmutex.Unlock()
			if err != nil && err != nats.ErrTimeout {
				// 重建失败时退避，避免断线期间空转刷日志；关闭可立即打断。
				timer := time.NewTimer(100 * time.Millisecond)
				select {
				case <-c.done:
					timer.Stop()
					return nil
				case <-timer.C:
				}
			}
			continue
		}

		resultInfo, err := c.UnmarshalResult(m.Data)
		if err != nil {
			log.Error("Unmarshal fail", log.Err(err))
			continue
		}
		c.cmutex.Lock()
		info := c.callinfos.Get(resultInfo.Cid)
		c.callinfos.Delete(resultInfo.Cid)
		c.cmutex.Unlock()
		if info == nil {
			log.Warnf("rpc callback no found : [%s]", resultInfo.Cid)
			continue
		}
		// 取出后由接收协程独占回调，超时方只删除映射，不再关闭通道。
		if callback := info.(ClinetCallInfo).call; callback != nil {
			select {
			case callback <- resultInfo:
			case <-c.done:
			}
			c.CloseFch(callback)
		}
	}
}

func (c *NatsClient) UnmarshalResult(data []byte) (*rpcpb.ResultInfo, error) {
	//fmt.Println(msg)
	//保存解码后的数据，Value可以为任意数据类型
	var resultInfo rpcpb.ResultInfo
	err := proto.Unmarshal(data, &resultInfo)
	if err != nil {
		return nil, err
	} else {
		return &resultInfo, err
	}
}

func (c *NatsClient) Unmarshal(data []byte) (*rpcpb.RPCInfo, error) {
	//fmt.Println(msg)
	//保存解码后的数据，Value可以为任意数据类型
	var rpcInfo rpcpb.RPCInfo
	err := proto.Unmarshal(data, &rpcInfo)
	if err != nil {
		return nil, err
	} else {
		return &rpcInfo, err
	}
}

// goroutine safe
func (c *NatsClient) Marshal(rpcInfo *rpcpb.RPCInfo) ([]byte, error) {
	//map2:= structs.Map(callInfo)
	b, err := proto.Marshal(rpcInfo)
	return b, err
}
