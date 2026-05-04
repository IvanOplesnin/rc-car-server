package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Web    WebConfig    `yaml:"web"`
	Motor  MotorConfig  `yaml:"motor"`
	Camera CameraConfig `yaml:"camera"`
	Safety SafetyConfig `yaml:"safety"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type WebConfig struct {
	StaticDir string `yaml:"static_dir"`
}

type MotorConfig struct {
	Address string `yaml:"address"`
}

type CameraConfig struct {
	StreamURL       string `yaml:"stream_url"`
	CheckIntervalMS int    `yaml:"check_interval_ms"`
	CheckTimeoutMS  int    `yaml:"check_timeout_ms"`
}

type SafetyConfig struct {
	CommandTimeoutMS int `yaml:"command_timeout_ms"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Server.Host == "" {
		return fmt.Errorf("server.host is required")
	}

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}

	if c.Web.StaticDir == "" {
		return fmt.Errorf("web.static_dir is required")
	}

	if c.Safety.CommandTimeoutMS <= 0 {
		return fmt.Errorf("safety.command_timeout_ms must be positive")
	}

	if c.Camera.CheckIntervalMS <= 0 {
		return fmt.Errorf("camera.check_interval_ms must be positive")
	}
	
	if c.Camera.CheckTimeoutMS <= 0 {
		return fmt.Errorf("camera.check_timeout_ms must be positive")
	}

	return nil
}

func (c *Config) HTTPAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}