package rpc

import (
	kitexregistry "github.com/cloudwego/kitex/pkg/registry"
	"github.com/spf13/viper"
	
	"github.com/Wenrh2004/sandbox/pkg/application/register/rpc/nacos"
)

func NewRegister(conf *viper.Viper) kitexregistry.Registry {
	if conf.Get("app.register.nacos") != nil {
		return nacos.NewNacosRegister(conf)
	}
	return nil
}
