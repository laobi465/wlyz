/**
 * KeyAuth SaaS Node.js SDK
 *
 * 面向终端软件的客户端 SDK，封装 9 个验证 API：
 *   login / verify / heartbeat / bind / unbind / get_var / notice / version / logout
 *
 * 依赖：
 *   - Node.js >= 14（内置 https / crypto / fetch Polyfill）
 *   - 无第三方依赖（用内置 https 模块）
 *
 * 签名算法（与后端 internal/middleware/signature.go 一致）：
 *   原文 = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
 *   签名 = HMAC-SHA512/256(secret, 原文) → 64 位小写 hex
 *   注：后端 sha512.New512_256 变体；Node.js crypto.createHmac('sha512/256', secret) 原生支持
 *
 * 铁律 04：API 地址 / AppKey / SignSecret 由调用方传入，SDK 内不硬编码
 * 铁律 06：所有接口错误抛出 KeyAuthError(code, message)，不静默吞异常
 */

'use strict';

const crypto = require('crypto');
const https = require('https');
const http = require('http');
const { URL } = require('url');

class KeyAuthError extends Error {
  constructor(code, message, httpStatus) {
    super(`[${code}] ${message}`);
    this.name = 'KeyAuthError';
    this.code = code;
    this.message = message;
    this.httpStatus = httpStatus || 0;
  }
}

/**
 * 计算 SHA-512/256 HMAC（与后端 sha512.New512_256 一致）
 * Node.js 14+ 原生支持；不支持时回退 sha256
 */
function hmacSha512_256Hex(secret, msg) {
  try {
    return crypto.createHmac('sha512/256', secret).update(msg, 'utf8').digest('hex');
  } catch (e) {
    // 回退 sha256（兼容性提示，后端 crypto.go:165 已标注待核实）
    return crypto.createHmac('sha256', secret).update(msg, 'utf8').digest('hex');
  }
}

/**
 * 简易 HTTPS/HTTP 请求封装（不依赖 axios）
 * @returns {Promise<{status: number, body: string}>}
 */
function httpRequest(method, urlStr, headers, body) {
  return new Promise((resolve, reject) => {
    const u = new URL(urlStr);
    const lib = u.protocol === 'https:' ? https : http;
    const opts = {
      method,
      hostname: u.hostname,
      port: u.port || (u.protocol === 'https:' ? 443 : 80),
      path: u.pathname + (u.search || ''),
      headers: Object.assign({ 'Content-Length': Buffer.byteLength(body || '') }, headers),
    };
    const req = lib.request(opts, (res) => {
      let chunks = '';
      res.on('data', (d) => { chunks += d; });
      res.on('end', () => resolve({ status: res.statusCode, body: chunks }));
    });
    req.on('error', reject);
    if (body) req.write(body);
    req.end();
  });
}

class KeyAuthClient {
  /**
   * @param {Object} opts
   * @param {string} opts.apiBase - 后端 API 根地址（如 https://yourdomain.com）
   * @param {string} opts.appKey - 应用 AppKey（ak_ 开头）
   * @param {string} opts.signSecret - 应用 SignSecret（明文）
   * @param {number} [opts.timeout=10000] - 超时毫秒
   * @param {string} [opts.appSecret] - 应用 AppSecret（未来加密通信使用）
   */
  constructor(opts) {
    if (!opts || !opts.apiBase || !opts.appKey || !opts.signSecret) {
      throw new KeyAuthError(1001, 'apiBase / appKey / signSecret 必填');
    }
    this.apiBase = opts.apiBase.replace(/\/$/, '');
    this.appKey = opts.appKey;
    this.signSecret = opts.signSecret;
    this.appSecret = opts.appSecret;
    this.timeout = opts.timeout || 10000;
  }

  // ---------------- 公共 API ----------------

  /** 登录（首次自动绑定设备） */
  async login(cardKey, hwid, opts = {}) {
    const payload = {
      card_key: cardKey,
      hwid: hwid,
      device_name: opts.deviceName || '',
      device_type: opts.deviceType || '',
    };
    return this._post('/api/v1/client/login', payload);
  }

  /** 验证卡密有效性（不绑定，不增加使用次数） */
  async verify(cardKey, hwid) {
    return this._post('/api/v1/client/verify', { card_key: cardKey, hwid });
  }

  /** 心跳保活（按 heartbeat_interval 周期调用） */
  async heartbeat(cardKey, hwid) {
    return this._post('/api/v1/client/heartbeat', { card_key: cardKey, hwid });
  }

  /** 手动绑定设备（多机场景；单机应用 login 已自动绑定） */
  async bind(cardKey, hwid, opts = {}) {
    return this._post('/api/v1/client/bind', {
      card_key: cardKey,
      hwid: hwid,
      device_name: opts.deviceName || '',
      device_type: opts.deviceType || '',
    });
  }

  /** 解绑设备（扣时 UnbindDeductSeconds） */
  async unbind(cardKey, hwid) {
    return this._post('/api/v1/client/unbind', { card_key: cardKey, hwid });
  }

  /** 获取云变量 */
  async getVar(cardKey, varKey) {
    return this._post('/api/v1/client/get_var', { card_key: cardKey, var_key: varKey });
  }

  /** 获取应用公告 */
  async notice() {
    return this._post('/api/v1/client/notice', {});
  }

  /** 检查版本更新 */
  async version(currentVersion = '', platform = '') {
    return this._post('/api/v1/client/version', {
      current_version: currentVersion,
      platform,
    });
  }

  /** 退出登录（仅记录日志，不影响设备绑定状态） */
  async logout(cardKey, hwid) {
    return this._post('/api/v1/client/logout', { card_key: cardKey, hwid });
  }

  // ---------------- 内部方法 ----------------

  async _post(path, payload) {
    const body = JSON.stringify(payload);
    const timestamp = String(Math.floor(Date.now() / 1000));
    const nonce = crypto.randomBytes(16).toString('hex');

    // 签名原文：METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY（与后端 signature.go:88 一致）
    const signString = ['POST', path, timestamp, nonce, body].join('\n');
    const signature = hmacSha512_256Hex(this.signSecret, signString);

    const headers = {
      'Content-Type': 'application/json',
      'X-App-Key': this.appKey,
      'X-Timestamp': timestamp,
      'X-Nonce': nonce,
      'X-Signature': signature,
    };

    let resp;
    try {
      resp = await httpRequest('POST', this.apiBase + path, headers, body);
    } catch (e) {
      throw new KeyAuthError(1006, `网络请求失败: ${e.message}`);
    }

    let data;
    try {
      data = JSON.parse(resp.body);
    } catch (e) {
      throw new KeyAuthError(1006, `响应非 JSON: ${resp.body.slice(0, 200)}`);
    }

    if (resp.status !== 200 || data.code !== 0) {
      throw new KeyAuthError(
        data.code || 1006,
        data.message || '未知错误',
        resp.status
      );
    }
    return data.data || {};
  }
}

module.exports = { KeyAuthClient, KeyAuthError };
module.exports.default = { KeyAuthClient, KeyAuthError };
