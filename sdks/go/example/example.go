// Package main KeyAuth Go SDK 使用示例
//
// 演示 9 个客户端 API 的完整调用流程：
//
//	go run example.go
package main

import (
	"fmt"
	"log"

	keyauth "github.com/your-org/keyauth-saas/sdks/go/keyauth"
)

func main() {
	// 铁律 04：所有配置由调用方传入，SDK 内不硬编码
	client := keyauth.NewClient(
		"https://your-domain.com",
		"ak_your_app_key",
		"sk_your_sign_secret",
	)

	// 1. 登录
	login, err := client.Login("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash", "我的电脑", "pc")
	if err != nil {
		log.Fatalf("登录失败: %v", err)
	}
	fmt.Printf("登录成功，token: %s\n", login.Token)
	fmt.Printf("卡密信息: %+v\n", login.Card)

	// 2. 验证
	if _, err := client.Verify("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash"); err != nil {
		log.Printf("验证失败: %v", err)
	}

	// 3. 心跳
	hb, err := client.Heartbeat("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash")
	if err != nil {
		log.Printf("心跳失败: %v", err)
	} else {
		fmt.Printf("心跳成功，下次心跳时间戳: %d\n", hb.NextHeartbeat)
	}

	// 4. 获取云变量
	v, err := client.GetVar("ABCD-1234-EFGH-5678", "pro_feature")
	if err != nil {
		log.Printf("获取云变量失败: %v", err)
	} else {
		fmt.Printf("云变量 %s = %s\n", v.VarKey, v.VarValue)
	}

	// 5. 公告
	if n, err := client.Notice(); err == nil {
		for _, item := range n.Notices {
			fmt.Printf("[公告] %s: %s\n", item.Title, item.Content)
		}
	}

	// 6. 版本检查
	if ver, err := client.Version("1.0.0", "windows"); err == nil {
		if ver.HasUpdate {
			fmt.Printf("发现新版本 %s，下载地址: %s\n", ver.LatestVersion, ver.DownloadURL)
		}
	}

	// 7. 退出登录
	if _, err := client.Logout("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash"); err != nil {
		log.Printf("退出失败: %v", err)
	}
}
