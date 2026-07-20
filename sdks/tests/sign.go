// Package main 客户端 SDK 签名对齐测试脚本（Go 版）
// 用法：go run sign.go <secret> <message>
// 输出：64 位小写 hex 签名（与后端 crypto.HMACSHA256 对齐）
//
// Go 标准库 crypto/sha512 原生提供 New512_256，无环境限制
package main

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"os"
)

func sign(secret, msg string) string {
	mac := hmac.New(sha512.New512_256, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run sign.go <secret> <message>")
		os.Exit(1)
	}
	fmt.Println(sign(os.Args[1], os.Args[2]))
}
