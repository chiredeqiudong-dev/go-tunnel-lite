package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestBasicLogging 测试基本日志功能
func TestBasicLogging(t *testing.T) {
	// 使用 bytes.Buffer 捕获日志输出
	var buf bytes.Buffer

	// 创建一个输出到 buffer 的 handler
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: LevelDebug,
	})
	// 临时替换全局 logger
	oldLogger := logger
	logger = slog.New(handler)
	defer func() { logger = oldLogger }() // 测试结束后恢复

	// 测试各级别日志
	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")

	output := buf.String()

	// 验证所有级别都输出了
	levels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	for _, level := range levels {
		if !strings.Contains(output, level) {
			t.Errorf("output should contain %s level", level)
		}
	}
}

// TestLevelFilter 测试日志级别过滤
func TestLevelFilter(t *testing.T) {
	var buf bytes.Buffer

	// 设置级别为 WARN，Debug 和 Info 应被过滤
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: LevelWarn,
	})
	oldLogger := logger
	logger = slog.New(handler)
	defer func() { logger = oldLogger }()

	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")

	output := buf.String()

	// Debug 和 Info 不应出现
	if strings.Contains(output, "DEBUG") {
		t.Error("DEBUG should be filtered when level is WARN")
	}
	if strings.Contains(output, "INFO") {
		t.Error("INFO should be filtered when level is WARN")
	}

	// Warn 和 Error 应该出现
	if !strings.Contains(output, "WARN") {
		t.Error("WARN should be present")
	}
	if !strings.Contains(output, "ERROR") {
		t.Error("ERROR should be present")
	}
}

// TestStructuredLogging 测试结构化日志
func TestStructuredLogging(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: LevelDebug,
	})
	oldLogger := logger
	logger = slog.New(handler)
	defer func() { logger = oldLogger }()

	// slog 的结构化日志使用 key-value 对
	Info("user login", "user_id", 12345, "ip", "192.168.1.1")

	output := buf.String()

	// 验证结构化字段存在
	expectedFields := []string{"user_id", "12345", "ip", "192.168.1.1"}
	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("output should contain %q, got:\n%s", field, output)
		}
	}
}

// TestWithLogger 测试 With 创建子 Logger
func TestWithLogger(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: LevelDebug,
	})
	oldLogger := logger
	logger = slog.New(handler)
	defer func() { logger = oldLogger }()

	// With 创建带固定字段的子 Logger
	// 这在处理一个连接/请求时非常有用
	clientLogger := With("client_id", "abc123")
	clientLogger.Info("connected")
	clientLogger.Info("data received", "bytes", 1024)

	output := buf.String()

	// client_id 应该出现两次（两条日志都有）
	count := strings.Count(output, "client_id")
	if count != 2 {
		t.Errorf("client_id should appear 2 times, got %d times\n%s", count, output)
	}
}

// TestJSONOutput 测试 JSON 格式输出
func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer

	// JSONHandler 输出 JSON 格式
	// 生产环境推荐，便于 ELK/Splunk 等工具分析
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: LevelDebug,
	})
	oldLogger := logger
	logger = slog.New(handler)
	defer func() { logger = oldLogger }()

	Info("test message", "key", "value")

	output := buf.String()

	// JSON 格式应该包含这些
	if !strings.Contains(output, `"msg"`) {
		t.Error("JSON output should contain \"msg\" field")
	}
	if !strings.Contains(output, `"level"`) {
		t.Error("JSON output should contain \"level\" field")
	}
}

// ============================================================
// 基准测试
// 运行：go test -bench=. ./internal/pkg/log
// ============================================================

func BenchmarkSlogInfo(b *testing.B) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: LevelInfo,
	})
	testLogger := slog.New(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testLogger.Info("benchmark message", "iteration", i)
	}
}

func BenchmarkSlogJSON(b *testing.B) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: LevelInfo,
	})
	testLogger := slog.New(handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testLogger.Info("benchmark message", "iteration", i)
	}
}
