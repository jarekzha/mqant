package registry

// Service Service
type Service struct {
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Metadata  map[string]string `json:"metadata"`
	Endpoints []*Endpoint       `json:"endpoints"`
	Nodes     []*Node           `json:"nodes"`
}

// Node 服务节点信息
type Node struct {
	ServerID   string            `json:"server_id"`   // 配置对应的逻辑 ID
	InstanceID string            `json:"instance_id"` // 每次启动唯一的实例 ID
	Address    string            `json:"address"`
	Port       int               `json:"port"`
	Metadata   map[string]string `json:"metadata"`
}

// Endpoint 服务节点信息
type Endpoint struct {
	Name     string            `json:"name"`
	Request  *Value            `json:"request"`
	Response *Value            `json:"response"`
	Metadata map[string]string `json:"metadata"`
}

// Value Value
type Value struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Values []*Value `json:"values"`
}
