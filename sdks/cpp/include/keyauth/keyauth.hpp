// KeyAuth SaaS C++ 客户端 SDK 头文件
// 面向终端软件的客户端 SDK，封装 9 个验证 API：
//   login / verify / heartbeat / bind / unbind / get_var / notice / version / logout
//
// 依赖：C++17 + libcurl + OpenSSL 1.1+（推荐 3.0+）+ nlohmann/json
//
// 签名算法（与后端 internal/middleware/signature.go 一致）：
//   原文 = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
//   签名 = HMAC-SHA512/256(secret, 原文) → 64 位小写 hex
//   注：OpenSSL 1.1+ 提供 EVP_sha512_256()，与后端字节级对齐；
//       若链接的 OpenSSL < 1.1（无 SHA-512/256），回退 EVP_sha256() 并 stderr 警告
//
// 铁律 04：API 地址 / AppKey / SignSecret 由调用方传入，SDK 内不硬编码
// 铁律 06：所有接口错误抛出 KeyAuthException(code, message)，不静默吞异常
#pragma once

#include <string>
#include <map>
#include <stdexcept>
#include <memory>
#include <cstdint>

namespace keyauth {

// SDK 版本号
inline constexpr const char* kVersion = "0.4.0";

// KeyAuth API 错误异常
class KeyAuthException : public std::runtime_error {
public:
    KeyAuthException(int code, const std::string& message, int http_status)
        : std::runtime_error("[" + std::to_string(code) + "] " + message),
          code_(code), message_(message), http_status_(http_status) {}

    int code() const noexcept { return code_; }
    const std::string& message() const noexcept { return message_; }
    int http_status() const noexcept { return http_status_; }

private:
    int code_;
    std::string message_;
    int http_status_;
};

// JSON 对象类型（依赖 nlohmann/json，业务侧可用 nlohmann::json 直接访问）
// 前向声明避免头文件强依赖
namespace nlohmann {  // forward
template <typename> class basic_json;
using json = basic_json<void>;
}  // namespace nlohmann

class KeyAuthClientImpl;

// KeyAuth SaaS 客户端 SDK
class KeyAuthClient {
public:
    /// 构造客户端
    /// @param api_base 后端 API 根地址（如 https://yourdomain.com）
    /// @param app_key 应用 AppKey（ak_ 开头）
    /// @param sign_secret 应用 SignSecret（sk_ 开头，AES 解密后的明文）
    /// @param timeout_sec HTTP 请求超时秒数
    KeyAuthClient(const std::string& api_base,
                  const std::string& app_key,
                  const std::string& sign_secret,
                  long timeout_sec = 10);

    ~KeyAuthClient();

    KeyAuthClient(const KeyAuthClient&) = delete;
    KeyAuthClient& operator=(const KeyAuthClient&) = delete;

    // ==================== 公共 API ====================

    // Login 登录（首次自动绑定设备）
    std::string Login(const std::string& card_key,
                      const std::string& hwid,
                      const std::string& device_name = "",
                      const std::string& device_type = "");

    // Verify 验证卡密有效性
    std::string Verify(const std::string& card_key, const std::string& hwid);

    // Heartbeat 心跳保活
    std::string Heartbeat(const std::string& card_key, const std::string& hwid);

    // Bind 手动绑定设备
    std::string Bind(const std::string& card_key,
                     const std::string& hwid,
                     const std::string& device_name = "",
                     const std::string& device_type = "");

    // Unbind 解绑设备
    std::string Unbind(const std::string& card_key, const std::string& hwid);

    // GetVar 获取云变量
    std::string GetVar(const std::string& card_key, const std::string& var_key);

    // Notice 获取应用公告
    std::string Notice();

    // Version 检查版本更新
    std::string Version(const std::string& current_version = "",
                        const std::string& platform = "");

    // Logout 退出登录
    std::string Logout(const std::string& card_key, const std::string& hwid);

    // 计算签名（静态方法，导出供测试用）
    static std::string Sign(const std::string& secret, const std::string& msg);

private:
    std::unique_ptr<KeyAuthClientImpl> impl_;
};

}  // namespace keyauth
