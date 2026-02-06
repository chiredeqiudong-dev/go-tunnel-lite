package main

import (
	"encoding/hex"
	"fmt"
	"github.com/chiredeqiudong-dev/go-tunnel-lite/internal/pkg/proto"
)

func main() {
	authReq := &proto.AuthRequest{
		ClientID: "test-client-123",
		Token:    "my-secret-token-12345",
		Version:  "1.0.0",
	}

	// 使用官方编码
	binaryData, err := proto.EncodeBinary(authReq)
	if err != nil {
		fmt.Printf("编码失败: %v\n", err)
		return
	}

	fmt.Printf("官方编码数据长度: %d\n", len(binaryData))
	fmt.Printf("官方编码数据(hex): %s\n", hex.EncodeToString(binaryData))

	// 尝试解码
	var decodedReq proto.AuthRequest
	err = proto.DecodeBinary(binaryData, &decodedReq)
	if err != nil {
		fmt.Printf("解码失败: %v\n", err)
		return
	}

	fmt.Printf("解码成功: %+v\n", decodedReq)
	fmt.Printf("ClientID: %s\n", decodedReq.ClientID)
	fmt.Printf("Token: %s\n", decodedReq.Token)
	fmt.Printf("Version: %s\n", decodedReq.Version)
}
