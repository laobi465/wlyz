#!/usr/bin/env python3
# 客户端 SDK 签名对齐测试脚本（Python 版）
# 用法：python3 sign.py <secret> <message>
# 输出：64 位小写 hex 签名（与后端 crypto.HMACSHA256 对齐）
import hashlib
import hmac
import sys


def sign(secret: str, msg: str) -> str:
    key = secret.encode("utf-8")
    data = msg.encode("utf-8")
    # 优先使用 sha512_256（OpenSSL 1.1+ 支持，通过字符串名传给 hmac.new）
    if "sha512_256" in hashlib.algorithms_available:
        return hmac.new(key, data, "sha512_256").hexdigest()
    # 回退 sha256（兼容性提示，与后端 sha512.New512_256 不同）
    return hmac.new(key, data, hashlib.sha256).hexdigest()


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("usage: sign.py <secret> <message>", file=sys.stderr)
        sys.exit(1)
    print(sign(sys.argv[1], sys.argv[2]))
