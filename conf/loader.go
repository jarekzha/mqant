/**
 * description: 配置加载
 * author: jarekzha@gmail.com
 * date: Aug 31, 2022
 */
package conf

import (
	"fmt"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
)

var k = koanf.New(".")

// LoadConfig 加载配置
func LoadConfig(path string) {
	// Read config.
	if err := readFileInto(path); err != nil {
		panic(err)
	}
	if Conf.RPC.RPCExpired == 0 {
		Conf.RPC.RPCExpired = 3
	}
	if Conf.RPC.MaxCoroutine == 0 {
		Conf.RPC.MaxCoroutine = 100
	}
}
func readFileInto(path string) error {
	if e := k.Load(file.Provider(path), json.Parser()); e != nil {
		return fmt.Errorf("loading path %s fail: %s", path, e.Error())
	}

	fmt.Println(k.All())

	k.UnmarshalWithConf("", &Conf, koanf.UnmarshalConf{Tag: "mqant"})
	fmt.Println(Conf)

	return nil
}
