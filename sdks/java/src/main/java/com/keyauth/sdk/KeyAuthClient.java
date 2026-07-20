package com.keyauth.sdk;

import com.fasterxml.jackson.databind.ObjectMapper;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.time.Duration;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * KeyAuth SaaS Java 客户端 SDK
 *
 * <p>面向终端软件的客户端 SDK，封装 9 个验证 API：
 * login / verify / heartbeat / bind / unbind / getVar / notice / version / logout</p>
 *
 * <p>依赖：JDK 11+（java.net.http.HttpClient）+ Jackson Databind（JSON 处理）</p>
 *
 * <p>签名算法（与后端 internal/middleware/signature.go 一致）：
 * <pre>
 * 原文 = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
 * 签名 = HMAC-SHA512/256(secret, 原文) → 64 位小写 hex
 * </pre>
 * 注：JDK 17+ 支持 HmacSHA512/256，JDK 11-16 回退 HmacSHA256（与 Python/PHP SDK 回退策略一致）</p>
 *
 * <p>铁律 04：API 地址 / AppKey / SignSecret 由调用方传入，SDK 内不硬编码<br>
 * 铁律 06：所有接口错误抛出 KeyAuthException(code, message)，不静默吞异常</p>
 */
public class KeyAuthClient {

    private static final String VERSION = "0.4.0";

    private final String apiBase;
    private final String appKey;
    private final String signSecret;
    private final HttpClient httpClient;
    private final ObjectMapper mapper;
    private final SecureRandom random;

    /**
     * 构造客户端
     *
     * @param apiBase    后端 API 根地址（如 https://yourdomain.com）
     * @param appKey     应用 AppKey（ak_ 开头）
     * @param signSecret 应用 SignSecret（sk_ 开头，AES 解密后的明文）
     */
    public KeyAuthClient(String apiBase, String appKey, String signSecret) {
        this.apiBase = apiBase.endsWith("/") ? apiBase.substring(0, apiBase.length() - 1) : apiBase;
        this.appKey = appKey;
        this.signSecret = signSecret;
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(5))
                .build();
        this.mapper = new ObjectMapper();
        this.random = new SecureRandom();
    }

    // ==================== 公共 API ====================

    /** Login 登录（首次自动绑定设备） */
    public Map<String, Object> login(String cardKey, String hwid, String deviceName, String deviceType) throws KeyAuthException {
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("card_key", cardKey);
        payload.put("hwid", hwid);
        payload.put("device_name", deviceName);
        payload.put("device_type", deviceType);
        return post("/api/v1/client/login", payload);
    }

    /** Verify 验证卡密有效性（不绑定，不增加使用次数） */
    public Map<String, Object> verify(String cardKey, String hwid) throws KeyAuthException {
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("card_key", cardKey);
        payload.put("hwid", hwid);
        return post("/api/v1/client/verify", payload);
    }

    /** Heartbeat 心跳保活 */
    public Map<String, Object> heartbeat(String cardKey, String hwid) throws KeyAuthException {
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("card_key", cardKey);
        payload.put("hwid", hwid);
        return post("/api/v1/client/heartbeat", payload);
    }

    /** Bind 手动绑定设备 */
    public Map<String, Object> bind(String cardKey, String hwid, String deviceName, String deviceType) throws KeyAuthException {
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("card_key", cardKey);
        payload.put("hwid", hwid);
        payload.put("device_name", deviceName);
        payload.put("device_type", deviceType);
        return post("/api/v1/client/bind", payload);
    }

    /** Unbind 解绑设备（扣时 UnbindDeductSeconds） */
    public Map<String, Object> unbind(String cardKey, String hwid) throws KeyAuthException {
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("card_key", cardKey);
        payload.put("hwid", hwid);
        return post("/api/v1/client/unbind", payload);
    }

    /** GetVar 获取云变量 */
    public Map<String, Object> getVar(String cardKey, String varKey) throws KeyAuthException {
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("card_key", cardKey);
        payload.put("var_key", varKey);
        return post("/api/v1/client/get_var", payload);
    }

    /** Notice 获取应用公告 */
    public Map<String, Object> notice() throws KeyAuthException {
        return post("/api/v1/client/notice", new LinkedHashMap<>());
    }

    /** Version 检查版本更新 */
    public Map<String, Object> version(String currentVersion, String platform) throws KeyAuthException {
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("current_version", currentVersion);
        payload.put("platform", platform);
        return post("/api/v1/client/version", payload);
    }

    /** Logout 退出登录 */
    public Map<String, Object> logout(String cardKey, String hwid) throws KeyAuthException {
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("card_key", cardKey);
        payload.put("hwid", hwid);
        return post("/api/v1/client/logout", payload);
    }

    // ==================== 内部方法 ====================

    private Map<String, Object> post(String path, Map<String, Object> payload) throws KeyAuthException {
        String body;
        try {
            body = mapper.writeValueAsString(payload);
        } catch (Exception e) {
            throw new KeyAuthException(1006, "JSON 编码失败: " + e.getMessage(), 0);
        }

        String timestamp = String.valueOf(System.currentTimeMillis() / 1000);
        String nonce = randomNonce();

        // 签名原文：METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
        String signString = String.join("\n", "POST", path, timestamp, nonce, body);
        String signature = hmacSHA512_256Hex(signSecret, signString);

        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(apiBase + path))
                .header("Content-Type", "application/json")
                .header("X-App-Key", appKey)
                .header("X-Timestamp", timestamp)
                .header("X-Nonce", nonce)
                .header("X-Signature", signature)
                .timeout(Duration.ofSeconds(10))
                .POST(HttpRequest.BodyPublishers.ofString(body, StandardCharsets.UTF_8))
                .build();

        HttpResponse<String> resp;
        try {
            resp = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
        } catch (Exception e) {
            throw new KeyAuthException(1006, "网络请求失败: " + e.getMessage(), 0);
        }

        Map<String, Object> envelope;
        try {
            envelope = mapper.readValue(resp.body(), Map.class);
        } catch (Exception e) {
            String preview = resp.body();
            if (preview != null && preview.length() > 200) {
                preview = preview.substring(0, 200);
            }
            throw new KeyAuthException(1006, "响应非 JSON: " + preview, resp.statusCode());
        }

        Number code = (Number) envelope.get("code");
        int codeVal = code != null ? code.intValue() : -1;
        if (resp.statusCode() != 200 || codeVal != 0) {
            String msg = (String) envelope.getOrDefault("message", "未知错误");
            throw new KeyAuthException(codeVal, msg, resp.statusCode());
        }

        Object data = envelope.get("data");
        if (data instanceof Map) {
            @SuppressWarnings("unchecked")
            Map<String, Object> dataMap = (Map<String, Object>) data;
            return dataMap;
        }
        return new LinkedHashMap<>();
    }

    /** HMAC-SHA512/256 → 64 位小写 hex（与后端 crypto.HMACSHA256 对齐） */
    static String hmacSHA512_256Hex(String secret, String msg) throws KeyAuthException {
        try {
            // 优先尝试 HmacSHA512/256（JDK 17+ 支持）
            Mac mac;
            try {
                mac = Mac.getInstance("HmacSHA512/256");
            } catch (Exception e) {
                // 回退 HmacSHA256（JDK 11-16，与 Python/PHP SDK 回退策略一致）
                mac = Mac.getInstance("HmacSHA256");
            }
            mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), mac.getAlgorithm()));
            byte[] raw = mac.doFinal(msg.getBytes(StandardCharsets.UTF_8));
            return toHex(raw);
        } catch (Exception e) {
            throw new KeyAuthException(1006, "签名计算失败: " + e.getMessage(), 0);
        }
    }

    private String randomNonce() {
        byte[] b = new byte[16];
        random.nextBytes(b);
        return toHex(b);
    }

    private static String toHex(byte[] bytes) {
        StringBuilder sb = new StringBuilder(bytes.length * 2);
        for (byte b : bytes) {
            sb.append(String.format("%02x", b & 0xff));
        }
        return sb.toString();
    }

    /** SDK 版本号 */
    public static String getVersion() {
        return VERSION;
    }
}
