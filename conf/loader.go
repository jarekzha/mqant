/**
 * description: 配置加载
 * author: jarekzha@gmail.com
 * date: Aug 31, 2022
 */
package conf

import (
	"fmt"
	"log"
	"path"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
)

var k = koanf.New(".")

// LoadConfig 加载配置
func LoadConfig(configFileUrl string) {
	// 读取配置文件
	readFromFile(configFileUrl)

	// 写入到struct中
	k.UnmarshalWithConf("", &Conf, koanf.UnmarshalConf{Tag: "mqant"})
	fmt.Println(Conf)

	// 默认值
	if Conf.RPC.RPCExpired == 0 {
		Conf.RPC.RPCExpired = 10
	}
}

// 从文件读取
func readFromFile(configFileUrl string) error {
	extension := path.Ext(configFileUrl)
	extension = extension[1:]
	var parser koanf.Parser
	switch extension {
	case "json":
		parser = json.Parser()
	case "yaml", "yml":
		parser = yaml.Parser()
	case "toml":
		parser = toml.Parser()
	default:
		log.Fatalf("loading config %s invalid file exension, support [json, yaml, yml, toml]", configFileUrl)
	}

	if e := k.Load(file.Provider(configFileUrl), parser); e != nil {
		log.Fatalf("loading config %s fail: %s", configFileUrl, e.Error())
	}

	fmt.Println(k.All())
	return nil
}
