package proto

import (
	"bytes"
	"encoding/json"
	"testing"
)

func BenchmarkProtocolEncoding(b *testing.B) {
	authReq := &AuthRequest{
		ClientID: "test-client-123",
		Token:    "my-secret-token-12345",
		Version:  "1.0.0",
	}

	b.Run("JSON (old)", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 直接使用JSON编码，避免混合协议的干扰
			data, err := json.Marshal(authReq)
			if err != nil {
				b.Fatal(err)
			}
			_ = data
		}
	})

	b.Run("Binary (new)", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data, err := EncodeBinary(authReq)
			if err != nil {
				b.Fatal(err)
			}
			_ = data
		}
	})
}

func BenchmarkProtocolDecoding(b *testing.B) {
	authReq := &AuthRequest{
		ClientID: "test-client-123",
		Token:    "my-secret-token-12345",
		Version:  "1.0.0",
	}

	// 生成JSON数据
	jsonData, err := json.Marshal(authReq)
	if err != nil {
		b.Fatal(err)
	}

	// 生成二进制数据
	binaryData, err := EncodeBinary(authReq)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("JSON (old)", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 直接使用JSON解码，避免混合协议的干扰
			var req AuthRequest
			err := json.Unmarshal(jsonData, &req)
			if err != nil {
				b.Fatal(err)
			}
			_ = req
		}
	})

	b.Run("Binary (new)", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var req AuthRequest
			err := DecodeBinary(binaryData, &req)
			if err != nil {
				b.Fatal(err)
			}
			_ = req
		}
	})
}

func BenchmarkMessageSize(b *testing.B) {
	authReq := &AuthRequest{
		ClientID: "test-client-123",
		Token:    "my-secret-token-12345",
		Version:  "1.0.0",
	}

	b.Run("JSON message size", func(b *testing.B) {
		data, err := json.Marshal(authReq)
		if err != nil {
			b.Fatal(err)
		}
		b.Logf("JSON message size: %d bytes", len(data))
	})

	b.Run("Binary message size", func(b *testing.B) {
		data, err := EncodeBinary(authReq)
		if err != nil {
			b.Fatal(err)
		}
		b.Logf("Binary message size: %d bytes", len(data))
	})
}

func BenchmarkThroughput(b *testing.B) {
	authReq := &AuthRequest{
		ClientID: "test-client-123",
		Token:    "my-secret-token-12345",
		Version:  "1.0.0",
	}

	var buf bytes.Buffer

	b.Run("JSON throughput", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, err := json.Marshal(authReq)
			if err != nil {
				b.Fatal(err)
			}
			_, err = buf.Write(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Binary throughput", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			data, err := EncodeBinary(authReq)
			if err != nil {
				b.Fatal(err)
			}
			_, err = buf.Write(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
