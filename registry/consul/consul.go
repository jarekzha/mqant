package consul

import (
	"github.com/jarekzha/mqant/registry"
)

func NewRegistry(opts ...registry.Option) registry.Registry {
	return registry.NewRegistry(opts...)
}
