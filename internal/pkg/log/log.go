package log

import (
	"context"
	"log/slog"
	"os"
)

/*
1. slog.Logger - 日志记录器
2. slog.Handler - 决定日志如何输出（Text/JSON）
3. slog.Level - 日志级别（Debug/Info/Warn/Error）
4. slog.Attr - 结构化的键值对
*/

// 日志级别别名
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// logger 全局日志实例
var logger *slog.Logger

// 全局初始化
func init() {
	// TextHandler 输出文本的格式
	// JSONHandler 输出 JSON 格式（生产环境）
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: LevelDebug, // 默认显示所有级别
		// AddSource: true,       // 添加源代码位置
	})
	logger = slog.New(handler)
}

// SetLevel 设置日志级别
func SetLevel(level slog.Level) {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger = slog.New(handler)
}

// SetJSONOutput 切换为 JSON 输出格式
func SetJSONOutput(level slog.Level) {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger = slog.New(handler)
}

// GetLogger 获取底层 slog.Logger
func GetLogger() *slog.Logger {
	return logger
}

// Debug 调试级别日志
func Debug(msg string, args ...any) {
	logger.Debug(msg, args...)
}

// Info 信息级别日志
func Info(msg string, args ...any) {
	logger.Info(msg, args...)
}

// Warn 警告级别日志
func Warn(msg string, args ...any) {
	logger.Warn(msg, args...)
}

// Error 错误级别日志
func Error(msg string, args ...any) {
	logger.Error(msg, args...)
}

// DebugContext 带 context 的调试日志
func DebugContext(ctx context.Context, msg string, args ...any) {
	logger.DebugContext(ctx, msg, args...)
}

// InfoContext 带 context 的信息日志
func InfoContext(ctx context.Context, msg string, args ...any) {
	logger.InfoContext(ctx, msg, args...)
}

// WarnContext 带 context 的警告日志
func WarnContext(ctx context.Context, msg string, args ...any) {
	logger.WarnContext(ctx, msg, args...)
}

// ErrorContext 带 context 的错误日志
func ErrorContext(ctx context.Context, msg string, args ...any) {
	logger.ErrorContext(ctx, msg, args...)
}

// With 创建带有固定属性的子 Logger
func With(args ...any) *slog.Logger {
	return logger.With(args...)
}

// WithGroup 创建带有分组的子 Logger
func WithGroup(name string) *slog.Logger {
	return logger.WithGroup(name)
}
