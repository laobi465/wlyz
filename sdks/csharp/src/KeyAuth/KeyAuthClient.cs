using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;

namespace KeyAuth.Sdk;

/// <summary>
/// KeyAuth SaaS C# 客户端 SDK
///
/// 面向终端软件的客户端 SDK，封装 9 个验证 API：
/// Login / Verify / Heartbeat / Bind / Unbind / GetVar / Notice / Version / Logout
///
/// 依赖：.NET 6+ + System.Text.Json（无第三方包）
///
/// 签名算法（与后端 internal/middleware/signature.go 一致）：
///     原文 = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
///     签名 = HMAC-SHA512/256(secret, 原文) → 64 位小写 hex
///
/// 注：.NET 原生不直接提供 HMACSHA512/256；优先尝试 BouncyCastle 提供者；
///     若不可用则回退 HMACSHA256（与 Python/PHP SDK 回退策略一致）。
///     生产环境建议安装 BouncyCastle.Cryptography NuGet 包以启用 SHA-512/256。
///
/// 铁律 04：API 地址 / AppKey / SignSecret 由调用方传入，SDK 内不硬编码
/// 铁律 06：所有接口错误抛出 KeyAuthException(code, message)，不静默吞异常
/// </summary>
public class KeyAuthClient
{
    private const string Version = "0.4.0";

    private readonly string _apiBase;
    private readonly string _appKey;
    private readonly string _signSecret;
    private readonly HttpClient _httpClient;
    private static readonly SHA512_256Provider _shaProvider = new();

    public KeyAuthClient(string apiBase, string appKey, string signSecret)
    {
        _apiBase = apiBase.EndsWith("/") ? apiBase[..^1] : apiBase;
        _appKey = appKey;
        _signSecret = signSecret;
        _httpClient = new HttpClient { Timeout = TimeSpan.FromSeconds(10) };
    }

    // ==================== 公共 API ====================

    /// <summary>Login 登录（首次自动绑定设备）</summary>
    public Task<JsonElement> LoginAsync(string cardKey, string hwid, string deviceName = "", string deviceType = "")
        => PostAsync("/api/v1/client/login", new
        {
            card_key = cardKey,
            hwid = hwid,
            device_name = deviceName,
            device_type = deviceType
        });

    /// <summary>Verify 验证卡密有效性</summary>
    public Task<JsonElement> VerifyAsync(string cardKey, string hwid)
        => PostAsync("/api/v1/client/verify", new { card_key = cardKey, hwid = hwid });

    /// <summary>Heartbeat 心跳保活</summary>
    public Task<JsonElement> HeartbeatAsync(string cardKey, string hwid)
        => PostAsync("/api/v1/client/heartbeat", new { card_key = cardKey, hwid = hwid });

    /// <summary>Bind 手动绑定设备</summary>
    public Task<JsonElement> BindAsync(string cardKey, string hwid, string deviceName = "", string deviceType = "")
        => PostAsync("/api/v1/client/bind", new
        {
            card_key = cardKey,
            hwid = hwid,
            device_name = deviceName,
            device_type = deviceType
        });

    /// <summary>Unbind 解绑设备</summary>
    public Task<JsonElement> UnbindAsync(string cardKey, string hwid)
        => PostAsync("/api/v1/client/unbind", new { card_key = cardKey, hwid = hwid });

    /// <summary>GetVar 获取云变量</summary>
    public Task<JsonElement> GetVarAsync(string cardKey, string varKey)
        => PostAsync("/api/v1/client/get_var", new { card_key = cardKey, var_key = varKey });

    /// <summary>Notice 获取应用公告</summary>
    public Task<JsonElement> NoticeAsync()
        => PostAsync("/api/v1/client/notice", new { });

    /// <summary>Version 检查版本更新</summary>
    public Task<JsonElement> VersionAsync(string currentVersion = "", string platform = "")
        => PostAsync("/api/v1/client/version", new { current_version = currentVersion, platform = platform });

    /// <summary>Logout 退出登录</summary>
    public Task<JsonElement> LogoutAsync(string cardKey, string hwid)
        => PostAsync("/api/v1/client/logout", new { card_key = cardKey, hwid = hwid });

    // ==================== 内部方法 ====================

    private async Task<JsonElement> PostAsync(string path, object payload)
    {
        string body = JsonSerializer.Serialize(payload);
        string timestamp = DateTimeOffset.UtcNow.ToUnixTimeSeconds().ToString();
        string nonce = RandomNonce();

        // 签名原文：METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
        string signString = $"POST\n{path}\n{timestamp}\n{nonce}\n{body}";
        string signature = _shaProvider.HmacHex(_signSecret, signString);

        using var req = new HttpRequestMessage(HttpMethod.Post, _apiBase + path)
        {
            Content = new StringContent(body, Encoding.UTF8, "application/json")
        };
        req.Headers.Add("X-App-Key", _appKey);
        req.Headers.Add("X-Timestamp", timestamp);
        req.Headers.Add("X-Nonce", nonce);
        req.Headers.Add("X-Signature", signature);

        HttpResponseMessage resp;
        try
        {
            resp = await _httpClient.SendAsync(req);
        }
        catch (Exception e)
        {
            throw new KeyAuthException(1006, "网络请求失败: " + e.Message, 0);
        }

        string raw = await resp.Content.ReadAsStringAsync();
        using var doc = JsonDocument.Parse(raw);
        var root = doc.RootElement;

        int code = root.TryGetProperty("code", out var c) && c.ValueKind == JsonValueKind.Number
            ? c.GetInt32()
            : -1;
        string message = root.TryGetProperty("message", out var m) && m.ValueKind == JsonValueKind.String
            ? m.GetString() ?? "未知错误"
            : "未知错误";

        if ((int)resp.StatusCode != 200 || code != 0)
        {
            throw new KeyAuthException(code, message, (int)resp.StatusCode);
        }

        if (root.TryGetProperty("data", out var data))
        {
            return data.Clone();
        }
        return default;
    }

    private static string RandomNonce()
    {
        Span<byte> b = stackalloc byte[16];
        RandomNumberGenerator.Fill(b);
        return Convert.ToHexString(b).ToLowerInvariant();
    }

    public static string GetVersion() => Version;
}

/// <summary>
/// SHA-512/256 HMAC 提供者
/// 优先使用 BouncyCastle（若运行时已加载）；否则回退 .NET 原生 HMACSHA256
/// </summary>
internal sealed class SHA512_256Provider
{
    private readonly bool _useBouncyCastle;

    public SHA512_256Provider()
    {
        // 反射探测 BouncyCastle.Cryptography 是否已加载
        _useBouncyCastle = Type.GetType("Org.BouncyCastle.Crypto.Digests.Sha512_256Digest, BouncyCastle.Cryptography") != null;
    }

    public string HmacHex(string secret, string msg)
    {
        byte[] key = Encoding.UTF8.GetBytes(secret);
        byte[] data = Encoding.UTF8.GetBytes(msg);
        byte[] raw = _useBouncyCastle ? HmacBouncyCastle(key, data) : HmacFallback(key, data);
        return Convert.ToHexString(raw).ToLowerInvariant();
    }

    private static byte[] HmacFallback(byte[] key, byte[] data)
    {
        using var h = new HMACSHA256(key);
        return h.ComputeHash(data);
    }

    private static byte[] HmacBouncyCastle(byte[] key, byte[] data)
    {
        // 通过反射调用 BouncyCastle HMac + Sha512_256Digest，避免硬依赖 NuGet 包
        var asm = Type.GetType("Org.BouncyCastle.Crypto.Digests.Sha512_256Digest, BouncyCastle.Cryptography")!;
        var digest = Activator.CreateInstance(asm)!;
        var macType = Type.GetType("Org.BouncyCastle.Crypto.Macs.HMac, BouncyCastle.Cryptography")!;
        var mac = Activator.CreateInstance(macType, digest)!;
        var keyParamType = Type.GetType("Org.BouncyCastle.Crypto.Parameters.KeyParameter, BouncyCastle.Cryptography")!;
        var keyParam = Activator.CreateInstance(keyParamType, key)!;
        macType.GetMethod("Init")!.Invoke(mac, new[] { keyParam });
        macType.GetMethod("BlockUpdate", new[] { typeof(byte[]), typeof(int), typeof(int) })!
            .Invoke(mac, new object[] { data, 0, data.Length });
        var result = new byte[32];
        macType.GetMethod("DoFinal", new[] { typeof(byte[]), typeof(int) })!
            .Invoke(mac, new object[] { result, 0 });
        return result;
    }
}

/// <summary>KeyAuth API 错误</summary>
public class KeyAuthException : Exception
{
    public int Code { get; }
    public new string Message { get; }
    public int HttpStatus { get; }

    public KeyAuthException(int code, string message, int httpStatus) : base($"[{code}] {message}")
    {
        Code = code;
        Message = message;
        HttpStatus = httpStatus;
    }
}
