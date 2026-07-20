// 客户端 SDK 签名对齐测试脚本（C# 版）
// 用法：
//   dotnet script sign.csx -- <secret> <message>
// 或编译运行：
//   csc sign.cs /out:sign.exe
//   sign.exe <secret> <message>
// 输出：64 位小写 hex 签名（与后端 crypto.HMACSHA256 对齐）
//
// 优先尝试 BouncyCastle；不可用回退 HMACSHA256

using System;
using System.Reflection;
using System.Security.Cryptography;
using System.Text;

class Sign
{
    static int Main(string[] args)
    {
        if (args.Length != 2)
        {
            Console.Error.WriteLine("usage: sign.exe <secret> <message>");
            return 1;
        }
        Console.WriteLine(SignHmac(args[0], args[1]));
        return 0;
    }

    static string SignHmac(string secret, string msg)
    {
        byte[] key = Encoding.UTF8.GetBytes(secret);
        byte[] data = Encoding.UTF8.GetBytes(msg);

        // 探测 BouncyCastle
        var bcType = Type.GetType("Org.BouncyCastle.Crypto.Digests.Sha512_256Digest, BouncyCastle.Cryptography");
        if (bcType != null)
        {
            return BouncyCastleHex(key, data, bcType);
        }

        // 回退 HMACSHA256
        using var h = new HMACSHA256(key);
        byte[] raw = h.ComputeHash(data);
        return BitConverter.ToString(raw).Replace("-", "").ToLowerInvariant();
    }

    static string BouncyCastleHex(byte[] key, byte[] data, Type digestType)
    {
        var digest = Activator.CreateInstance(digestType)!;
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
        return BitConverter.ToString(result).Replace("-", "").ToLowerInvariant();
    }
}
