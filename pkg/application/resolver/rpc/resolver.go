package rpc

import (
	"github.com/cloudwego/kitex/pkg/discovery"
	consulapi "github.com/hashicorp/consul/api"
	consul "github.com/kitex-contrib/registry-consul"
	"github.com/spf13/viper"
)

func NewRPCResolver(conf *viper.Viper) discovery.Resolver {
	c := &consulapi.Config{
		Address: conf.GetString("app.bootstrap.consul.address"),
		Scheme:  conf.GetString("app.bootstrap.consul.scheme"),
		Token:   conf.GetString("app.bootstrap.consul.token"),
	}
	r, err := consul.NewConsulResolverWithConfig(c)
	if err != nil {
		panic(err)
	}
	return r
}
