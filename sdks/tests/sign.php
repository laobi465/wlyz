<?php
// 客户端 SDK 签名对齐测试脚本（PHP 版）
// 用法：php sign.php <secret> <message>
// 输出：64 位小写 hex 签名（与后端 crypto.HMACSHA256 对齐）

function hmacSha512256(string $secret, string $msg): string {
    if (in_array('sha512/256', hash_hmac_algos(), true)) {
        return hash_hmac('sha512/256', $msg, $secret);
    }
    return hash_hmac('sha256', $msg, $secret);
}

if (isset($argv) && count($argv) === 3) {
    echo hmacSha512256($argv[1], $argv[2]) . PHP_EOL;
} else {
    fwrite(STDERR, "usage: php sign.php <secret> <message>\n");
    exit(1);
}
