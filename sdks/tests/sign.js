#!/usr/bin/env node
// 客户端 SDK 签名对齐测试脚本（Node.js 版）
// 用法：node sign.js <secret> <message>
// 输出：64 位小写 hex 签名（与后端 crypto.HMACSHA256 对齐）
// 退出码：0 成功；2 当前 Node 编译的 OpenSSL 不支持 sha512/256（环境限制）
'use strict';
const crypto = require('crypto');

function isSha512_256Supported() {
    try {
        crypto.createHmac('sha512/256', 'test').update('x').digest('hex');
        return true;
    } catch (e) {
        return false;
    }
}

function sign(secret, msg) {
    // 仅在 sha512/256 可用时计算签名；否则退出码 2（与 SDK 的 sha256 fallback 区分）
    if (!isSha512_256Supported()) {
        console.error('UNSUPPORTED: sha512/256 not available in this Node build');
        process.exit(2);
    }
    return crypto.createHmac('sha512/256', secret).update(msg, 'utf8').digest('hex');
}

if (require.main === module) {
    const [,, secret, msg] = process.argv;
    if (!secret || msg === undefined) {
        console.error('usage: sign.js <secret> <message>');
        process.exit(1);
    }
    console.log(sign(secret, msg));
}
