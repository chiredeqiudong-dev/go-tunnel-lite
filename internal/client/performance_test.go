package client

import (
	"sync"
	"testing"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/config"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proto"
)

// BenchmarkTunnelCacheLookup 隧道配置缓存查找性能测试
func BenchmarkTunnelCacheLookup(b *testing.B) {
	cfg := &config.ClientConfig{
		Client: config.ClientSettings{
			Tunnels: make([]config.TunnelConfig, 10),
		},
	}

	for i := 0; i < 10; i++ {
		cfg.Client.Tunnels[i] = config.TunnelConfig{
			Name:       "tunnel-" + string(rune('a'+i)),
			LocalAddr:  "127.0.0.1:8080",
			RemotePort: 8080 + i,
		}
	}

	b.Run("LinearLookup", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, tunnel := range cfg.Client.Tunnels {
				if tunnel.Name == "tunnel-e" {
					break
				}
			}
		}
	})

	b.Run("CacheLookup", func(b *testing.B) {
		cache := make(map[string]*config.TunnelConfig)
		for i := range cfg.Client.Tunnels {
			cache[cfg.Client.Tunnels[i].Name] = &cfg.Client.Tunnels[i]
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = cache["tunnel-e"]
		}
	})
}

// BenchmarkMessageProcessing 消息处理性能测试
func BenchmarkMessageProcessing(b *testing.B) {
	msg := &proto.Message{
		Type: proto.TypePing,
		Data: nil,
	}

	b.Run("SingleMessage", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = msg
		}
	})

	b.Run("BatchMessages", func(b *testing.B) {
		batch := make([]*proto.Message, 10)
		for i := 0; i < 10; i++ {
			batch[i] = &proto.Message{
				Type: proto.TypePing,
				Data: nil,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, m := range batch {
				_ = m
			}
		}
	})
}

// BenchmarkConcurrentAccess 并发访问性能测试
func BenchmarkConcurrentAccess(b *testing.B) {
	cache := make(map[string]*config.TunnelConfig)
	var mu sync.RWMutex

	for i := 0; i < 100; i++ {
		cfg := &config.TunnelConfig{
			Name:       "tunnel-" + string(rune('a'+i%26)),
			LocalAddr:  "127.0.0.1:8080",
			RemotePort: 8080 + i,
		}
		cache[cfg.Name] = cfg
	}

	b.Run("ConcurrentRead", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.RLock()
				_ = cache["tunnel-a"]
				mu.RUnlock()
			}
		})
	})

	b.Run("ConcurrentWrite", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				mu.Lock()
				cache["tunnel-write"] = &config.TunnelConfig{Name: "tunnel-write"}
				mu.Unlock()
				i++
			}
		})
	})
}

// BenchmarkMessageQueue 消息队列性能测试
func BenchmarkMessageQueue(b *testing.B) {
	queue := NewMessageQueue(10)
	msg := &proto.Message{Type: proto.TypePing}

	b.Run("Push", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			queue.Push(msg)
		}
	})

	b.Run("PushPop", func(b *testing.B) {
		go func() {
			for i := 0; i < b.N; i++ {
				queue.PopBatch()
			}
		}()

		for i := 0; i < b.N; i++ {
			queue.Push(msg)
		}
	})
}
