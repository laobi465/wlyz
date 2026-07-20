// 客户端 SDK 签名对齐测试脚本（Java 版）
// 用法：java Sign <secret> <message>
// 输出：64 位小写 hex 签名（与后端 crypto.HMACSHA256 对齐）
//
// 编译：javac Sign.java
// 运行：java Sign <secret> <message>
//
// JDK 17+ 支持 HmacSHA512/256；JDK 11-16 回退 HmacSHA256
// 退出码 2 视为环境限制（JDK 不支持 HmacSHA512/256 且策略禁止回退）—— 当前实现总是回退，不退出 2

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;

public class Sign {

    public static void main(String[] args) throws Exception {
        if (args.length != 2) {
            System.err.println("usage: java Sign <secret> <message>");
            System.exit(1);
        }
        System.out.println(sign(args[0], args[1]));
    }

    static String sign(String secret, String msg) throws Exception {
        Mac mac;
        try {
            mac = Mac.getInstance("HmacSHA512/256");
        } catch (Exception e) {
            // 回退 HmacSHA256（与 SDK 一致）
            mac = Mac.getInstance("HmacSHA256");
        }
        mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), mac.getAlgorithm()));
        byte[] raw = mac.doFinal(msg.getBytes(StandardCharsets.UTF_8));
        return toHex(raw);
    }

    private static String toHex(byte[] bytes) {
        StringBuilder sb = new StringBuilder(bytes.length * 2);
        for (byte b : bytes) {
            sb.append(String.format("%02x", b & 0xff));
        }
        return sb.toString();
    }
}
