package config

import (
	"strings"
	
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/spf13/viper"
)

type NacosConfig struct {
	conf *viper.Viper
	cli  config_client.IConfigClient
}

func (n NacosConfig) GetConfig() *viper.Viper {
	config, err := n.cli.GetConfig(vo.ConfigParam{
		DataId: n.conf.GetString("app.config.nacos.data_id"),
		Group:  n.conf.GetString("app.config.nacos.group"),
	})
	if err != nil {
		panic(err)
	}
	
	// 使用 viper 解析 YAML 配置
	v := viper.New()
	v.SetConfigType("yaml")
	err = v.ReadConfig(strings.NewReader(config))
	if err != nil {
		panic(err)
	}
	
	return v
}

func NewNacosConfig(conf *viper.Viper) *NacosConfig {
	cc := constant.ClientConfig{
		NamespaceId:         conf.GetString("app.config.nacos.namespace"),
		TimeoutMs:           conf.GetUint64("app.config.nacos.timeout"),
		NotLoadCacheAtStart: conf.GetBool("app.register.nacos.is_load_cache"),
		LogDir:              conf.GetString("app.register.nacos.log_dir"),
		CacheDir:            conf.GetString("app.register.nacos.cache_dir"),
		LogLevel:            conf.GetString("app.register.nacos.log_level"),
	}
	
	sc := []constant.ServerConfig{
		*constant.NewServerConfig(conf.GetString("app.config.nacos.addr"), conf.GetUint64("app.config.nacos.port")),
	}
	// a more graceful way to create config client
	client, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)
	if err != nil {
		panic(err)
	}
	
	return &NacosConfig{
		conf: conf,
		cli:  client,
	}
}
