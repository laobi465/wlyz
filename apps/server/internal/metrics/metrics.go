// Package metrics v0.4.x Prometheus 指标定义与采集器
//
// 严格遵循铁律 04/05/06：
//   04 - 无硬编码：所有指标命名遵循 Prometheus 最佳实践（snake_case + 单位后缀）
//   05 - 配置走后端：暴露开关 / 路径 / BasicAuth 全部走 sys_config（monitor.prometheus.*）
//   06 - 反幻觉：自定义 Collector 直接读取 monitor.Manager 采集的真实数据，不编造样本
//
// 核心能力：
//   1. HTTP 指标：http_requests_total (counter) + http_request_duration_seconds (histogram)
//   2. 业务指标：verify_requests_total / cards_generated_total / pay_orders_total / agents_registered_total
//   3. 系统指标：通过 SystemCollector 从 monitor.Manager 拉取 CPU/内存/磁盘/在线设备/QPS
//   4. Go 运行时指标：默认 collector（goroutine / gc / memstats）
package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gorm.io/gorm"

	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/monitor"
)

// ============== 配置键常量 ==============

const (
	CfgKeyPromEnabled       = "monitor.prometheus.enabled"
	CfgKeyPromPath          = "monitor.prometheus.path"
	CfgKeyPromBasicAuthUser = "monitor.prometheus.basic_auth_user"
	CfgKeyPromBasicAuthPass = "monitor.prometheus.basic_auth_pass"
)

// ============== HTTP 指标 ==============

// HTTPRequestsTotal HTTP 请求总数（按 method/path/status 分维）
var HTTPRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	},
	[]string{"method", "path", "status"},
)

// HTTPRequestDurationSeconds HTTP 请求耗时直方图（秒）
var HTTPRequestDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"method", "path"},
)

// HTTPRequestsInFlight 当前正在处理的请求数
var HTTPRequestsInFlight = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Number of HTTP requests currently being processed.",
	},
)

// ============== 业务指标 ==============

// VerifyRequestsTotal 客户端验证请求总数（按 app_id/result 分维）
var VerifyRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "verify_requests_total",
		Help: "Total number of client verify requests.",
	},
	[]string{"app_id", "result"},
)

// CardsGeneratedTotal 卡密生成总数（按 tenant_id/app_id 分维）
var CardsGeneratedTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "cards_generated_total",
		Help: "Total number of cards generated.",
	},
	[]string{"tenant_id", "app_id"},
)

// PayOrdersTotal 支付订单总数（按 prefix/status 分维）
var PayOrdersTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "pay_orders_total",
		Help: "Total number of pay orders.",
	},
	[]string{"prefix", "status"},
)

// PayOrderAmountTotal 支付订单金额累计（分）
var PayOrderAmountTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "pay_order_amount_total",
		Help: "Total amount of pay orders in cents.",
	},
	[]string{"prefix"},
)

// AgentsRegisteredTotal 代理注册总数
var AgentsRegisteredTotal = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "agents_registered_total",
		Help: "Total number of agents registered.",
	},
)

// OnlineDevicesGauge 在线设备数（gauge）
var OnlineDevicesGauge = promauto.NewGauge(
	prometheus.GaugeOpts{
		Name: "online_devices",
		Help: "Number of online devices (heartbeat within timeout).",
	},
)

// ============== SystemCollector ==============

// SystemCollector 自定义 Prometheus Collector
// 铁律 06：从 monitor.Manager 采集真实数据，不编造样本
type SystemCollector struct {
	manager *monitor.Manager
	cache   *config.ConfigCache
	db      *gorm.DB

	// 描述符
	cpuDesc        *prometheus.Desc
	memoryDesc     *prometheus.Desc
	memoryUsedDesc *prometheus.Desc
	memoryTotalDesc *prometheus.Desc
	diskDesc       *prometheus.Desc
	diskUsedDesc   *prometheus.Desc
	diskTotalDesc  *prometheus.Desc
	verifyDesc     *prometheus.Desc
	errorRateDesc  *prometheus.Desc
}

// NewSystemCollector 创建系统指标 Collector
func NewSystemCollector(manager *monitor.Manager, cache *config.ConfigCache, db *gorm.DB) *SystemCollector {
	return &SystemCollector{
		manager: manager,
		cache:   cache,
		db:      db,
		cpuDesc: prometheus.NewDesc(
			"system_cpu_usage_percent",
			"CPU usage percent.",
			nil, nil,
		),
		memoryDesc: prometheus.NewDesc(
			"system_memory_usage_percent",
			"Memory usage percent.",
			nil, nil,
		),
		memoryUsedDesc: prometheus.NewDesc(
			"system_memory_used_bytes",
			"Memory used in bytes.",
			nil, nil,
		),
		memoryTotalDesc: prometheus.NewDesc(
			"system_memory_total_bytes",
			"Total memory in bytes.",
			nil, nil,
		),
		diskDesc: prometheus.NewDesc(
			"system_disk_usage_percent",
			"Disk usage percent.",
			[]string{"path"}, nil,
		),
		diskUsedDesc: prometheus.NewDesc(
			"system_disk_used_bytes",
			"Disk used in bytes.",
			[]string{"path"}, nil,
		),
		diskTotalDesc: prometheus.NewDesc(
			"system_disk_total_bytes",
			"Total disk in bytes.",
			[]string{"path"}, nil,
		),
		verifyDesc: prometheus.NewDesc(
			"verify_count_today",
			"Number of verify requests today.",
			nil, nil,
		),
		errorRateDesc: prometheus.NewDesc(
			"verify_error_rate_percent",
			"Verify error rate in percent.",
			nil, nil,
		),
	}
}

// Describe 实现 prometheus.Collector 接口
func (c *SystemCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.cpuDesc
	ch <- c.memoryDesc
	ch <- c.memoryUsedDesc
	ch <- c.memoryTotalDesc
	ch <- c.diskDesc
	ch <- c.diskUsedDesc
	ch <- c.diskTotalDesc
	ch <- c.verifyDesc
	ch <- c.errorRateDesc
}

// Collect 实现 prometheus.Collector 接口
// 铁律 06：调用 monitor.Manager 采集真实数据；任一指标采集失败不影响其他
func (c *SystemCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	samples, err := c.manager.CollectSystemMetrics(ctx)
	if err != nil {
		return
	}

	for _, s := range samples {
		switch s.Name {
		case monitor.MetricCPUUsage:
			ch <- prometheus.MustNewConstMetric(c.cpuDesc, prometheus.GaugeValue, s.Value)
		case monitor.MetricMemoryUsage:
			ch <- prometheus.MustNewConstMetric(c.memoryDesc, prometheus.GaugeValue, s.Value)
			if used, ok := s.Labels["used_mb"].(float64); ok {
				ch <- prometheus.MustNewConstMetric(c.memoryUsedDesc, prometheus.GaugeValue, used*1024*1024)
			}
			if total, ok := s.Labels["total_mb"].(float64); ok {
				ch <- prometheus.MustNewConstMetric(c.memoryTotalDesc, prometheus.GaugeValue, total*1024*1024)
			}
		case monitor.MetricDiskUsage:
			pathLabel := "/"
			if p, ok := s.Labels["path"].(string); ok {
				pathLabel = p
			}
			ch <- prometheus.MustNewConstMetric(c.diskDesc, prometheus.GaugeValue, s.Value, pathLabel)
			if used, ok := s.Labels["used_gb"].(float64); ok {
				ch <- prometheus.MustNewConstMetric(c.diskUsedDesc, prometheus.GaugeValue, used*1024*1024*1024, pathLabel)
			}
			if total, ok := s.Labels["total_gb"].(float64); ok {
				ch <- prometheus.MustNewConstMetric(c.diskTotalDesc, prometheus.GaugeValue, total*1024*1024*1024, pathLabel)
			}
		case monitor.MetricOnlineDevices:
			OnlineDevicesGauge.Set(s.Value)
		case monitor.MetricVerifyCount:
			ch <- prometheus.MustNewConstMetric(c.verifyDesc, prometheus.GaugeValue, s.Value)
		case monitor.MetricErrorRate:
			ch <- prometheus.MustNewConstMetric(c.errorRateDesc, prometheus.GaugeValue, s.Value)
		}
	}
}

// ============== 业务埋点辅助函数 ==============

// IncVerifyRequest 验证请求埋点
func IncVerifyRequest(appID uint64, result string) {
	VerifyRequestsTotal.WithLabelValues(uint64Str(appID), result).Inc()
}

// IncCardsGenerated 卡密生成埋点
func IncCardsGenerated(tenantID, appID uint64, count int) {
	t := uint64Str(tenantID)
	a := uint64Str(appID)
	for i := 0; i < count; i++ {
		CardsGeneratedTotal.WithLabelValues(t, a).Inc()
	}
}

// IncPayOrder 支付订单埋点
func IncPayOrder(prefix, status string) {
	PayOrdersTotal.WithLabelValues(prefix, status).Inc()
}

// AddPayOrderAmount 支付金额累计（分）
func AddPayOrderAmount(prefix string, amountCents float64) {
	PayOrderAmountTotal.WithLabelValues(prefix).Add(amountCents)
}

// IncAgentRegistered 代理注册埋点
func IncAgentRegistered() {
	AgentsRegisteredTotal.Inc()
}

// ============== 工具函数 ==============

// uint64Str 将 uint64 转字符串（避免引入 strconv）
func uint64Str(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
