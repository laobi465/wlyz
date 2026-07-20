// 客户端 SDK 签名对齐测试脚本（C++ 版）
// 用法：./sign <secret> <message>
// 输出：64 位小写 hex 签名（与后端 crypto.HMACSHA256 对齐）
//
// 编译：g++ -std=c++17 -O2 sign.cpp -o sign -lcrypto
//   注：仅依赖 OpenSSL（无需 libcurl / nlohmann/json）
//
// OpenSSL 1.1+ 支持 EVP_sha512_256()；< 1.1 自动回退 EVP_sha256()
// 回退时输出 stderr 警告，stdout 仍输出签名（与后端可能不匹配）

#include <openssl/hmac.h>
#include <openssl/evp.h>

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <iostream>
#include <sstream>
#include <iomanip>

static std::string toHex(const unsigned char* data, unsigned int len) {
    std::ostringstream oss;
    oss << std::hex << std::setfill('0');
    for (unsigned int i = 0; i < len; ++i) {
        oss << std::setw(2) << static_cast<int>(data[i]);
    }
    return oss.str();
}

static std::string sign(const std::string& secret, const std::string& msg) {
    const EVP_MD* md = EVP_sha512_256();
    if (md == nullptr) {
        std::fprintf(stderr, "[sign.cpp] WARN: OpenSSL 不支持 SHA-512/256，回退 SHA-256\n");
        md = EVP_sha256();
    }
    unsigned char result[EVP_MAX_MD_SIZE];
    unsigned int result_len = 0;
    HMAC(md,
         secret.data(), static_cast<int>(secret.size()),
         reinterpret_cast<const unsigned char*>(msg.data()), msg.size(),
         result, &result_len);
    return toHex(result, result_len);
}

int main(int argc, char* argv[]) {
    if (argc != 3) {
        std::fprintf(stderr, "usage: %s <secret> <message>\n", argv[0]);
        return 1;
    }
    std::cout << sign(argv[1], argv[2]) << std::endl;
    return 0;
}
