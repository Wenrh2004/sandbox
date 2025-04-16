package config

import "github.com/spf13/viper"

type Config interface {
	GetConfig() *viper.Viper
}

func NewConfig(conf *viper.Viper) Config {
	if conf.Get("app.config.nacos") != nil {
		return NewNacosConfig(conf)
	}
	return nil
}
