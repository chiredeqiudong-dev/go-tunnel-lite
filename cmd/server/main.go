package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/config"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/log"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/server"
)

var (
	configFile = flag.String("c", "server.yaml", "配置文件路径")
	showHelp   = flag.Bool("h", false, "显示帮助信息")
)

func main() {
	flag.Parse()

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	// 加载配置
	cfg, err := config.LoadServerConfig(*configFile)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	log.Info("========================================")
	log.Info("  Go-Tunnel-Lite Server 启动中...")
	log.Info("========================================")

	// 创建服务端
	srv := server.NewServer(cfg)

	// 启动服务
	if err := srv.Start(); err != nil {
		log.Error("服务端启动失败", "error", err)
		os.Exit(1)
	}

	log.Info("服务端启动成功!")
	log.Info("控制端口", "addr", cfg.Server.ControlAddr)
	log.Info("等待客户端连接...")

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Info("收到信号，正在关闭服务...", "signal", sig)

	// 优雅关闭
	srv.Stop()

	log.Info("服务端已关闭")
}

func printUsage() {
	fmt.Println(`
Go-Tunnel-Lite Server - 内网穿透服务端

用法:
  go-tunnel-server [选项]

选项:
  -c string    配置文件路径 (默认 "server.yaml")
  -h           显示帮助信息

示例:
  go-tunnel-server -c /etc/tunnel/server.yaml

配置文件格式 (YAML):
  server:
    control_addr: "0.0.0.0:7000"    # 控制端口
    token: "your-secret-token"      # 认证令牌
    heartbeat_interval: 30          # 心跳间隔(秒)
    heartbeat_timeout: 90           # 心跳超时(秒)
  
  log:
    level: "info"                   # 日志级别
    file: ""                        # 日志文件(空则输出到控制台)`)
}
