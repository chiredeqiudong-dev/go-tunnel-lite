package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/client"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/config"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/log"
)

var (
	configFile = flag.String("c", "client.yaml", "配置文件路径")
	showHelp   = flag.Bool("h", false, "显示帮助信息")
)

func main() {
	flag.Parse()

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	// 加载配置
	cfg, err := config.LoadClientConfig(*configFile)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	log.Info("========================================")
	log.Info("  Go-Tunnel-Lite Client 启动中...")
	log.Info("========================================")

	// 创建客户端
	cli := client.NewClient(cfg)

	// 启动客户端（连接服务端）
	if err := cli.Start(); err != nil {
		log.Error("客户端启动失败", "error", err)
		os.Exit(1)
	}

	log.Info("客户端启动成功!")
	log.Info("已连接服务端", "addr", cfg.Client.ServerAddr)
	log.Info("注册隧道数量", "count", len(cfg.Client.Tunnels))
	for _, t := range cfg.Client.Tunnels {
		log.Info("隧道", "name", t.Name, "remote_port", t.RemotePort, "local_addr", t.LocalAddr)
	}
	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Info("收到信号，正在关闭客户端...", "signal", sig)

	// 优雅关闭
	cli.Stop()
	log.Info("客户端已关闭")
}

func printUsage() {
	fmt.Println(`
Go-Tunnel-Lite Client - 内网穿透客户端

用法:
  go-tunnel-client [选项]

选项:
  -c string    配置文件路径 (默认 "client.yaml")
  -h           显示帮助信息

示例:
  go-tunnel-client -c /etc/tunnel/client.yaml

配置文件格式 (YAML):
  client:
    server_addr: "127.0.0.1:7000"   # 服务端地址
    token: "your-secret-token"      # 认证令牌
    heartbeat_interval: 30          # 心跳间隔(秒)
  
  tunnels:
    - name: "web"                   # 隧道名称
      local_addr: "127.0.0.1:8080"  # 本地服务地址
      remote_port: 8080             # 远程暴露端口
    
    - name: "ssh"
      local_addr: "127.0.0.1:22"
      remote_port: 2222
  
  log:
    level: "info"                   # 日志级别
    file: ""                        # 日志文件`)
}
