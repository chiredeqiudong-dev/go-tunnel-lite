# Go-Tunnel-Lite Makefile

# 变量定义
BINARY_SERVER = go-tunnel-server
BINARY_CLIENT = go-tunnel-client
BUILD_DIR = bin
GO = go

# 版本信息
VERSION ?= 1.0.0
BUILD_TIME = $(shell date +%Y-%m-%d\ %H:%M:%S)
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 编译参数
LDFLAGS = -ldflags "-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)' -X 'main.GitCommit=$(GIT_COMMIT)'"

.PHONY: all build server client test clean run-server run-client help

# 默认目标
all: build

# 编译所有
build: server client
	@echo "编译完成!"
	@echo "可执行文件位于 $(BUILD_DIR)/ 目录"

# 编译服务端
server:
	@echo "编译服务端..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_SERVER) ./cmd/server

# 编译客户端
client:
	@echo "编译客户端..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CLIENT) ./cmd/client

# 运行测试
test:
	@echo "运行测试..."
	$(GO) test -v ./...

# 运行服务端（开发模式）
run-server:
	$(GO) run ./cmd/server -c configs/server.yaml

# 运行客户端（开发模式）
run-client:
	$(GO) run ./cmd/client -c configs/client.yaml

# 清理编译产物
clean:
	@echo "清理..."
	@rm -rf $(BUILD_DIR)
	@echo "清理完成!"

# 帮助信息
help:
	@echo "Go-Tunnel-Lite 编译脚本"
	@echo ""
	@echo "使用方法:"
	@echo "  make [目标]"
	@echo ""
	@echo "目标:"
	@echo "  all         编译所有（默认）"
	@echo "  build       编译服务端和客户端"
	@echo "  server      仅编译服务端"
	@echo "  client      仅编译客户端"
	@echo "  test        运行测试"
	@echo "  run-server  运行服务端（开发模式）"
	@echo "  run-client  运行客户端（开发模式）"
	@echo "  clean       清理编译产物"
	@echo "  help        显示此帮助信息"
