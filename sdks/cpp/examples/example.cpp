// KeyAuth C++ SDK 使用示例
// 编译：g++ -std=c++17 -O2 example.cpp ../src/keyauth.cpp -o example -lcurl -lcrypto -lssl
#include "keyauth/keyauth.hpp"

#include <nlohmann/json.hpp>

#include <iostream>

int main() {
    // 铁律 04：所有配置由调用方传入
    keyauth::KeyAuthClient client(
        "https://your-domain.com",
        "ak_your_app_key",
        "sk_your_sign_secret"
    );

    try {
        // 1. 登录
        std::string loginJson = client.Login("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash", "我的电脑", "pc");
        auto login = nlohmann::json::parse(loginJson);
        std::cout << "登录成功，token: " << login["token"].get<std::string>() << std::endl;

        // 2. 验证
        client.Verify("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash");

        // 3. 心跳
        std::string hbJson = client.Heartbeat("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash");
        auto hb = nlohmann::json::parse(hbJson);
        std::cout << "心跳成功，next_heartbeat: " << hb["next_heartbeat"] << std::endl;

        // 4. 云变量
        std::string vJson = client.GetVar("ABCD-1234-EFGH-5678", "pro_feature");
        auto v = nlohmann::json::parse(vJson);
        std::cout << "云变量: " << v["var_value"].get<std::string>() << std::endl;

        // 5. 公告
        std::string nJson = client.Notice();
        std::cout << "公告 JSON: " << nJson << std::endl;

        // 6. 版本检查
        std::string verJson = client.Version("1.0.0", "windows");
        auto ver = nlohmann::json::parse(verJson);
        std::cout << "has_update: " << ver["has_update"] << std::endl;

        // 7. 退出
        client.Logout("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash");
    } catch (const keyauth::KeyAuthException& e) {
        std::cerr << "KeyAuth 错误: code=" << e.code() << " msg=" << e.message() << std::endl;
        return 1;
    }
    return 0;
}
