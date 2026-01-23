package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadServerConfig 测试服务端配置加载
func TestLoadServerConfig(t *testing.T) {
	// 创建临时配置文件
	content := `
server:
  control_addr: "0.0.0.0:7000"
  token: "my-secret-token"
  heartbeat_interval: 30s
  heartbeat_timeout: 90s
  log_level: "info"
`
	tmpFile := createTempFile(t, "server-*.yaml", content)
	defer os.Remove(tmpFile)

	// 加载配置
	config, err := LoadServerConfig(tmpFile)
	if err != nil {
		t.Fatalf("LoadServerConfig failed: %v", err)
	}

	// 验证配置值
	if config.Server.ControlAddr != "0.0.0.0:7000" {
		t.Errorf("ControlAddr = %q, want %q", config.Server.ControlAddr, "0.0.0.0:7000")
	}
	if config.Server.Token != "my-secret-token" {
		t.Errorf("Token = %q, want %q", config.Server.Token, "my-secret-token")
	}
	if config.Server.HeartbeatInterval != 30*time.Second {
		t.Errorf("HeartbeatInterval = %v, want %v", config.Server.HeartbeatInterval, 30*time.Second)
	}
}

// TestLoadClientConfig 测试客户端配置加载
func TestLoadClientConfig(t *testing.T) {
	content := `
client:
  server_addr: "your-server-ip:7000"
  token: "my-secret-token"
  heartbeat_interval: 30s
  log_level: "debug"
  tunnels:
    - name: "web"
      local_addr: "127.0.0.1:8080"
      remote_port: 8080
    - name: "ssh"
      local_addr: "127.0.0.1:22"
      remote_port: 2222
`
	tmpFile := createTempFile(t, "client-*.yaml", content)
	defer os.Remove(tmpFile)

	config, err := LoadClientConfig(tmpFile)
	if err != nil {
		t.Fatalf("LoadClientConfig failed: %v", err)
	}

	// 验证基本配置
	if config.Client.ServerAddr != "your-server-ip:7000" {
		t.Errorf("ServerAddr = %q, want %q", config.Client.ServerAddr, "your-server-ip:7000")
	}

	// 验证隧道配置
	if len(config.Client.Tunnels) != 2 {
		t.Fatalf("Tunnels count = %d, want 2", len(config.Client.Tunnels))
	}
	if config.Client.Tunnels[0].Name != "web" {
		t.Errorf("Tunnel[0].Name = %q, want %q", config.Client.Tunnels[0].Name, "web")
	}
	if config.Client.Tunnels[1].RemotePort != 2222 {
		t.Errorf("Tunnel[1].RemotePort = %d, want %d", config.Client.Tunnels[1].RemotePort, 2222)
	}
}

// TestServerConfigValidation 测试服务端配置验证
func TestServerConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid config",
			content: `
server:
  control_addr: "0.0.0.0:7000"
  token: "secret"
`,
			wantErr: false,
		},
		{
			name: "missing control_addr",
			content: `
server:
  token: "secret"
`,
			wantErr: true,
		},
		{
			name: "missing token",
			content: `
server:
  control_addr: "0.0.0.0:7000"
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, "server-*.yaml", tt.content)
			defer os.Remove(tmpFile)

			_, err := LoadServerConfig(tmpFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadServerConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestClientConfigValidation 测试客户端配置验证
func TestClientConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid config",
			content: `
client:
  server_addr: "server:7000"
  token: "secret"
  tunnels:
    - name: "web"
      local_addr: "127.0.0.1:80"
      remote_port: 8080
`,
			wantErr: false,
		},
		{
			name: "missing tunnels",
			content: `
client:
  server_addr: "server:7000"
  token: "secret"
`,
			wantErr: true,
		},
		{
			name: "invalid remote_port",
			content: `
client:
  server_addr: "server:7000"
  token: "secret"
  tunnels:
    - name: "web"
      local_addr: "127.0.0.1:80"
      remote_port: 99999
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, "client-*.yaml", tt.content)
			defer os.Remove(tmpFile)

			_, err := LoadClientConfig(tmpFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadClientConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// createTempFile 创建临时文件的辅助函数
func createTempFile(t *testing.T, pattern, content string) string {
	t.Helper()
	tmpFile := filepath.Join(t.TempDir(), pattern)
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	return tmpFile
}
