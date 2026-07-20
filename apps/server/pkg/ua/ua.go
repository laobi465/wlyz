// Package ua User-Agent 解析工具
// 自实现轻量级解析器，覆盖主流浏览器/OS/设备类型，零第三方依赖
// 用于登录设备列表 / 日志审计 / 安全统计等场景
package ua

import (
	"strings"
)

// DeviceInfo 解析后的设备信息
type DeviceInfo struct {
	OS         string // macOS / Windows / Android / iOS / Linux / Unknown
	OSVersion  string // 10.15.7 / 11 / 14.2.1（空表示未知）
	Browser    string // Chrome / Firefox / Safari / Edge / curl / Unknown
	Version    string // 浏览器主版本号（如 90.0.4430.85 -> 90）
	DeviceType string // pc / mobile / tablet / bot / unknown
	DeviceName string // 简短展示名，如 "macOS / Chrome"
}

// 预定义常量
const (
	OSUnknown  = "Unknown"
	OSMacOS    = "macOS"
	OSWindows  = "Windows"
	OSAndroid  = "Android"
	OSiOS      = "iOS"
	OSLinux    = "Linux"

	BrowserUnknown = "Unknown"
	BrowserChrome  = "Chrome"
	BrowserFirefox = "Firefox"
	BrowserSafari  = "Safari"
	BrowserEdge    = "Edge"
	BrowserCurl    = "curl"
	BrowserBot     = "Bot"

	DevicePC     = "pc"
	DeviceMobile = "mobile"
	DeviceTablet = "tablet"
	DeviceBot    = "bot"
	DeviceUnknown = "unknown"
)

// Parse 解析 User-Agent 字符串
// 输入为原始 UA，返回结构化设备信息
// 空字符串或无法识别时返回 Unknown 字段
func Parse(ua string) DeviceInfo {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return DeviceInfo{
			OS:         OSUnknown,
			Browser:    BrowserUnknown,
			DeviceType: DeviceUnknown,
			DeviceName: "Unknown Device",
		}
	}

	osName, osVersion := parseOS(ua)
	browser, browserVersion := parseBrowser(ua)
	deviceType := detectDeviceType(ua, osName)
	deviceName := buildDeviceName(osName, browser)

	return DeviceInfo{
		OS:         osName,
		OSVersion:  osVersion,
		Browser:    browser,
		Version:    browserVersion,
		DeviceType: deviceType,
		DeviceName: deviceName,
	}
}

// IsBot 判断是否为爬虫/机器人
// 用于风控统计与日志区分
func IsBot(ua string) bool {
	uaLower := strings.ToLower(ua)
	botKeywords := []string{
		"bot", "crawler", "spider", "googlebot", "bingbot",
		"baiduspider", "yandexbot", "slurp", "duckduckbot",
	}
	for _, kw := range botKeywords {
		if strings.Contains(uaLower, kw) {
			return true
		}
	}
	return false
}

// parseOS 解析 OS 名称与版本
func parseOS(ua string) (string, string) {
	uaLower := strings.ToLower(ua)

	switch {
	case strings.Contains(uaLower, "windows"):
		// Windows NT 10.0 -> 10
		ver := extractVersion(ua, "windows nt ", ";")
		if ver != "" {
			ver = normalizeWindowsVersion(ver)
		}
		return OSWindows, ver

	case strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad"):
		// CPU OS 14_2_1 like Mac OS X -> 14.2.1
		ver := extractVersion(ua, "cpu os ", " like")
		if ver == "" {
			ver = extractVersion(ua, "cpu iphone os ", " like")
		}
		ver = strings.ReplaceAll(ver, "_", ".")
		return OSiOS, ver

	case strings.Contains(uaLower, "mac os x") || strings.Contains(uaLower, "macintosh"):
		// Mac OS X 10_15_7 -> 10.15.7
		ver := extractVersion(ua, "mac os x ", ")")
		if ver == "" {
			ver = extractVersion(ua, "mac os x ", ";")
		}
		ver = strings.ReplaceAll(ver, "_", ".")
		return OSMacOS, ver

	case strings.Contains(uaLower, "android"):
		// Android 11; -> 11
		ver := extractVersion(ua, "android ", ";")
		return OSAndroid, ver

	case strings.Contains(uaLower, "linux"):
		return OSLinux, ""
	}

	return OSUnknown, ""
}

// parseBrowser 解析浏览器名称与主版本号
// 注意：UA 中浏览器顺序为「实际引擎 → 外壳」，需按从特殊到一般的顺序匹配
func parseBrowser(ua string) (string, string) {
	uaLower := strings.ToLower(ua)

	// 1. Edge（基于 Chromium，需先匹配）
	if strings.Contains(uaLower, "edg/") {
		return BrowserEdge, extractVersion(ua, "Edg/", " ")
	}

	// 2. curl
	if strings.Contains(uaLower, "curl/") {
		return BrowserCurl, extractVersion(ua, "curl/", " ")
	}

	// 3. 爬虫
	if IsBot(ua) {
		return BrowserBot, ""
	}

	// 4. Firefox
	if strings.Contains(uaLower, "firefox/") {
		return BrowserFirefox, extractVersion(ua, "Firefox/", " ")
	}

	// 5. Chrome（注意：Edge UA 也含 Chrome/，但已在上面匹配过 Edge）
	if strings.Contains(uaLower, "chrome/") {
		return BrowserChrome, extractVersion(ua, "Chrome/", " ")
	}

	// 6. Safari（注意：Chrome UA 也含 Safari/，需在 Chrome 之后匹配）
	if strings.Contains(uaLower, "safari/") {
		// Safari 版本号在 Version/ 字段
		ver := extractVersion(ua, "Version/", " ")
		return BrowserSafari, ver
	}

	return BrowserUnknown, ""
}

// detectDeviceType 判定设备类型
func detectDeviceType(ua, osName string) string {
	uaLower := strings.ToLower(ua)

	// 爬虫优先
	if IsBot(ua) {
		return DeviceBot
	}

	// iPad / Android Tablet
	if strings.Contains(uaLower, "ipad") || strings.Contains(uaLower, "tablet") {
		return DeviceTablet
	}

	// iPhone / Android Mobile / 通用 mobile 标识
	switch osName {
	case OSiOS:
		if !strings.Contains(uaLower, "ipad") {
			return DeviceMobile
		}
	case OSAndroid:
		// Android 手机 UA 含 Mobile，平板不含
		if strings.Contains(uaLower, "mobile") {
			return DeviceMobile
		}
		return DeviceTablet
	}

	if strings.Contains(uaLower, "mobile") || strings.Contains(uaLower, "iphone") {
		return DeviceMobile
	}

	// PC 浏览器（Windows/macOS/Linux + Chrome/Firefox/Safari/Edge）
	if osName == OSWindows || osName == OSMacOS || osName == OSLinux {
		return DevicePC
	}

	return DeviceUnknown
}

// buildDeviceName 构造简短设备名
func buildDeviceName(osName, browser string) string {
	if osName == OSUnknown && browser == BrowserUnknown {
		return "Unknown Device"
	}
	if osName == OSUnknown {
		return browser
	}
	if browser == BrowserUnknown {
		return osName
	}
	return osName + " / " + browser
}

// extractVersion 从 UA 中提取版本号
// prefix 大小写敏感（按 UA 原始大小写匹配），suffix 用于界定结束位置
// 返回的版本号已去除末尾的非数字字符（如点号、分号、空格）
func extractVersion(ua, prefix, suffix string) string {
	idx := strings.Index(ua, prefix)
	if idx < 0 {
		// 大小写不敏感兜底
		idx = strings.Index(strings.ToLower(ua), strings.ToLower(prefix))
		if idx < 0 {
			return ""
		}
		// 用小写后的 prefix 长度切原始串
		// 但为简化，直接用 ToLower 的版本（不影响后续提取）
		uaLower := strings.ToLower(ua)
		start := idx + len(prefix)
		rest := uaLower[start:]
		end := strings.Index(rest, suffix)
		if end < 0 {
			end = len(rest)
		}
		return cleanVersion(rest[:end])
	}

	start := idx + len(prefix)
	rest := ua[start:]
	end := strings.Index(rest, suffix)
	if end < 0 {
		end = len(rest)
	}
	return cleanVersion(rest[:end])
}

// cleanVersion 清理版本号字符串
// 去除末尾的非数字字符，保留版本号主体
// 允许数字 / 点号 / 下划线（iOS 与 macOS UA 用 _ 分隔版本，如 10_15_7 / 14_2_1）
// 例：10_15_7 -> 10_15_7（保留 _，由调用方转换为 .）
//     90.0.4430.85 -> 90.0.4430.85
//     11; -> 11
func cleanVersion(s string) string {
	s = strings.TrimSpace(s)
	// 去除末尾分号、空格
	s = strings.TrimRight(s, "; ")
	// 仅保留数字 / 点号 / 下划线
	var b strings.Builder
	hasDigit := false
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == '_' {
			b.WriteRune(r)
			if r >= '0' && r <= '9' {
				hasDigit = true
			}
		} else {
			break
		}
	}
	if !hasDigit {
		return ""
	}
	return b.String()
}

// normalizeWindowsVersion 将 Windows NT 版本号转为用户友好版本
// 10.0 -> 10/11（10.0.22000+ 为 Win11，但 UA 中无法精确判定，统一返回 10）
// 6.3 -> 8.1
// 6.2 -> 8
// 6.1 -> 7
// 6.0 -> Vista
// 5.1 -> XP
func normalizeWindowsVersion(ntVer string) string {
	switch ntVer {
	case "10.0":
		return "10"
	case "6.3":
		return "8.1"
	case "6.2":
		return "8"
	case "6.1":
		return "7"
	case "6.0":
		return "Vista"
	case "5.1":
		return "XP"
	case "5.2":
		return "XP"
	}
	return ntVer
}
