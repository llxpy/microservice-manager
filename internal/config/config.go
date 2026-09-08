package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	ScanDir         string        `yaml:"scanDir"`
	ScanInterval    time.Duration `yaml:"scanInterval"`
	LogDir          string        `yaml:"logDir"`
	LogMaxSizeMB    int64         `yaml:"logMaxSizeMB"`
	MetricsInterval time.Duration `yaml:"metricsInterval"`
	HealthInterval  time.Duration `yaml:"healthInterval"`
	WsPath          string        `yaml:"wsPath"`
	ApiPrefix       string        `yaml:"apiPrefix"`
	RestartDelay    time.Duration `yaml:"restartDelay"`
	StopTimeout     time.Duration `yaml:"stopTimeout"`
}

func defaults(c *Config) {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 9090
	}
	if c.ScanInterval == 0 {
		c.ScanInterval = 30 * time.Second
	}
	if c.LogDir == "" {
		c.LogDir = "./logs"
	}
	if c.LogMaxSizeMB == 0 {
		c.LogMaxSizeMB = 50
	}
	if c.MetricsInterval == 0 {
		c.MetricsInterval = 3 * time.Second
	}
	if c.HealthInterval == 0 {
		c.HealthInterval = 10 * time.Second
	}
	if c.WsPath == "" {
		c.WsPath = "/ws"
	}
	if c.ApiPrefix == "" {
		c.ApiPrefix = "/api"
	}
	if c.RestartDelay == 0 {
		c.RestartDelay = 5 * time.Second
	}
	if c.StopTimeout == 0 {
		c.StopTimeout = 10 * time.Second
	}
}

func Load(path string) (*Config, error) {
	c := &Config{}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, c); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	defaults(c)
	return c, nil
}
