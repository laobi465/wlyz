package com.keyauth.sdk.example;

import com.keyauth.sdk.KeyAuthClient;
import com.keyauth.sdk.KeyAuthException;

import java.util.Map;

/**
 * KeyAuth Java SDK 使用示例
 *
 * 演示 9 个客户端 API 的完整调用流程
 */
public class Example {

    public static void main(String[] args) {
        // 铁律 04：所有配置由调用方传入，SDK 内不硬编码
        KeyAuthClient client = new KeyAuthClient(
                "https://your-domain.com",
                "ak_your_app_key",
                "sk_your_sign_secret"
        );

        try {
            // 1. 登录
            Map<String, Object> login = client.login("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash", "我的电脑", "pc");
            System.out.println("登录成功，token: " + login.get("token"));

            // 2. 验证
            client.verify("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash");

            // 3. 心跳
            Map<String, Object> hb = client.heartbeat("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash");
            System.out.println("心跳成功，next_heartbeat: " + hb.get("next_heartbeat"));

            // 4. 云变量
            Map<String, Object> v = client.getVar("ABCD-1234-EFGH-5678", "pro_feature");
            System.out.println("云变量: " + v.get("var_value"));

            // 5. 公告
            Map<String, Object> n = client.notice();
            System.out.println("公告: " + n.get("notices"));

            // 6. 版本检查
            Map<String, Object> ver = client.version("1.0.0", "windows");
            System.out.println("has_update: " + ver.get("has_update"));

            // 7. 退出
            client.logout("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash");
        } catch (KeyAuthException e) {
            System.err.printf("KeyAuth 错误: code=%d msg=%s%n", e.getCode(), e.getMessage());
        }
    }
}
