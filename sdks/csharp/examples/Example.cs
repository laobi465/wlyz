using System;
using System.Threading.Tasks;
using KeyAuth.Sdk;

class Example
{
    static async Task Main(string[] args)
    {
        // 铁律 04：所有配置由调用方传入
        var client = new KeyAuthClient(
            "https://your-domain.com",
            "ak_your_app_key",
            "sk_your_sign_secret"
        );

        try
        {
            // 1. 登录
            var login = await client.LoginAsync("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash", "我的电脑", "pc");
            Console.WriteLine($"登录成功，token: {login.GetProperty("token").GetString()}");

            // 2. 验证
            await client.VerifyAsync("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash");

            // 3. 心跳
            var hb = await client.HeartbeatAsync("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash");
            Console.WriteLine($"心跳成功，next_heartbeat: {hb.GetProperty("next_heartbeat").GetInt64()}");

            // 4. 云变量
            var v = await client.GetVarAsync("ABCD-1234-EFGH-5678", "pro_feature");
            Console.WriteLine($"云变量: {v.GetProperty("var_value").GetString()}");

            // 5. 公告
            var n = await client.NoticeAsync();
            Console.WriteLine($"公告: {n.GetProperty("notices")}");

            // 6. 版本检查
            var ver = await client.VersionAsync("1.0.0", "windows");
            Console.WriteLine($"has_update: {ver.GetProperty("has_update").GetBoolean()}");

            // 7. 退出
            await client.LogoutAsync("ABCD-1234-EFGH-5678", "cpu-mac-disk-hash");
        }
        catch (KeyAuthException e)
        {
            Console.Error.WriteLine($"KeyAuth 错误: code={e.Code} msg={e.Message}");
        }
    }
}
