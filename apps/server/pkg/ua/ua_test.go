// pkg/ua 单元测试
// 覆盖主流浏览器/OS/设备类型/爬虫/空字符串/边界场景
// 铁律 06：所有断言基于已知固定输入，无随机/不确定性
package ua

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParse_ChromeOnMacOS Chrome on macOS（最常见场景）
func TestParse_ChromeOnMacOS(t *testing.T) {
	uaStr := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.85 Safari/537.36"
	info := Parse(uaStr)

	assert.Equal(t, OSMacOS, info.OS)
	assert.Equal(t, "10.15.7", info.OSVersion)
	assert.Equal(t, BrowserChrome, info.Browser)
	assert.Equal(t, "90.0.4430.85", info.Version)
	assert.Equal(t, DevicePC, info.DeviceType)
	assert.Equal(t, "macOS / Chrome", info.DeviceName)
}

// TestParse_FirefoxOnWindows Firefox on Windows 10
func TestParse_FirefoxOnWindows(t *testing.T) {
	uaStr := "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:89.0) Gecko/20100101 Firefox/89.0"
	info := Parse(uaStr)

	assert.Equal(t, OSWindows, info.OS)
	assert.Equal(t, "10", info.OSVersion)
	assert.Equal(t, BrowserFirefox, info.Browser)
	assert.Equal(t, "89.0", info.Version)
	assert.Equal(t, DevicePC, info.DeviceType)
	assert.Equal(t, "Windows / Firefox", info.DeviceName)
}

// TestParse_SafariOnIPhone Safari on iPhone（iOS 移动端）
func TestParse_SafariOnIPhone(t *testing.T) {
	uaStr := "Mozilla/5.0 (iPhone; CPU iPhone OS 14_2_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1"
	info := Parse(uaStr)

	assert.Equal(t, OSiOS, info.OS)
	assert.Equal(t, "14.2.1", info.OSVersion)
	assert.Equal(t, BrowserSafari, info.Browser)
	assert.Equal(t, "14.0", info.Version)
	assert.Equal(t, DeviceMobile, info.DeviceType)
	assert.Equal(t, "iOS / Safari", info.DeviceName)
}

// TestParse_EdgeOnWindows Edge on Windows（Chromium 内核，需先于 Chrome 匹配）
func TestParse_EdgeOnWindows(t *testing.T) {
	uaStr := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.93 Safari/537.36 Edg/90.0.818.62"
	info := Parse(uaStr)

	assert.Equal(t, OSWindows, info.OS)
	assert.Equal(t, BrowserEdge, info.Browser, "Edge UA 含 Chrome/，需先匹配 Edge")
	assert.Equal(t, "90.0.818.62", info.Version)
	assert.Equal(t, DevicePC, info.DeviceType)
	assert.Equal(t, "Windows / Edge", info.DeviceName)
}

// TestParse_ChromeOnAndroidMobile Chrome on Android 手机
func TestParse_ChromeOnAndroidMobile(t *testing.T) {
	uaStr := "Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.91 Mobile Safari/537.36"
	info := Parse(uaStr)

	assert.Equal(t, OSAndroid, info.OS)
	assert.Equal(t, "11", info.OSVersion)
	assert.Equal(t, BrowserChrome, info.Browser)
	assert.Equal(t, "90.0.4430.91", info.Version)
	assert.Equal(t, DeviceMobile, info.DeviceType)
	assert.Equal(t, "Android / Chrome", info.DeviceName)
}

// TestParse_ChromeOnAndroidTablet Chrome on Android 平板（无 Mobile 标识）
func TestParse_ChromeOnAndroidTablet(t *testing.T) {
	uaStr := "Mozilla/5.0 (Linux; Android 11; SM-T870) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.91 Safari/537.36"
	info := Parse(uaStr)

	assert.Equal(t, OSAndroid, info.OS)
	assert.Equal(t, DeviceTablet, info.DeviceType, "Android UA 不含 Mobile 应识别为 tablet")
}

// TestParse_IPad iPad 平板
func TestParse_IPad(t *testing.T) {
	uaStr := "Mozilla/5.0 (iPad; CPU OS 14_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1"
	info := Parse(uaStr)

	assert.Equal(t, OSiOS, info.OS)
	assert.Equal(t, "14.2", info.OSVersion)
	assert.Equal(t, DeviceTablet, info.DeviceType)
}

// TestParse_Curl curl 命令行工具（SDK 测试常用）
func TestParse_Curl(t *testing.T) {
	uaStr := "curl/8.0.1"
	info := Parse(uaStr)

	assert.Equal(t, BrowserCurl, info.Browser)
	assert.Equal(t, "8.0.1", info.Version)
	assert.Equal(t, OSUnknown, info.OS)
	assert.Equal(t, DeviceUnknown, info.DeviceType)
}

// TestParse_Empty 空字符串
func TestParse_Empty(t *testing.T) {
	info := Parse("")

	assert.Equal(t, OSUnknown, info.OS)
	assert.Equal(t, BrowserUnknown, info.Browser)
	assert.Equal(t, DeviceUnknown, info.DeviceType)
	assert.Equal(t, "Unknown Device", info.DeviceName)
}

// TestParse_OnlySpaces 仅空白字符
func TestParse_OnlySpaces(t *testing.T) {
	info := Parse("   \t\n  ")

	assert.Equal(t, OSUnknown, info.OS)
	assert.Equal(t, BrowserUnknown, info.Browser)
	assert.Equal(t, "Unknown Device", info.DeviceName)
}

// TestParse_Googlebot Googlebot 爬虫
func TestParse_Googlebot(t *testing.T) {
	uaStr := "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	info := Parse(uaStr)

	assert.Equal(t, BrowserBot, info.Browser)
	assert.Equal(t, DeviceBot, info.DeviceType)
	assert.True(t, IsBot(uaStr))
}

// TestParse_BaiduSpider 百度爬虫
func TestParse_BaiduSpider(t *testing.T) {
	uaStr := "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)"
	info := Parse(uaStr)

	assert.Equal(t, BrowserBot, info.Browser)
	assert.Equal(t, DeviceBot, info.DeviceType)
	assert.True(t, IsBot(uaStr))
}

// TestParse_LinuxFirefox Linux + Firefox
func TestParse_LinuxFirefox(t *testing.T) {
	uaStr := "Mozilla/5.0 (X11; Linux x86_64; rv:89.0) Gecko/20100101 Firefox/89.0"
	info := Parse(uaStr)

	assert.Equal(t, OSLinux, info.OS)
	assert.Equal(t, BrowserFirefox, info.Browser)
	assert.Equal(t, DevicePC, info.DeviceType)
}

// TestParse_OldWindows 旧版 Windows XP（NT 5.1）
func TestParse_OldWindows(t *testing.T) {
	uaStr := "Mozilla/5.0 (Windows NT 5.1; rv:52.0) Gecko/20100101 Firefox/52.0"
	info := Parse(uaStr)

	assert.Equal(t, OSWindows, info.OS)
	assert.Equal(t, "XP", info.OSVersion, "NT 5.1 应转换为 XP")
}

// TestParse_Windows8 Windows 8（NT 6.2）
func TestParse_Windows8(t *testing.T) {
	uaStr := "Mozilla/5.0 (Windows NT 6.2; Win64; x64) AppleWebKit/537.36 Chrome/90.0.4430.85 Safari/537.36"
	info := Parse(uaStr)

	assert.Equal(t, OSWindows, info.OS)
	assert.Equal(t, "8", info.OSVersion, "NT 6.2 应转换为 8")
}

// TestIsBot 各种爬虫识别
func TestIsBot(t *testing.T) {
	botUAs := []string{
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		"Baiduspider+(+http://www.baidu.com/search/spider.htm)",
		"Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)",
		"DuckDuckBot/1.0; (+http://duckduckgo.com/duckduckbot.html)",
		"Slurp",
	}
	for _, ua := range botUAs {
		assert.True(t, IsBot(ua), "应识别为爬虫: %s", ua)
	}

	normalUAs := []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/90.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 14_2 like Mac OS X) Safari/604.1",
		"curl/8.0",
		"",
	}
	for _, ua := range normalUAs {
		assert.False(t, IsBot(ua), "不应识别为爬虫: %s", ua)
	}
}

// TestParse_SDKUserAgent 客户端 SDK 自定义 UA（v0.3.6 三语言 SDK）
// 测试场景：keyauth-py/1.0 这种自定义 UA 应识别为 Unknown 而非崩溃
func TestParse_SDKUserAgent(t *testing.T) {
	uaStr := "keyauth-py/1.0"
	info := Parse(uaStr)

	require.NotPanics(t, func() {
		_ = Parse(uaStr)
	})
	assert.Equal(t, OSUnknown, info.OS)
	assert.Equal(t, BrowserUnknown, info.Browser)
	assert.Equal(t, DeviceUnknown, info.DeviceType)
	assert.Equal(t, "Unknown Device", info.DeviceName)
}

// TestParse_DeviceNameFormatting DeviceName 拼接逻辑
func TestParse_DeviceNameFormatting(t *testing.T) {
	tests := []struct {
		name      string
		ua        string
		expectName string
	}{
		{"OS+Browser", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/90.0", "macOS / Chrome"},
		{"仅 OS（Linux 无浏览器）", "Mozilla/5.0 (X11; Linux x86_64)", "Linux"},
		{"仅 Browser（curl）", "curl/8.0", "curl"},
		{"Unknown", "unknown-string-xyz", "Unknown Device"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := Parse(tc.ua)
			assert.Equal(t, tc.expectName, info.DeviceName)
		})
	}
}

// TestParse_EdgeBeforeChrome Edge 必须先于 Chrome 匹配
// 验证：Edge UA 同时含 Edg/ 与 Chrome/，不应被识别为 Chrome
func TestParse_EdgeBeforeChrome(t *testing.T) {
	edgeUA := "Mozilla/5.0 (Windows NT 10.0) Chrome/90.0.4430.93 Edg/90.0.818.62"
	info := Parse(edgeUA)
	assert.Equal(t, BrowserEdge, info.Browser, "Edge 必须先于 Chrome 匹配")
	assert.NotEqual(t, BrowserChrome, info.Browser)
}

// TestParse_SafariAfterChrome Safari 必须在 Chrome 之后匹配
// 验证：Chrome UA 同时含 Chrome/ 与 Safari/，不应被识别为 Safari
func TestParse_SafariAfterChrome(t *testing.T) {
	chromeUA := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/90.0.4430.85 Safari/537.36"
	info := Parse(chromeUA)
	assert.Equal(t, BrowserChrome, info.Browser, "Chrome 必须先于 Safari 匹配（Chrome UA 含 Safari/）")
	assert.NotEqual(t, BrowserSafari, info.Browser)
}

// TestParse_VersionExtraction 版本号提取
func TestParse_VersionExtraction(t *testing.T) {
	tests := []struct {
		name    string
		ua      string
		wantVer string
	}{
		{"Chrome 完整版本", "Mozilla/5.0 Chrome/90.0.4430.85 Safari/537.36", "90.0.4430.85"},
		{"Firefox 完整版本", "Mozilla/5.0 Firefox/89.0", "89.0"},
		{"Safari Version 字段", "Mozilla/5.0 Version/14.0 Safari/604.1", "14.0"},
		{"curl 版本", "curl/8.0.1", "8.0.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := Parse(tc.ua)
			assert.Equal(t, tc.wantVer, info.Version)
		})
	}
}
