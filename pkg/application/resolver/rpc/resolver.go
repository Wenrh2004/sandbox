package rpc

import (
	"github.com/cloudwego/kitex/pkg/discovery"
	consulapi "github.com/hashicorp/consul/api"
	consul "github.com/kitex-contrib/registry-consul"
	"github.com/spf13/viper"
)

func NewRPCResolver(conf *viper.Viper) discovery.Resolver {
	c := &consulapi.Config{
		Address: conf.GetString("app.config.consul.address"),
		Scheme:  conf.GetString("app.config.consul.scheme"),
		Token:   conf.GetString("app.config.consul.token"),
	}
	r, err := consul.NewConsulResolverWithConfig(c)
	if err != nil {
		panic(err)
	}
	return r
}
