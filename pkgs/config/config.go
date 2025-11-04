package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Server struct {
	Address string
	Port    uint16
	Type    string
}

type Configuration struct {
	Server Server

	// CurrentLoco describes a contextual configuration of current locomotive
	Loco Loco
}

type Loco struct {
	LocoAddr         uint16
	DecoderType      string
	RailboxSoundSlot uint8
}

// LocoAddr represents locomotive address
type LocoAddr uint16

func NewConfig() (*Configuration, error) {
	config := Configuration{}
	config.Loco = Loco{}

	// application configuration
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigName(".rb")
	v.AddConfigPath("$HOME/")
	v.AddConfigPath(".")
	_ = v.SafeWriteConfig()

	v.SetDefault("server.address", "192.168.0.111")
	v.SetDefault("server.port", 21105)
	v.SetDefault("server.type", "z21")

	// contextual locomotive configuration (when current working directory is a locomotive directory that contains loco.json file)
	l := viper.New()
	l.SetConfigType("json")
	l.SetConfigName("loco")
	l.AddConfigPath(".")
	l.ReadInConfig()

	// read both configuration files
	if err := v.ReadInConfig(); err != nil {
		return &Configuration{}, fmt.Errorf("cannot parse config: %s", err.Error())
	}
	if err := v.Unmarshal(&config); err != nil {
		return &config, fmt.Errorf("cannot parse config: %s", err.Error())
	}
	if err := l.ReadInConfig(); err != nil {
		// make loco.json fully optional
		if !strings.Contains(err.Error(), "Not Found") {
			return &Configuration{}, fmt.Errorf("cannot parse config: %s", err.Error())
		}
	}
	if err := l.Unmarshal(&config.Loco); err != nil {
		return &config, fmt.Errorf("cannot parse config: %s", err.Error())
	}

	return &config, nil
}
