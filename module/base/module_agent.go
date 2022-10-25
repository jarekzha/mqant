// Copyright 2014 loolgame Author. All Rights Reserved.
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

// Package basemodule 模块管理
package basemodule

import (
	"runtime"
	"sync"

	"github.com/jarekzha/mqant/conf"
	"github.com/jarekzha/mqant/log"
	"github.com/jarekzha/mqant/module"
)

// ModuleAgent 模块结构
type ModuleAgent struct {
	mi       module.Module
	settings *conf.ModuleSettings
	closeSig chan bool
	wg       sync.WaitGroup
}

func run(m *ModuleAgent) {
	defer func() {
		if r := recover(); r != nil {
			if conf.LenStackBuf > 0 {
				buf := make([]byte, conf.LenStackBuf)
				l := runtime.Stack(buf, false)
				log.Errorf("%v: %s", r, buf[:l])
			} else {
				log.Errorf("%v", r)
			}
		}
	}()
	m.mi.Run(m.closeSig)
	m.wg.Done()
}

func destroy(m *ModuleAgent) {
	defer func() {
		if r := recover(); r != nil {
			if conf.LenStackBuf > 0 {
				buf := make([]byte, conf.LenStackBuf)
				l := runtime.Stack(buf, false)
				log.Errorf("%v: %s", r, buf[:l])
			} else {
				log.Errorf("%v", r)
			}
		}
	}()
	m.mi.OnDestroy()
}
