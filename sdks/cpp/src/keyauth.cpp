// KeyAuth SaaS C++ 客户端 SDK 实现
// 依赖：libcurl + OpenSSL 1.1+ + nlohmann/json
#include "keyauth.hpp"

#include <curl/curl.h>
#include <openssl/hmac.h>
#include <openssl/evp.h>
#include <openssl/rand.h>

#include <nlohmann/json.hpp>

#include <sstream>
#include <iomanip>
#include <chrono>
#include <cstring>
#include <vector>

namespace keyauth {

// ============== 内部实现 ==============

class KeyAuthClientImpl {
public:
    std::string api_base;
    std::string app_key;
    std::string sign_secret;
    long timeout_sec;

    KeyAuthClientImpl(const std::string& api_base,
                      const std::string& app_key,
                      const std::string& sign_secret,
                      long timeout_sec)
        : api_base(stripTrailingSlash(api_base)),
          app_key(app_key),
          sign_secret(sign_secret),
          timeout_sec(timeout_sec) {}

    // POST 请求，返回 data 字段的 JSON 字符串
    std::string Post(const std::string& path, const std::string& body) {
        std::string timestamp = std::to_string(nowUnix());
        std::string nonce = randomNonce();

        // 签名原文：METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
        std::string sign_string = "POST\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + body;
        std::string signature = KeyAuthClient::Sign(sign_secret, sign_string);

        std::string url = api_base + path;
        std::string response;
        long http_code = 0;

        CURL* curl = curl_easy_init();
        if (!curl) {
            throw KeyAuthException(1006, "curl_easy_init 失败", 0);
        }

        struct curl_slist* headers = nullptr;
        headers = curl_slist_append(headers, "Content-Type: application/json");
        headers = curl_slist_append(headers, ("X-App-Key: " + app_key).c_str());
        headers = curl_slist_append(headers, ("X-Timestamp: " + timestamp).c_str());
        headers = curl_slist_append(headers, ("X-Nonce: " + nonce).c_str());
        headers = curl_slist_append(headers, ("X-Signature: " + signature).c_str());

        curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
        curl_easy_setopt(curl, CURLOPT_POST, 1L);
        curl_easy_setopt(curl, CURLOPT_POSTFIELDS, body.c_str());
        curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
        curl_easy_setopt(curl, CURLOPT_TIMEOUT, timeout_sec);
        curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, &writeCb);
        curl_easy_setopt(curl, CURLOPT_WRITEDATA, &response);

        CURLcode res = curl_easy_perform(curl);
        curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &http_code);
        curl_slist_free_all(headers);
        curl_easy_cleanup(curl);

        if (res != CURLE_OK) {
            throw KeyAuthException(1006, std::string("网络请求失败: ") + curl_easy_strerror(res), 0);
        }

        // 解析 envelope
        try {
            auto envelope = nlohmann::json::parse(response);
            int code = envelope.value("code", -1);
            std::string message = envelope.value("message", std::string("未知错误"));
            if (http_code != 200 || code != 0) {
                throw KeyAuthException(code, message, static_cast<int>(http_code));
            }
            if (!envelope.contains("data")) {
                return "{}";
            }
            return envelope["data"].dump();
        } catch (const nlohmann::json::parse_error& e) {
            std::string preview = response.substr(0, std::min<size_t>(200, response.size()));
            throw KeyAuthException(1006, "响应非 JSON: " + preview, static_cast<int>(http_code));
        }
    }

    // 计算 HMAC-SHA512/256 → 64 位小写 hex
    static std::string HmacSHA512_256Hex(const std::string& secret, const std::string& msg) {
        unsigned char result[EVP_MAX_MD_SIZE];
        unsigned int result_len = 0;

        // OpenSSL 1.1+ 提供 EVP_sha512_256()
        const EVP_MD* md = EVP_sha512_256();
        if (md == nullptr) {
            // 回退到 sha256（OpenSSL < 1.1，理论上不应触发，仅作兜底）
            md = EVP_sha256();
            static bool warned = false;
            if (!warned) {
                fprintf(stderr, "[keyauth-cpp] WARN: OpenSSL 不支持 SHA-512/256，回退 SHA-256（与后端可能不匹配）\n");
                warned = true;
            }
        }

        unsigned char* ok = HMAC(md,
                                 secret.data(), static_cast<int>(secret.size()),
                                 reinterpret_cast<const unsigned char*>(msg.data()),
                                 msg.size(),
                                 result, &result_len);
        if (ok == nullptr) {
            throw KeyAuthException(1006, "HMAC 计算失败", 0);
        }
        return toHex(result, result_len);
    }

private:
    static size_t writeCb(char* ptr, size_t size, size_t nmemb, void* userdata) {
        auto* s = static_cast<std::string*>(userdata);
        s->append(ptr, size * nmemb);
        return size * nmemb;
    }

    static std::string stripTrailingSlash(const std::string& s) {
        if (!s.empty() && s.back() == '/') return s.substr(0, s.size() - 1);
        return s;
    }

    static int64_t nowUnix() {
        return std::chrono::duration_cast<std::chrono::seconds>(
                   std::chrono::system_clock::now().time_since_epoch())
            .count();
    }

    static std::string randomNonce() {
        unsigned char b[16];
        if (RAND_bytes(b, sizeof(b)) != 1) {
            // 退化路径：用时间戳兜底（仅在熵源不可用时触发）
            return std::to_string(nowUnix()) + "fallback";
        }
        return toHex(b, sizeof(b));
    }

    static std::string toHex(const unsigned char* data, unsigned int len) {
        std::ostringstream oss;
        oss << std::hex << std::setfill('0');
        for (unsigned int i = 0; i < len; ++i) {
            oss << std::setw(2) << static_cast<int>(data[i]);
        }
        return oss.str();
    }
};

// ============== 公共 API 实现 ==============

KeyAuthClient::KeyAuthClient(const std::string& api_base,
                             const std::string& app_key,
                             const std::string& sign_secret,
                             long timeout_sec)
    : impl_(std::make_unique<KeyAuthClientImpl>(api_base, app_key, sign_secret, timeout_sec)) {}

KeyAuthClient::~KeyAuthClient() = default;

static std::string BuildPayload(const std::map<std::string, std::string>& fields) {
    nlohmann::json j;
    for (const auto& [k, v] : fields) {
        j[k] = v;
    }
    return j.dump();
}

std::string KeyAuthClient::Login(const std::string& card_key, const std::string& hwid,
                                 const std::string& device_name, const std::string& device_type) {
    return impl_->Post("/api/v1/client/login", BuildPayload({
        {"card_key", card_key},
        {"hwid", hwid},
        {"device_name", device_name},
        {"device_type", device_type},
    }));
}

std::string KeyAuthClient::Verify(const std::string& card_key, const std::string& hwid) {
    return impl_->Post("/api/v1/client/verify", BuildPayload({
        {"card_key", card_key}, {"hwid", hwid},
    }));
}

std::string KeyAuthClient::Heartbeat(const std::string& card_key, const std::string& hwid) {
    return impl_->Post("/api/v1/client/heartbeat", BuildPayload({
        {"card_key", card_key}, {"hwid", hwid},
    }));
}

std::string KeyAuthClient::Bind(const std::string& card_key, const std::string& hwid,
                                const std::string& device_name, const std::string& device_type) {
    return impl_->Post("/api/v1/client/bind", BuildPayload({
        {"card_key", card_key},
        {"hwid", hwid},
        {"device_name", device_name},
        {"device_type", device_type},
    }));
}

std::string KeyAuthClient::Unbind(const std::string& card_key, const std::string& hwid) {
    return impl_->Post("/api/v1/client/unbind", BuildPayload({
        {"card_key", card_key}, {"hwid", hwid},
    }));
}

std::string KeyAuthClient::GetVar(const std::string& card_key, const std::string& var_key) {
    return impl_->Post("/api/v1/client/get_var", BuildPayload({
        {"card_key", card_key}, {"var_key", var_key},
    }));
}

std::string KeyAuthClient::Notice() {
    return impl_->Post("/api/v1/client/notice", "{}");
}

std::string KeyAuthClient::Version(const std::string& current_version, const std::string& platform) {
    return impl_->Post("/api/v1/client/version", BuildPayload({
        {"current_version", current_version},
        {"platform", platform},
    }));
}

std::string KeyAuthClient::Logout(const std::string& card_key, const std::string& hwid) {
    return impl_->Post("/api/v1/client/logout", BuildPayload({
        {"card_key", card_key}, {"hwid", hwid},
    }));
}

std::string KeyAuthClient::Sign(const std::string& secret, const std::string& msg) {
    return KeyAuthClientImpl::HmacSHA512_256Hex(secret, msg);
}

}  // namespace keyauth
