package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

/*
负责加载和验证服务端、客户端的配置文件
*/

// ServerConfig 服务端配置
type ServerConfig struct {
	Server ServerSettings `yaml:"server"`
}

// ServerSettings 服务端详细设置
type ServerSettings struct {
	ControlAddr       string        `yaml:"control_addr"`
	Token             string        `yaml:"token"`
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	HeartbeatTimeout  time.Duration `yaml:"heartbeat_timeout"`
	LogLevel          string        `yaml:"log_level"`
	PublicPorts       []int         `yaml:"public_ports"` // 允许客户端使用的端口白名单，为空则允许所有端口
}

type ClientConfig struct {
	Client ClientSettings `yaml:"client"`
}

// ClientSettings 客户端详细设置
type ClientSettings struct {
	ServerAddr        string         `yaml:"server_addr"`
	Token             string         `yaml:"token"`
	HeartbeatInterval time.Duration  `yaml:"heartbeat_interval"`
	LogLevel          string         `yaml:"log_level"`
	Tunnels           []TunnelConfig `yaml:"tunnels"`
}

// TunnelConfig 单个隧道配置
type TunnelConfig struct {
	Name       string `yaml:"name"`
	LocalAddr  string `yaml:"local_addr"`
	RemotePort int    `yaml:"remote_port"`
}

// Validate 验证服务端配置
func (c *ServerConfig) Validate() error {
	if c.Server.ControlAddr == "" {
		return fmt.Errorf("server.control_addr is required")
	}
	if c.Server.Token == "" {
		return fmt.Errorf("server.token is required")
	}
	if c.Server.HeartbeatInterval <= 0 {
		c.Server.HeartbeatInterval = 30 * time.Second // 默认30秒
	}
	if c.Server.HeartbeatTimeout <= 0 {
		c.Server.HeartbeatTimeout = 90 * time.Second // 默认90秒
	}
	return nil
}

// Validate 验证客户端配置
func (c *ClientConfig) Validate() error {
	if c.Client.ServerAddr == "" {
		return fmt.Errorf("client.server_addr is required")
	}
	if c.Client.Token == "" {
		return fmt.Errorf("client.token is required")
	}
	if len(c.Client.Tunnels) == 0 {
		return fmt.Errorf("client.tunnels is required, at least one tunnel")
	}
	if c.Client.HeartbeatInterval <= 0 {
		c.Client.HeartbeatInterval = 30 * time.Second
	}

	// 验证每个隧道配置
	for i, t := range c.Client.Tunnels {
		if t.Name == "" {
			return fmt.Errorf("tunnel[%d].name is required", i)
		}
		if t.LocalAddr == "" {
			return fmt.Errorf("tunnel[%d].local_addr is required", i)
		}
		if t.RemotePort <= 0 || t.RemotePort > 65535 {
			return fmt.Errorf("tunnel[%d].remote_port must be between 1 and 65535", i)
		}
	}
	return nil
}

// LoadServerConfig 加载服务端配置
func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ServerConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// LoadClientConfig 加载客户端配置
func LoadClientConfig(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ClientConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}
