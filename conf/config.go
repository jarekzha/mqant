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

package conf

var (
	// LenStackBuf 异常堆栈信息
	LenStackBuf = 1024
	// Conf 配置结构体
	Conf = Config{}
)

// Config 配置结构体
type Config struct {
	Log      map[string]interface{}       `mqant:"Log"`
	RPC      RPC                          `mqant:"RPC"`
	Module   map[string][]*ModuleSettings `mqant:"Module"`
	Mqtt     Mqtt                         `mqant:"Mqtt"`
	Settings map[string]interface{}       `mqant:"Settings"`
}

// rpc 进程间通信配置
type RPC struct {
	// 模块同时可以创建的最大协程数量默认是100
	MaxCoroutine int `mqant:"MaxCoroutine"`
	// 远程访问最后期限值 单位秒[默认5秒] 这个值指定了在客户端可以等待服务端多长时间来应答
	RPCExpired int `mqant:"RPCExpired"`
	// 是否打印RPC的日志
	Log bool `mqant:"Log"`
}

// ModuleSettings 模块配置
type ModuleSettings struct {
	ID        string                 `mqant:"ID"`
	Host      string                 `mqant:"Host"`
	ProcessID string                 `mqant:"ProcessID"`
	Settings  map[string]interface{} `mqant:"Settings"`
}

// Mqtt mqtt协议配置
type Mqtt struct {
	WriteLoopChanNum int `mqant:"WriteLoopChanNum"` // 最大写入包队列缓存 (1,+∞)
	ReadPackLoop     int `mqant:"ReadPackLoop"`     // 最大读取包队列缓存
	ReadTimeout      int `mqant:"ReadTimeout"`      // 读取超时
	WriteTimeout     int `mqant:"WriteTimeout"`     // 写入超时
}
