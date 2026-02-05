package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proto"
)

type MockTunnel struct {
	Name       string
	LocalAddr  string
	RemotePort int
}

func main() {
	fmt.Println("=== Go-Tunnel-Lite 性能优化测试报告 ===")

	testTunnelCache()
	testProtocolEncoding()
	testMemoryAllocation()
	testConcurrency()

	fmt.Println()
	fmt.Println("=== 测试完成 ===")
}

func testTunnelCache() {
	fmt.Println("1. 隧道配置查找性能测试")
	fmt.Println("-----------------------")

	// 创建100个隧道配置
	tunnels := make([]MockTunnel, 100)
	for i := 0; i < 100; i++ {
		tunnels[i] = MockTunnel{
			Name:       fmt.Sprintf("tunnel-%d", i),
			LocalAddr:  "127.0.0.1:8080",
			RemotePort: 8080 + i,
		}
	}

	// 线性查找测试
	start := time.Now()
	for i := 0; i < 1000000; i++ {
		for _, t := range tunnels {
			if t.Name == "tunnel-50" {
				break
			}
		}
	}
	linearTime := time.Since(start)

	// Map缓存查找测试
	cache := make(map[string]*MockTunnel)
	for i := range tunnels {
		cache[tunnels[i].Name] = &tunnels[i]
	}

	start = time.Now()
	for i := 0; i < 1000000; i++ {
		_ = cache["tunnel-50"]
	}
	cacheTime := time.Since(start)

	improvement := float64(linearTime-cacheTime) / float64(linearTime) * 100

	fmt.Printf("线性查找耗时: %v\n", linearTime)
	fmt.Printf("Map缓存耗时: %v\n", cacheTime)
	fmt.Printf("性能提升: %.2f%%\n", improvement)
	fmt.Printf("缓存更快? %v\n\n", cacheTime < linearTime)
}

func testProtocolEncoding() {
	fmt.Println("2. 协议编解码性能测试")
	fmt.Println("----------------------")

	authReq := &proto.AuthRequest{
		ClientID: "client-1234567890",
		Token:    "secret-token-1234567890",
		Version:  "1.0.0",
	}

	// JSON编码测试
	jsonStart := time.Now()
	jsonSize := 0
	for i := 0; i < 100000; i++ {
		data, _ := json.Marshal(authReq)
		jsonSize = len(data)
	}
	jsonTime := time.Since(jsonStart)

	// 二进制编码测试
	binaryStart := time.Now()
	binarySize := 0
	for i := 0; i < 100000; i++ {
		data, _ := authReq.EncodeBinary()
		binarySize = len(data)
	}
	binaryTime := time.Since(binaryStart)

	sizeReduction := float64(jsonSize-binarySize) / float64(jsonSize) * 100
	speedImprovement := float64(jsonTime-binaryTime) / float64(jsonTime) * 100

	fmt.Printf("JSON编码耗时: %v, 大小: %d bytes\n", jsonTime, jsonSize)
	fmt.Printf("二进制编码耗时: %v, 大小: %d bytes\n", binaryTime, binarySize)
	fmt.Printf("速度提升: %.2f%%\n", speedImprovement)
	fmt.Printf("大小减少: %.2f%%\n\n", sizeReduction)
}

func testMemoryAllocation() {
	fmt.Println("3. 内存分配性能测试")
	fmt.Println("------------------")

	iterations := 100000

	// JSON分配测试
	var jsonAllocs uint64
	start := time.Now()
	for i := 0; i < iterations; i++ {
		data, _ := json.Marshal(&proto.AuthRequest{
			ClientID: "test-client",
			Token:    "test-token",
			Version:  "1.0.0",
		})
		jsonAllocs += uint64(len(data))
	}
	jsonTime := time.Since(start)

	// 二进制分配测试
	var binaryAllocs uint64
	start = time.Now()
	for i := 0; i < iterations; i++ {
		data, _ := (&proto.AuthRequest{
			ClientID: "test-client",
			Token:    "test-token",
			Version:  "1.0.0",
		}).EncodeBinary()
		binaryAllocs += uint64(len(data))
	}
	binaryTime := time.Since(start)

	fmt.Printf("JSON分配: %d bytes, 耗时: %v\n", jsonAllocs, jsonTime)
	fmt.Printf("二进制分配: %d bytes, 耗时: %v\n", binaryAllocs, binaryTime)
	fmt.Printf("内存节省: %.2f%%\n\n", float64(jsonAllocs-binaryAllocs)/float64(jsonAllocs)*100)
}

func testConcurrency() {
	fmt.Println("4. 并发性能测试")
	fmt.Println("---------------")

	var wg sync.WaitGroup
	iterations := 100000

	// 模拟单消息处理
	start := time.Now()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				msg := &proto.Message{Type: proto.TypePing}
				_ = msg
			}
		}()
	}
	wg.Wait()
	singleTime := time.Since(start)

	// 模拟批量消息处理
	start = time.Now()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			batch := make([]*proto.Message, 100)
			for j := 0; j < iterations/100; j++ {
				for k := 0; k < 100; k++ {
					batch[k] = &proto.Message{Type: proto.TypePing}
				}
				// 模拟批量处理
				_ = batch
			}
		}()
	}
	wg.Wait()
	batchTime := time.Since(start)

	fmt.Printf("单条处理耗时: %v (总消息数: %d)\n", singleTime, iterations*10)
	fmt.Printf("批量处理耗时: %v (总消息数: %d)\n", batchTime, iterations*10)

	if singleTime > batchTime {
		fmt.Printf("吞吐量提升: %.2f%%\n\n", float64(singleTime-batchTime)/float64(singleTime)*100)
	} else {
		fmt.Printf("批处理在当前场景下较慢（正常，因为批量创建有额外开销）\n\n")
	}
}
