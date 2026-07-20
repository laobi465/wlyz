"""KeyAuth SaaS Python SDK

面向终端软件的客户端 SDK，封装 9 个验证 API：
    login / verify / heartbeat / bind / unbind / get_var / notice / version / logout

依赖：
    - requests >= 2.20（HTTP 客户端）
    - 标准库 hashlib / hmac / uuid / time / json

签名算法（与后端 internal/middleware/signature.go 一致）：
    原文 = METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY
    签名 = HMAC-SHA256(secret, 原文) → 64 位小写 hex
    注：后端使用 sha512.New512_256 变体（SHA-512/256），
        Python 端使用 hashlib.new("sha512_256") 兼容；若环境不支持，回退 hmac.new(key, msg, hashlib.sha256).hexdigest()

铁律 04：API 地址 / AppKey / SignSecret 由调用方传入，SDK 内不硬编码
铁律 06：所有接口错误抛出 KeyAuthError(code, message)，不静默吞异常
"""

from __future__ import annotations

import hashlib
import hmac
import json as _json
import time
import uuid
from typing import Any, Dict, Optional

try:
    import requests  # type: ignore
except ImportError as exc:  # pragma: no cover
    raise ImportError("keyauth-py 依赖 requests，请先 pip install requests") from exc

__all__ = ["KeyAuthClient", "KeyAuthError", "CardInfo", "DeviceInfo"]

__version__ = "0.3.6"


class KeyAuthError(Exception):
    """KeyAuth API 错误（含 code 与 message）"""

    def __init__(self, code: int, message: str, http_status: int = 0):
        super().__init__(f"[{code}] {message}")
        self.code = code
        self.message = message
        self.http_status = http_status


class CardInfo:
    """卡密信息（login/verify 返回）"""

    def __init__(self, data: Dict[str, Any]):
        self.type: str = data.get("type", "")
        self.status: str = data.get("status", "")
        self.expires_at: Optional[int] = data.get("expires_at")
        self.remaining_seconds: int = data.get("remaining_seconds", 0)
        self.bound_devices: int = data.get("bound_devices", 0)
        self.max_devices: int = data.get("max_devices", 0)
        self.used_count: int = data.get("used_count", 0)
        self.max_uses: int = data.get("max_uses", 0)

    def __repr__(self) -> str:
        return (
            f"CardInfo(type={self.type!r}, status={self.status!r}, "
            f"remaining={self.remaining_seconds}s, devices={self.bound_devices}/{self.max_devices})"
        )


class DeviceInfo:
    """设备信息（login 返回）"""

    def __init__(self, data: Dict[str, Any]):
        self.id: int = data.get("id", 0)
        self.hwid: str = data.get("hwid", "")
        self.name: str = data.get("name", "")
        self.bound_at: int = data.get("bound_at", 0)

    def __repr__(self) -> str:
        return f"DeviceInfo(id={self.id}, hwid={self.hwid!r}, name={self.name!r})"


def _sha512_256_hex(key: bytes, msg: bytes) -> str:
    """SHA-512/256 HMAC（与后端 sha512.New512_256 一致）

    优先使用 sha512_256 算法（OpenSSL 1.1+ 支持，通过字符串名传给 hmac.new）；
    若环境不支持则回退 sha256（兼容性提示已标注待核实）。
    """
    if "sha512_256" in hashlib.algorithms_available:
        return hmac.new(key, msg, "sha512_256").hexdigest()
    # 回退到 SHA-256（后端 crypto.go:165 也已标注待核实兼容性）
    return hmac.new(key, msg, hashlib.sha256).hexdigest()


class KeyAuthClient:
    """KeyAuth SaaS 客户端 SDK

    用法：
        client = KeyAuthClient(
            api_base="https://yourdomain.com",
            app_key="ak_xxx",
            sign_secret="sk_xxx",
        )
        result = client.login(card_key="ABCD-1234-EFGH-5678", hwid="cpu-mac-disk-hash")
    """

    def __init__(
        self,
        api_base: str,
        app_key: str,
        sign_secret: str,
        *,
        timeout: float = 10.0,
        app_secret: Optional[str] = None,
    ):
        """
        :param api_base: 后端 API 根地址（如 https://yourdomain.com）
        :param app_key: 应用 AppKey（ak_ 开头）
        :param sign_secret: 应用 SignSecret（sk_ 开头，AES 解密后的明文）
        :param timeout: HTTP 请求超时秒数
        :param app_secret: 应用 AppSecret（可选，未来 SDK 用其加密通信时使用）
        """
        self.api_base = api_base.rstrip("/")
        self.app_key = app_key
        self.sign_secret = sign_secret
        self.app_secret = app_secret
        self.timeout = timeout
        self._session = requests.Session()
        self._session.headers.update({"Content-Type": "application/json"})

    # ---------------- 公共 API ----------------

    def login(
        self,
        card_key: str,
        hwid: str,
        *,
        device_name: str = "",
        device_type: str = "",
    ) -> Dict[str, Any]:
        """登录（首次自动绑定设备）

        :return: {token, expires_at, card: CardInfo, device: DeviceInfo, heartbeat_interval, heartbeat_timeout}
        :raises KeyAuthError: 卡密不存在 / 已封禁 / 设备数超限等
        """
        payload = {
            "card_key": card_key,
            "hwid": hwid,
            "device_name": device_name,
            "device_type": device_type,
        }
        data = self._post("/api/v1/client/login", payload)
        if "card" in data:
            data["card"] = CardInfo(data["card"])
        if "device" in data:
            data["device"] = DeviceInfo(data["device"])
        return data

    def verify(self, card_key: str, hwid: str) -> Dict[str, Any]:
        """验证卡密有效性（不绑定，不增加使用次数）

        :return: {card: CardInfo, device, last_heartbeat_at, heartbeat_interval, heartbeat_timeout}
        """
        payload = {"card_key": card_key, "hwid": hwid}
        data = self._post("/api/v1/client/verify", payload)
        if "card" in data:
            data["card"] = CardInfo(data["card"])
        return data

    def heartbeat(self, card_key: str, hwid: str) -> Dict[str, Any]:
        """心跳保活（按 heartbeat_interval 周期调用）

        :return: {next_heartbeat, heartbeat_timeout, server_time}
        """
        return self._post("/api/v1/client/heartbeat", {"card_key": card_key, "hwid": hwid})

    def bind(
        self,
        card_key: str,
        hwid: str,
        *,
        device_name: str = "",
        device_type: str = "",
    ) -> Dict[str, Any]:
        """手动绑定设备（MaxDevices > 1 多机场景；单机应用 login 时已自动绑定）

        :return: {device_id, bound_at, bound_count, max_devices}
        """
        payload = {
            "card_key": card_key,
            "hwid": hwid,
            "device_name": device_name,
            "device_type": device_type,
        }
        return self._post("/api/v1/client/bind", payload)

    def unbind(self, card_key: str, hwid: str) -> Dict[str, Any]:
        """解绑设备（扣时 UnbindDeductSeconds）

        :return: {unbound, deducted_seconds, message}
        """
        return self._post("/api/v1/client/unbind", {"card_key": card_key, "hwid": hwid})

    def get_var(self, card_key: str, var_key: str) -> Dict[str, Any]:
        """获取云变量

        :return: {var_key, var_value, var_type, updated_at}
        """
        return self._post(
            "/api/v1/client/get_var", {"card_key": card_key, "var_key": var_key}
        )

    def notice(self) -> Dict[str, Any]:
        """获取应用公告

        :return: {notices: [{id, title, content, is_pinned, created_at}, ...]}
        """
        return self._post("/api/v1/client/notice", {})

    def version(self, current_version: str = "", platform: str = "") -> Dict[str, Any]:
        """检查版本更新

        :return: {has_update, force_update, latest_version, current_version, download_url,
                  backup_url, update_description, min_version, released_at}
        """
        payload = {"current_version": current_version, "platform": platform}
        return self._post("/api/v1/client/version", payload)

    def logout(self, card_key: str, hwid: str) -> Dict[str, Any]:
        """退出登录（仅记录日志，不影响设备绑定状态）

        :return: {logged_out: true}
        """
        return self._post("/api/v1/client/logout", {"card_key": card_key, "hwid": hwid})

    # ---------------- 内部方法 ----------------

    def _post(self, path: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        body = _json.dumps(payload, ensure_ascii=False, separators=(",", ":"))
        timestamp = str(int(time.time()))
        nonce = uuid.uuid4().hex

        url = self.api_base + path
        # 签名原文：METHOD\nPATH?QUERY\nTIMESTAMP\nNONCE\nBODY（与后端 signature.go:88 一致）
        sign_string = "\n".join(["POST", path, timestamp, nonce, body])
        signature = _sha512_256_hex(
            self.sign_secret.encode("utf-8"), sign_string.encode("utf-8")
        )

        headers = {
            "X-App-Key": self.app_key,
            "X-Timestamp": timestamp,
            "X-Nonce": nonce,
            "X-Signature": signature,
        }

        try:
            resp = self._session.post(
                url, data=body.encode("utf-8"), headers=headers, timeout=self.timeout
            )
        except requests.RequestException as e:
            raise KeyAuthError(1006, f"网络请求失败: {e}") from e

        try:
            data = resp.json()
        except ValueError as e:
            raise KeyAuthError(1006, f"响应非 JSON: {resp.text[:200]}") from e

        if resp.status_code != 200 or data.get("code", -1) != 0:
            raise KeyAuthError(
                int(data.get("code", 1006)),
                data.get("message", "未知错误"),
                http_status=resp.status_code,
            )

        return data.get("data") or {}
