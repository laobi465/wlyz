// Package model 定义所有数据模型（GORM 映射）
// 严格遵循分层规范：model 层只包含纯数据结构，不含业务方法
package model

import "time"

// BaseModel 公共字段
type BaseModel struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

// ============== 平台层 ==============

// SysConfig 系统配置表（铁律 05 核心）
type SysConfig struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigKey   string    `gorm:"uniqueIndex;size:128;not null" json:"config_key"`
	ConfigValue string    `gorm:"type:text" json:"config_value"`
	ConfigType  string    `gorm:"size:32;not null;default:string" json:"config_type"`   // string/number/bool/json
	ConfigName  string    `gorm:"size:128" json:"config_name"`
	ConfigGroup string    `gorm:"size:64;index;not null;default:system" json:"config_group"`
	Remark      string    `gorm:"size:255" json:"remark"`
	CreatedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (SysConfig) TableName() string { return "sys_config" }

// SysAdmin 平台超管账号
type SysAdmin struct {
	BaseModel
	Username     string `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string `gorm:"size:255;not null" json:"-"` // bcrypt cost=12
	Email        string `gorm:"size:128" json:"email"`
	Phone        string `gorm:"size:32" json:"phone"`
	Status       string `gorm:"size:32;not null;default:active" json:"status"` // active/disabled
	TOTPSecret   string `gorm:"size:64" json:"-"`                               // 2FA 密钥（AES 加密）
	BackupCodes  string `gorm:"size:512" json:"-"`                              // v0.4.0：2FA 备用码（AES 加密的逗号分隔字符串）
	LastLoginAt  *time.Time `json:"last_login_at"`
	LastLoginIP  string `gorm:"size:45" json:"last_login_ip"`
}

func (SysAdmin) TableName() string { return "sys_admin" }

// SysTenant 租户（开发者）
type SysTenant struct {
	BaseModel
	TenantCode   string  `gorm:"uniqueIndex;size:32;not null" json:"tenant_code"`
	Username     string  `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string  `gorm:"size:255;not null" json:"-"`
	Email        string  `gorm:"size:128" json:"email"`
	Phone        string  `gorm:"size:32" json:"phone"`
	Company      string  `gorm:"size:128" json:"company"`
	Status       string  `gorm:"size:32;index;not null;default:pending" json:"status"` // pending/active/suspended/deleted
	PackageID    uint64  `gorm:"index;not null" json:"package_id"`
	ExpiresAt    *time.Time `gorm:"index" json:"expires_at"`
	TOTPSecret   string  `gorm:"size:64" json:"-"`
	BackupCodes  string  `gorm:"size:512" json:"-"` // v0.4.0：2FA 备用码（AES 加密的逗号分隔字符串）
	LastLoginAt  *time.Time `json:"last_login_at"`
	LastLoginIP  string  `gorm:"size:45" json:"last_login_ip"`
	Balance      float64 `gorm:"type:decimal(12,2);not null;default:0" json:"balance"`         // v0.3.4：可提现余额
	FrozenBalance float64 `gorm:"type:decimal(12,2);not null;default:0" json:"frozen_balance"` // v0.3.4：冻结余额（提现申请中）
	Remark       string  `gorm:"size:255" json:"remark"`
}

func (SysTenant) TableName() string { return "sys_tenant" }

// SysPackage 平台套餐
type SysPackage struct {
	BaseModel
	Name                  string  `gorm:"size:64;not null" json:"name"`
	Description           string  `gorm:"size:255" json:"description"`
	MonthlyPrice          float64 `gorm:"type:decimal(10,2);not null;default:0" json:"monthly_price"`
	YearlyPrice           float64 `gorm:"type:decimal(10,2);not null;default:0" json:"yearly_price"`
	MaxApps               int     `gorm:"not null;default:1" json:"max_apps"`
	MaxCards              int     `gorm:"not null;default:1000" json:"max_cards"`
	MaxAgents             int     `gorm:"not null;default:0" json:"max_agents"`
	AllowCustomPay        bool    `gorm:"not null;default:false" json:"allow_custom_pay"`
	CustomPayFee          float64 `gorm:"type:decimal(10,2);not null;default:0" json:"custom_pay_fee"`
	PlatformCommissionRate float64 `gorm:"type:decimal(5,2);not null;default:5.00" json:"platform_commission_rate"`
	Features              string  `gorm:"type:json" json:"features"`
	Status                string  `gorm:"size:32;not null;default:active" json:"status"`
}

func (SysPackage) TableName() string { return "sys_package" }

// TenantPayConfig 租户自有易支付配置
type TenantPayConfig struct {
	BaseModel
	TenantID       uint64 `gorm:"uniqueIndex:uk_tenant_channel;not null" json:"tenant_id"`
	Channel        string `gorm:"uniqueIndex:uk_tenant_channel;size:32;not null;default:epay" json:"channel"`
	Enabled        bool   `gorm:"not null;default:false" json:"enabled"`
	GatewayURL     string `gorm:"size:255" json:"gateway_url"`
	PID            string `gorm:"size:64" json:"pid"`
	KeyEncrypted   string `gorm:"type:text" json:"-"` // AES-256-GCM 加密
	Methods        string `gorm:"type:json" json:"methods"` // ["wechat","alipay","qq"]
	NotifyPath     string `gorm:"size:255" json:"notify_path"`
	ReturnPath     string `gorm:"size:255" json:"return_path"`
	LastTestAt     *time.Time `json:"last_test_at"`
	LastTestResult string `gorm:"size:32" json:"last_test_result"`
}

func (TenantPayConfig) TableName() string { return "tenant_pay_config" }

// ============== 应用层 ==============

// App 开发者应用
type App struct {
	BaseModel
	TenantID            uint64 `gorm:"index;not null" json:"tenant_id"`
	AppKey              string `gorm:"uniqueIndex;size:64;not null" json:"app_key"`
	AppSecret           string `gorm:"size:255;not null" json:"-"` // AES 加密
	SignSecret          string `gorm:"size:255;not null" json:"-"` // AES 加密（旧密钥轮换保留）
	SignSecretPrev      string `gorm:"size:255" json:"-"`          // 旧签名密钥（轮换期保留 7 天）
	Name                string `gorm:"size:128;not null" json:"name"`
	Description         string `gorm:"type:text" json:"description"`
	Icon                string `gorm:"size:255" json:"icon"`
	Status              string `gorm:"size:32;index;not null;default:active" json:"status"`
	MaxDevices          int    `gorm:"not null;default:1" json:"max_devices"`
	HeartbeatInterval   int    `gorm:"not null;default:60" json:"heartbeat_interval"`
	HeartbeatTimeout    int    `gorm:"not null;default:180" json:"heartbeat_timeout"`
	OfflineGrace        int    `gorm:"not null;default:86400" json:"offline_grace"`
	UnbindDeductSeconds int    `gorm:"not null;default:86400" json:"unbind_deduct_seconds"`
	AgentCommissionMode string `gorm:"size:32;not null;default:diff" json:"agent_commission_mode"` // percentage/diff
	// v0.4.x S-04：应用审核（pending/approved/rejected）；app.audit.enabled=1 时新应用初始为 pending
	AuditStatus string     `gorm:"size:16;index;not null;default:approved" json:"audit_status"`
	AuditRemark string     `gorm:"size:255;not null;default:''" json:"audit_remark"`
	AuditedAt  *time.Time `json:"audited_at"`
	AuditedBy  uint64     `gorm:"not null;default:0" json:"audited_by"`
}

func (App) TableName() string { return "app" }

// AppCardType 卡类套餐
type AppCardType struct {
	BaseModel
	TenantID        uint64  `gorm:"index;not null" json:"tenant_id"`
	AppID           uint64  `gorm:"index;not null" json:"app_id"`
	Name            string  `gorm:"size:64;not null" json:"name"`
	Type            string  `gorm:"size:32;not null" json:"type"` // duration/count/permanent/trial/feature
	DurationSeconds int64   `gorm:"not null;default:0" json:"duration_seconds"` // 永久卡=-1
	MaxUses         int     `gorm:"not null;default:1" json:"max_uses"`
	Price           float64 `gorm:"type:decimal(10,2);not null;default:0" json:"price"`
	AgentBasePrice  float64 `gorm:"type:decimal(10,2);not null;default:0" json:"agent_base_price"`
	Features        string  `gorm:"type:json" json:"features"`
	Status          string  `gorm:"size:32;not null;default:active" json:"status"`
}

func (AppCardType) TableName() string { return "app_card_type" }

// AppCard 卡密
type AppCard struct {
	BaseModel
	TenantID         uint64     `gorm:"index;not null" json:"tenant_id"`
	AppID            uint64     `gorm:"index;not null" json:"app_id"`
	CardTypeID       uint64     `gorm:"not null" json:"card_type_id"`
	CardKey          string     `gorm:"size:64;not null" json:"card_key"`
	CardKeyHash      string     `gorm:"uniqueIndex;size:128;not null" json:"-"` // SHA-512
	Checksum         string     `gorm:"size:8;not null" json:"checksum"`
	Status           string     `gorm:"size:32;index;not null;default:unused" json:"status"` // unused/active/expired/banned/disabled
	BatchNo          string     `gorm:"index;size:32" json:"batch_no"`
	Prefix           string     `gorm:"size:16" json:"prefix"`
	GroupTag         string     `gorm:"size:64" json:"group_tag"`
	DurationSeconds  int64      `gorm:"not null" json:"duration_seconds"`
	UsedCount        int        `gorm:"not null;default:0" json:"used_count"`
	MaxUses          int        `gorm:"not null;default:1" json:"max_uses"`
	BoundDeviceID    *uint64    `gorm:"index" json:"bound_device_id"`
	EndUserID        *uint64    `gorm:"index" json:"end_user_id"` // v0.4.0 终端用户绑定（可空，向前兼容）
	ActivatedAt      *time.Time `gorm:"index" json:"activated_at"`
	ExpiresAt        *time.Time `gorm:"index" json:"expires_at"`
	LastVerifyAt     *time.Time `json:"last_verify_at"`
	CreatedBy        uint64     `gorm:"not null" json:"created_by"`
	CreatorType      string     `gorm:"size:32;not null;default:tenant" json:"creator_type"`
	OrderID          *uint64    `gorm:"index" json:"order_id"`
	BannedAt         *time.Time `json:"banned_at"`
	BannedReason     string     `gorm:"size:255" json:"banned_reason"`
}

func (AppCard) TableName() string { return "app_card" }

// AppDevice 设备绑定
type AppDevice struct {
	BaseModel
	TenantID          uint64     `gorm:"index;not null" json:"tenant_id"`
	AppID             uint64     `gorm:"index;not null" json:"app_id"`
	CardID            uint64     `gorm:"index;not null" json:"card_id"`
	HWID              string     `gorm:"size:128;not null" json:"hwid"`
	HWIDRaw           string     `gorm:"type:text" json:"hwid_raw"`
	HWIDComponents    string     `gorm:"type:text" json:"hwid_components"`     // v0.4.0 多维指纹 JSON（cpu/motherboard/mac/disk/bios 等）
	UserAgent         string     `gorm:"size:512" json:"user_agent"`           // v0.4.0 客户端 UA
	ClientIPExt       string     `gorm:"size:45" json:"client_ip_ext"`         // v0.4.0 首次绑定 IP
	ScreenResolution  string     `gorm:"size:32" json:"screen_resolution"`     // v0.4.0 屏幕分辨率
	Timezone          string     `gorm:"size:64" json:"timezone"`              // v0.4.0 客户端时区
	Language          string     `gorm:"size:32" json:"language"`              // v0.4.0 客户端语言
	DeviceName        string     `gorm:"size:128" json:"device_name"`
	DeviceType        string     `gorm:"size:32" json:"device_type"`
	IPAddress         string     `gorm:"size:45" json:"ip_address"`
	Status            string     `gorm:"size:32;index;not null;default:active" json:"status"` // active/offline/banned/unbound
	LastHeartbeatAt   *time.Time `gorm:"index" json:"last_heartbeat_at"`
	FirstBoundAt      time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"first_bound_at"`
	UnboundAt         *time.Time `json:"unbound_at"`
}

func (AppDevice) TableName() string { return "app_device" }

// AppOrder 订单
type AppOrder struct {
	BaseModel
	TenantID         uint64  `gorm:"index;not null" json:"tenant_id"`
	AppID            uint64  `gorm:"index;not null" json:"app_id"`
	CardTypeID       uint64  `gorm:"not null" json:"card_type_id"`
	OrderNo          string  `gorm:"uniqueIndex;size:64;not null" json:"order_no"`
	BuyerUserID      *uint64 `gorm:"index" json:"buyer_user_id"`
	BuyerContact     string  `gorm:"size:128" json:"buyer_contact"`
	AgentID          *uint64 `gorm:"index" json:"agent_id"`
	Quantity         int     `gorm:"not null;default:1" json:"quantity"`
	UnitPrice        float64 `gorm:"type:decimal(10,2);not null" json:"unit_price"`
	TotalAmount      float64 `gorm:"type:decimal(10,2);not null" json:"total_amount"`
	CommissionAmount float64 `gorm:"type:decimal(10,2);not null;default:0" json:"commission_amount"`
	PayChannel       string  `gorm:"size:32;not null" json:"pay_channel"` // epay_wechat/epay_alipay/epay_qq/manual/balance
	PayStatus        string  `gorm:"size:32;index;not null;default:pending" json:"pay_status"`
	PayTradeNo       string  `gorm:"size:128" json:"pay_trade_no"`
	PaidAt           *time.Time `gorm:"index" json:"paid_at"`
	CardIDs          string  `gorm:"type:json" json:"card_ids"`
	RefundAmount     float64 `gorm:"type:decimal(10,2)" json:"refund_amount"`
	RefundedAt       *time.Time `json:"refunded_at"`
	ClientIP         string  `gorm:"size:45" json:"client_ip"`
}

func (AppOrder) TableName() string { return "app_order" }

// AppCloudVar 云变量
type AppCloudVar struct {
	BaseModel
	TenantID  uint64 `gorm:"index;not null" json:"tenant_id"`
	AppID     uint64 `gorm:"index;not null" json:"app_id"`
	VarKey    string `gorm:"size:128;not null" json:"var_key"`
	VarValue  string `gorm:"type:text" json:"var_value"`
	VarType   string `gorm:"size:32;not null;default:string" json:"var_type"`
	ReadOnly  bool   `gorm:"not null;default:false" json:"read_only"`
	Remark    string `gorm:"size:255" json:"remark"`
	Status    string `gorm:"size:32;not null;default:active" json:"status"`
}

func (AppCloudVar) TableName() string { return "app_cloud_var" }

// AppVersion 应用版本
type AppVersion struct {
	BaseModel
	TenantID           uint64 `gorm:"index;not null" json:"tenant_id"`
	AppID              uint64 `gorm:"index;not null" json:"app_id"`
	Version            string `gorm:"size:32;not null" json:"version"`
	Channel            string `gorm:"size:32;not null;default:stable" json:"channel"` // stable/beta/dev
	ReleaseStrategy    string `gorm:"size:32;not null;default:full" json:"release_strategy"`     // v0.4.0：full=全量 / grayscale=灰度 / canary=金丝雀
	GrayscaleRate      float64 `gorm:"type:decimal(5,2);not null;default:0" json:"grayscale_rate"` // v0.4.0：灰度比例 0-100，grayscale 策略下生效
	GrayscalePlatforms string `gorm:"size:200;not null;default:''" json:"grayscale_platforms"`    // v0.4.0：逗号分隔 windows/macos/linux/android/ios，空=不限
	GrayscaleRegions   string `gorm:"size:500;not null;default:''" json:"grayscale_regions"`      // v0.4.0：逗号分隔省/州代码，空=不限
	GrayscaleChannels  string `gorm:"size:200;not null;default:''" json:"grayscale_channels"`     // v0.4.0：逗号分隔 stable/beta/dev，空=不限
	MinVersion         string `gorm:"size:32;not null" json:"min_version"`
	DownloadURL        string `gorm:"size:255" json:"download_url"`
	BackupURL          string `gorm:"size:255" json:"backup_url"`
	ForceUpdate        bool   `gorm:"not null;default:false" json:"force_update"`
	UpdateContent      string `gorm:"type:text" json:"update_content"`
	Status             string `gorm:"size:32;not null;default:active" json:"status"`
}

func (AppVersion) TableName() string { return "app_version" }

// ============== 代理层 ==============

// Agent 代理商
type Agent struct {
	BaseModel
	TenantID              uint64  `gorm:"index;not null" json:"tenant_id"`
	Username              string  `gorm:"uniqueIndex:uk_tenant_username;size:64;not null" json:"username"`
	PasswordHash          string  `gorm:"size:255;not null" json:"-"`
	RealName              string  `gorm:"size:64" json:"real_name"`
	Phone                 string  `gorm:"size:32" json:"phone"`
	Email                 string  `gorm:"size:128" json:"email"`
	Status                string  `gorm:"size:32;index;not null;default:active" json:"status"`
	Balance               float64 `gorm:"type:decimal(12,2);not null;default:0" json:"balance"`
	CommissionRate        float64 `gorm:"type:decimal(5,2);not null;default:10.00" json:"commission_rate"`
	CommissionMode        string  `gorm:"size:32;not null;default:percentage" json:"commission_mode"` // percentage/diff
	InviterID             *uint64 `gorm:"index" json:"inviter_id"`
	ParentID              uint64  `gorm:"index;not null;default:0" json:"parent_id"`  // v0.4.0：上级代理 ID（0=一级代理）
	Level                 int     `gorm:"not null;default:1" json:"level"`           // v0.4.0：代理层级（1/2/3，最大 3）
	TOTPSecret            string  `gorm:"size:64" json:"-"` // 2FA 密钥（AES 加密）
	BackupCodes           string  `gorm:"size:512" json:"-"` // v0.4.0：2FA 备用码（AES 加密的逗号分隔字符串）
	Subdomain             string  `gorm:"size:64" json:"subdomain"`
	SubdomainStatus       string  `gorm:"size:16;index;not null;default:none" json:"subdomain_status"` // v0.4.x：none/pending/approved/rejected
	LastLoginAt           *time.Time `json:"last_login_at"`
	LastLoginIP           string  `gorm:"size:45" json:"last_login_ip"`
}

func (Agent) TableName() string { return "agent" }

// AgentInviteCode 代理邀请码
type AgentInviteCode struct {
	BaseModel
	TenantID              uint64 `gorm:"index;not null" json:"tenant_id"`
	Code                  string `gorm:"uniqueIndex;size:32;not null" json:"code"`
	MaxUses               int    `gorm:"not null;default:1" json:"max_uses"`
	UsedCount             int    `gorm:"not null;default:0" json:"used_count"`
	UsedByAgentID         *uint64 `gorm:"index" json:"used_by_agent_id"`
	ValidDays             int    `gorm:"not null;default:30" json:"valid_days"`
	ExpiresAt             time.Time `gorm:"index;not null" json:"expires_at"`
	Status                string `gorm:"size:32;index;not null;default:active" json:"status"` // active/disabled/exhausted/expired
	AllowedApps           string `gorm:"type:json" json:"allowed_apps"`
	DefaultCommissionRate float64 `gorm:"type:decimal(5,2);not null;default:10.00" json:"default_commission_rate"`
	CreatedBy             uint64 `gorm:"not null" json:"created_by"`
	CreatorType           string `gorm:"size:16;not null;default:tenant" json:"creator_type"`           // v0.4.0：tenant/agent（创建者类型）
	CreatorAgentID        uint64 `gorm:"not null;default:0" json:"creator_agent_id"`                   // v0.4.0：creator_type=agent 时填，否则 0
}

func (AgentInviteCode) TableName() string { return "agent_invite_code" }

// AgentBalanceLog 代理余额流水
type AgentBalanceLog struct {
	BaseModel
	AgentID         uint64   `gorm:"index;not null" json:"agent_id"`
	TenantID        uint64   `gorm:"index;not null" json:"tenant_id"`
	Type            string   `gorm:"size:32;not null" json:"type"` // recharge/deduct/commission/withdraw/adjust
	Amount          float64  `gorm:"type:decimal(12,2);not null" json:"amount"`
	BalanceAfter    float64  `gorm:"type:decimal(12,2);not null" json:"balance_after"`
	RelatedOrderID  *uint64  `gorm:"index" json:"related_order_id"`
	RelatedCardIDs  string   `gorm:"type:json" json:"related_card_ids"`
	PayMethod       string   `gorm:"size:32" json:"pay_method"`
	PayVoucher      string   `gorm:"size:255" json:"pay_voucher"`
	Status          string   `gorm:"size:32;index;not null;default:pending" json:"status"`
	Remark          string   `gorm:"size:255" json:"remark"`
}

func (AgentBalanceLog) TableName() string { return "agent_balance_log" }

// AgentWithdraw 代理提现
type AgentWithdraw struct {
	BaseModel
	AgentID      uint64     `gorm:"index;not null" json:"agent_id"`
	TenantID     uint64     `gorm:"index;not null" json:"tenant_id"`
	Amount       float64    `gorm:"type:decimal(12,2);not null" json:"amount"`
	PayMethod    string     `gorm:"size:32;not null" json:"pay_method"` // wechat/alipay/bank
	PayAccount   string     `gorm:"size:128;not null" json:"pay_account"`
	Status       string     `gorm:"size:32;index;not null;default:pending" json:"status"` // pending/approved/rejected/paid/failed
	AuditRemark string     `gorm:"size:255" json:"audit_remark"`
	PayTradeNo   string     `gorm:"size:128" json:"pay_trade_no"`
	PaidAt       *time.Time `json:"paid_at"`
	AuditedBy    *uint64    `json:"audited_by"`
}

func (AgentWithdraw) TableName() string { return "agent_withdraw" }

// AgentCommission 代理佣金
type AgentCommission struct {
	BaseModel
	AgentID       uint64   `gorm:"index;not null" json:"agent_id"`
	TenantID      uint64   `gorm:"index;not null" json:"tenant_id"`
	OrderID       uint64   `gorm:"index;not null" json:"order_id"`
	CardID        *uint64  `gorm:"index" json:"card_id"`
	SaleAmount    float64  `gorm:"type:decimal(12,2);not null" json:"sale_amount"`
	CommissionRate float64 `gorm:"type:decimal(5,2);not null" json:"commission_rate"`
	Amount        float64  `gorm:"type:decimal(12,2);not null" json:"amount"`
	SettleStatus  string   `gorm:"size:32;index;not null;default:pending" json:"settle_status"` // pending/settled/rejected
	SettledAt     *time.Time `json:"settled_at"`
	SettleMethod  string   `gorm:"size:32" json:"settle_method"`
	SettleRemark  string   `gorm:"size:255" json:"settle_remark"`
}

func (AgentCommission) TableName() string { return "agent_commission" }

// AgentRegistrationOrder 代理注册订单
type AgentRegistrationOrder struct {
	BaseModel
	OrderNo       string     `gorm:"uniqueIndex;size:64;not null" json:"order_no"`
	InviteCodeID  uint64     `gorm:"index;not null" json:"invite_code_id"`
	TenantID      uint64     `gorm:"index;not null" json:"tenant_id"`
	AgentID       *uint64    `gorm:"index" json:"agent_id"`
	Username      string     `gorm:"size:64;not null" json:"username"`
	Phone         string     `gorm:"size:32" json:"phone"`
	Amount        float64    `gorm:"type:decimal(10,2);not null" json:"amount"`
	PayChannel    string     `gorm:"size:32;not null" json:"pay_channel"`
	PayStatus     string     `gorm:"size:32;index;not null;default:pending" json:"pay_status"`
	PayTradeNo    string     `gorm:"size:128" json:"pay_trade_no"`
	PaidAt        *time.Time `json:"paid_at"`
	ClientIP      string     `gorm:"size:45" json:"client_ip"`
	// v0.4.x S-17：超管退款字段（refund_status=none 时未退款）
	RefundStatus string     `gorm:"size:16;index;not null;default:none" json:"refund_status"` // none/refunded
	RefundAmount float64    `gorm:"type:decimal(10,2);not null;default:0" json:"refund_amount"`
	RefundAt     *time.Time `json:"refund_at"`
	RefundBy     uint64     `gorm:"not null;default:0" json:"refund_by"`
	RefundReason string     `gorm:"size:255;not null;default:''" json:"refund_reason"`
}

func (AgentRegistrationOrder) TableName() string { return "agent_registration_order" }

// ============== v0.4.x 开发者安全配置（D-15） ==============

// TenantSecurityConfig 开发者安全配置（IP 黑名单 + 验证 API 频率限制）
type TenantSecurityConfig struct {
	ID                    uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID              uint64    `gorm:"uniqueIndex:uk_tenant_id;not null" json:"tenant_id"`
	IPBlacklist           string    `gorm:"type:text;not null" json:"ip_blacklist"`                  // JSON 数组：["1.2.3.4","10.0.0.0/8"]
	VerifyRateLimitPerMin int       `gorm:"not null;default:0" json:"verify_rate_limit_per_min"`     // 客户端验证 API 限速（每分钟，0=不限）
	LoginRateLimitPerMin  int       `gorm:"not null;default:0" json:"login_rate_limit_per_min"`     // 客户端登录 API 限速（每分钟，0=不限）
	UpdatedAt             time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (TenantSecurityConfig) TableName() string { return "tenant_security_config" }

// ============== v0.4.x 开发者月费订单 ==============

// TenantMonthlyFeeOrder 开发者月度服务费订单
// 订单号前缀 MFD，与 ORD/TOP/REG 区分，dispatchPaidOrder 按 prefix 分发
type TenantMonthlyFeeOrder struct {
	BaseModel
	TenantID    uint64     `gorm:"index;not null" json:"tenant_id"`
	PeriodStart time.Time  `gorm:"index:idx_period;not null" json:"period_start"`
	PeriodEnd   time.Time  `gorm:"index:idx_period;not null" json:"period_end"`
	Amount      float64    `gorm:"type:decimal(10,2);not null" json:"amount"`
	PayStatus   string     `gorm:"size:16;index;not null;default:pending" json:"pay_status"` // pending/paid/closed
	PayMode     string     `gorm:"size:32;not null;default:''" json:"pay_mode"`             // platform_epay/manual
	OrderNo     string     `gorm:"uniqueIndex;size:64;not null" json:"order_no"`            // MFD 前缀
	PaidAt      *time.Time `json:"paid_at"`
}

func (TenantMonthlyFeeOrder) TableName() string { return "tenant_monthly_fee_order" }

// ============== 公告层 ==============

// Notice 统一公告表
type Notice struct {
	BaseModel
	Type          string     `gorm:"size:32;index;not null" json:"type"` // platform/developer/app/agent_notify
	TenantID      *uint64    `gorm:"index" json:"tenant_id"`
	AppID         *uint64    `gorm:"index" json:"app_id"`
	Title         string     `gorm:"size:255;not null" json:"title"`
	Content       string     `gorm:"type:text;not null" json:"content"`
	ContentFormat string     `gorm:"size:16;not null;default:text" json:"content_format"` // v0.4.0：text=纯文本 / html=富文本
	IsPinned      bool       `gorm:"not null;default:false" json:"is_pinned"`
	IsPopup       bool       `gorm:"not null;default:false" json:"is_popup"`
	ShowBadge     bool       `gorm:"not null;default:true" json:"show_badge"`
	StartAt       time.Time  `gorm:"index;not null" json:"start_at"`
	EndAt         *time.Time `gorm:"index" json:"end_at"`
	Status        string     `gorm:"size:32;index;not null;default:draft" json:"status"` // draft/published/offline
	ViewCount     int        `gorm:"not null;default:0" json:"view_count"`
	Sort          int        `gorm:"not null;default:0" json:"sort"` // 排序权重，越大越靠前
	CreatedBy     uint64     `gorm:"not null" json:"created_by"`
}

func (Notice) TableName() string { return "notice" }

// NoticeTarget 公告精准投递
type NoticeTarget struct {
	BaseModel
	NoticeID   uint64 `gorm:"index;not null" json:"notice_id"`
	TargetType string `gorm:"size:32;not null" json:"target_type"` // all_tenants/all_agents/specific_tenant/specific_agent/specific_app
	TargetID   *uint64 `gorm:"index" json:"target_id"`
}

func (NoticeTarget) TableName() string { return "notice_target" }

// NoticeRead 公告已读记录
type NoticeRead struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	NoticeID  uint64    `gorm:"uniqueIndex:uk_notice_user;not null" json:"notice_id"`
	UserType  string    `gorm:"uniqueIndex:uk_notice_user;size:32;not null" json:"user_type"` // tenant/agent/admin/end_user
	UserID    uint64    `gorm:"uniqueIndex:uk_notice_user;not null" json:"user_id"`
	ReadAt    time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"read_at"`
}

func (NoticeRead) TableName() string { return "notice_read" }

// ============== 安全 / 日志 ==============

// SecIPBlacklist IP 黑名单
type SecIPBlacklist struct {
	BaseModel
	IP            string `gorm:"size:45;not null" json:"ip"`
	Reason        string `gorm:"size:255" json:"reason"`
	Source        string `gorm:"size:32;not null;default:manual" json:"source"` // manual/auto
	CreatedBy     *uint64 `json:"created_by"`
	CreatedByType string `gorm:"size:32" json:"created_by_type"` // admin/tenant/auto
	ExpiresAt     *time.Time `gorm:"index" json:"expires_at"`
}

func (SecIPBlacklist) TableName() string { return "sec_ip_blacklist" }

// LogVerify 验证日志
type LogVerify struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID   uint64    `gorm:"index;not null" json:"tenant_id"`
	AppID      uint64    `gorm:"index;not null" json:"app_id"`
	CardID     *uint64   `gorm:"index" json:"card_id"`
	DeviceID   *uint64   `gorm:"index" json:"device_id"`
	Action     string    `gorm:"size:32;index;not null" json:"action"` // login/verify/heartbeat/bind/unbind/getvar/notice/version
	Result     string    `gorm:"size:32;not null" json:"result"`       // success/fail/banned/expired/device_mismatch/rate_limited
	ClientIP   string    `gorm:"size:45" json:"client_ip"`
	UserAgent  string    `gorm:"size:255" json:"user_agent"`
	Extra      string    `gorm:"type:json" json:"extra"`
	CreatedAt  time.Time `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (LogVerify) TableName() string { return "log_verify" }

// LogOperation 操作日志
type LogOperation struct {
	BaseModel
	OperatorType string `gorm:"size:32;not null" json:"operator_type"` // admin/tenant/agent
	OperatorID   uint64 `gorm:"not null" json:"operator_id"`
	Username     string `gorm:"size:64" json:"username"` // 操作者用户名冗余
	OperatorIP   string `gorm:"size:45" json:"operator_ip"`
	UserAgent    string `gorm:"size:255" json:"user_agent"`
	Module       string `gorm:"size:64;index" json:"module"`
	Action       string `gorm:"size:64;not null" json:"action"`
	Status       string `gorm:"size:32;not null;default:success" json:"status"` // success/fail
	TargetType   string `gorm:"size:64" json:"target_type"`
	TargetID     *uint64 `gorm:"index" json:"target_id"`
	Detail       string `gorm:"type:json" json:"detail"`
}

func (LogOperation) TableName() string { return "log_operation" }

// ============== 平台结算 ==============

// PlatformSettlement 平台抽成结算记录
// 每笔走平台总支付的订单在支付成功后写入一条记录，用于后续开发者结算
type PlatformSettlement struct {
	BaseModel
	TenantID          uint64     `gorm:"index:idx_tenant_status;not null" json:"tenant_id"`
	OrderID           uint64     `gorm:"uniqueIndex:uk_order;not null" json:"order_id"`
	OrderNo           string     `gorm:"size:64;not null" json:"order_no"`
	GrossAmount       float64    `gorm:"type:decimal(10,2);not null" json:"gross_amount"`            // 订单总额
	CommissionRate    float64    `gorm:"type:decimal(5,2);not null" json:"commission_rate"`          // 抽成比例 %
	CommissionAmount  float64    `gorm:"type:decimal(10,2);not null" json:"commission_amount"`       // 平台抽成
	NetAmount         float64    `gorm:"type:decimal(10,2);not null" json:"net_amount"`              // 开发者应得
	Status            string     `gorm:"size:32;index:idx_tenant_status;not null;default:pending" json:"status"` // pending/settled/rejected
	SettledAt         *time.Time `json:"settled_at"`
	SettleBatchNo     string     `gorm:"size:64" json:"settle_batch_no"`
	SettleMethod      string     `gorm:"size:32" json:"settle_method"` // manual/alipay/wechat/bank
	SettleRemark      string     `gorm:"size:255" json:"settle_remark"`
}

func (PlatformSettlement) TableName() string { return "platform_settlement" }

// TenantBalanceLog 开发者余额流水（v0.3.4 新增）
// type: settle（结算入账，正）/ withdraw（提现扣款，负）/ refund（提现驳回退回，正）/ adjust（人工调整）
// status: pending / settled / rejected
type TenantBalanceLog struct {
	BaseModel
	TenantID             uint64   `gorm:"index;not null" json:"tenant_id"`
	Type                 string   `gorm:"size:32;not null" json:"type"`
	Amount               float64  `gorm:"type:decimal(12,2);not null" json:"amount"`
	BalanceAfter         float64  `gorm:"type:decimal(12,2);not null" json:"balance_after"`
	RelatedOrderID       *uint64  `gorm:"index" json:"related_order_id"`
	RelatedSettlementID  *uint64  `gorm:"index" json:"related_settlement_id"`
	RelatedWithdrawID    *uint64  `gorm:"index" json:"related_withdraw_id"`
	PayMethod            string   `gorm:"size:32" json:"pay_method"`
	SettleBatchNo        string   `gorm:"size:64" json:"settle_batch_no"`
	Status               string   `gorm:"size:32;index;not null;default:pending" json:"status"`
	Remark               string   `gorm:"size:255" json:"remark"`
}

func (TenantBalanceLog) TableName() string { return "tenant_balance_log" }

// TenantWithdraw 开发者提现申请（v0.3.4 新增）
// status: pending（待审核）/ paid（已打款）/ rejected（已驳回）/ failed（打款失败）
type TenantWithdraw struct {
	BaseModel
	TenantID    uint64     `gorm:"index;not null" json:"tenant_id"`
	Amount      float64    `gorm:"type:decimal(12,2);not null" json:"amount"`
	PayMethod   string     `gorm:"size:32;not null" json:"pay_method"` // wechat/alipay/bank
	PayAccount  string     `gorm:"size:128;not null" json:"pay_account"`
	Status      string     `gorm:"size:32;index;not null;default:pending" json:"status"`
	AuditRemark string     `gorm:"size:255" json:"audit_remark"`
	PayTradeNo  string     `gorm:"size:128" json:"pay_trade_no"`
	PaidAt      *time.Time `json:"paid_at"`
	AuditedBy   *uint64    `json:"audited_by"`
}

func (TenantWithdraw) TableName() string { return "tenant_withdraw" }

// ============== v0.3.1 新增 ==============

// LogLoginFailed 登录失败日志（用于安全中心统计）
type LogLoginFailed struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserType  string    `gorm:"size:32;not null" json:"user_type"` // admin/tenant/agent
	Username  string    `gorm:"size:64;not null" json:"username"`
	ClientIP  string    `gorm:"size:45;not null" json:"client_ip"`
	Reason    string    `gorm:"size:64;not null" json:"reason"` // wrong_password/disabled/locked/unknown
	UserAgent string    `gorm:"size:255" json:"user_agent"`
	CreatedAt time.Time `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (LogLoginFailed) TableName() string { return "log_login_failed" }

// RefreshTokenDevice refresh token 设备会话（用于 ListLoginDevices / KickDevice）
type RefreshTokenDevice struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserRole     string     `gorm:"size:32;not null" json:"user_role"` // admin/tenant/agent
	UserID       uint64     `gorm:"not null" json:"user_id"`
	RefreshJTI   string     `gorm:"uniqueIndex;size:64;not null" json:"refresh_jti"`
	DeviceName   string     `gorm:"size:128" json:"device_name"`
	DeviceType   string     `gorm:"size:32" json:"device_type"` // pc/mobile/tablet
	ClientIP     string     `gorm:"size:45" json:"client_ip"`
	UserAgent    string     `gorm:"size:512" json:"user_agent"`
	LastActiveAt time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"last_active_at"`
	ExpiresAt    time.Time  `gorm:"index;not null" json:"expires_at"`
	Revoked      bool       `gorm:"not null;default:false" json:"revoked"`
	RevokedAt    *time.Time `json:"revoked_at"`
	CreatedAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (RefreshTokenDevice) TableName() string { return "refresh_token_device" }

// ============== v0.4.0 在线更新 ==============

// SystemUpdateLog 在线更新审计日志（v0.4.0）
type SystemUpdateLog struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TriggerSource   string    `gorm:"size:32;not null;default:manual" json:"trigger_source"` // webhook / manual / rollback
	TriggerBy       uint64    `gorm:"not null;default:0" json:"trigger_by"`                  // 触发者 admin id（webhook 时为 0）
	TriggerIP       string    `gorm:"size:45;not null;default:''" json:"trigger_ip"`
	CommitBefore    string    `gorm:"size:64;not null;default:''" json:"commit_before"`
	CommitAfter     string    `gorm:"size:64;not null;default:''" json:"commit_after"`
	Branch          string    `gorm:"size:64;not null;default:''" json:"branch"`
	Status          string    `gorm:"size:32;not null;default:pending" json:"status"` // pending / running / success / failed / rolled_back
	StepsJSON       string    `gorm:"type:text" json:"steps_json"`                    // [{step,status,duration_ms,error}]
	LogText         string    `gorm:"type:mediumtext" json:"log_text"`
	ErrorMessage    string    `gorm:"size:512;not null;default:''" json:"error_message"`
	DurationMs      int       `gorm:"not null;default:0" json:"duration_ms"`
	RolledBackFrom  uint64    `gorm:"not null;default:0" json:"rolled_back_from"` // 若为回滚，原失败更新 id（0=非回滚）
	CreatedAt       time.Time `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (SystemUpdateLog) TableName() string { return "system_update_log" }

// ============== v0.4.0 数据备份恢复 ==============

// SystemBackupLog 备份审计日志（v0.4.0）
type SystemBackupLog struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	BackupType    string    `gorm:"size:32;not null;default:manual" json:"backup_type"` // manual / auto / restore_source
	TriggerBy     uint64    `gorm:"not null;default:0" json:"trigger_by"`
	TriggerIP     string    `gorm:"size:45;not null;default:''" json:"trigger_ip"`
	FilePath      string    `gorm:"size:512;not null;default:''" json:"file_path"`
	FileSize      int64     `gorm:"not null;default:0" json:"file_size"`
	Checksum      string    `gorm:"size:64;not null;default:''" json:"checksum"`
	Status        string    `gorm:"size:32;not null;default:pending" json:"status"` // pending / running / success / failed / deleted
	ErrorMessage  string    `gorm:"size:512;not null;default:''" json:"error_message"`
	DurationMs    int       `gorm:"not null;default:0" json:"duration_ms"`
	TablesCount   int       `gorm:"not null;default:0" json:"tables_count"`
	RowsCount     int64     `gorm:"not null;default:0" json:"rows_count"`
	RestoredFrom  uint64    `gorm:"not null;default:0" json:"restored_from"`
	CreatedAt     time.Time `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (SystemBackupLog) TableName() string { return "system_backup_log" }

// ============== v0.4.0 监控告警 ==============

// SystemMetric 系统指标时序数据（v0.4.0）
type SystemMetric struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	MetricName  string    `gorm:"size:64;not null" json:"metric_name"` // cpu_usage/memory_usage/disk_usage/qps/verify_count/online_devices/error_rate
	MetricValue float64   `gorm:"not null" json:"metric_value"`
	MetricUnit  string    `gorm:"size:16;not null;default:''" json:"metric_unit"` // %/count/ratio/mb
	LabelsJSON  string    `gorm:"size:512;not null;default:'{}'" json:"labels_json"`
	CollectedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"collected_at"`
}

func (SystemMetric) TableName() string { return "system_metric" }

// SystemAlert 告警事件（v0.4.0）
type SystemAlert struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AlertRule   string     `gorm:"size:64;not null" json:"alert_rule"` // 与 metric_name 对应
	Severity    string     `gorm:"size:16;not null;default:warning" json:"severity"` // info/warning/critical/fatal
	Status      string     `gorm:"size:16;not null;default:firing" json:"status"` // firing/resolved/silenced/acked
	MetricValue float64    `gorm:"not null" json:"metric_value"`
	Threshold   float64    `gorm:"not null" json:"threshold"`
	Operator    string     `gorm:"size:8;not null;default:'>'" json:"operator"` // > / < / >= / <= / ==
	Message     string     `gorm:"size:512;not null;default:''" json:"message"`
	LabelsJSON  string     `gorm:"size:512;not null;default:'{}'" json:"labels_json"`
	FiredAt     time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"fired_at"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	AckedBy     uint64     `gorm:"not null;default:0" json:"acked_by"`
	AckedAt     *time.Time `json:"acked_at"`
	NotifySent  int        `gorm:"not null;default:0" json:"notify_sent"` // 0=未发送 1=已发送
	CreatedAt   time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (SystemAlert) TableName() string { return "system_alert" }

// ============== v0.4.0 通知系统 ==============

// NotifyTemplate 通知模板（v0.4.0）
type NotifyTemplate struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Code      string    `gorm:"size:64;not null" json:"code"`                       // verify_code / order_paid / agent_commission
	Name      string    `gorm:"size:128;not null" json:"name"`                      // 模板名称
	Channel   string    `gorm:"size:16;not null" json:"channel"`                    // sms / email / inapp
	Subject   string    `gorm:"size:255;not null;default:''" json:"subject"`         // 标题（email 用）
	Content   string    `gorm:"type:text;not null" json:"content"`                   // 模板内容，{{var}} 占位符
	Variables string    `gorm:"size:512;not null;default:'[]'" json:"variables"`      // 变量列表 JSON
	TenantID  uint64    `gorm:"not null;default:0" json:"tenant_id"`                 // 0=平台通用模板
	Status    string    `gorm:"size:16;not null;default:enabled" json:"status"`      // enabled / disabled
	Remark    string    `gorm:"size:255;not null;default:''" json:"remark"`
	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (NotifyTemplate) TableName() string { return "notify_template" }

// NotifyLog 通知发送日志（v0.4.0）
type NotifyLog struct {
	ID            uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateID    uint64     `gorm:"not null;default:0" json:"template_id"`
	TemplateCode  string     `gorm:"size:64;not null;default:''" json:"template_code"`
	Channel       string     `gorm:"size:16;not null" json:"channel"`
	Recipient     string     `gorm:"size:255;not null" json:"recipient"`
	Subject       string     `gorm:"size:255;not null;default:''" json:"subject"`
	Content       string     `gorm:"type:text;not null" json:"content"`
	Status        string     `gorm:"size:16;not null;default:pending" json:"status"` // pending / sent / failed
	ProviderMsgID string     `gorm:"size:128;not null;default:''" json:"provider_msgid"`
	ErrorMessage  string     `gorm:"size:512;not null;default:''" json:"error_message"`
	RetryCount    int        `gorm:"not null;default:0" json:"retry_count"`
	Priority      int        `gorm:"not null;default:0" json:"priority"` // 0=普通 1=高 2=紧急
	TenantID      uint64     `gorm:"not null;default:0" json:"tenant_id"`
	SentAt        *time.Time `json:"sent_at"`
	CreatedAt     time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (NotifyLog) TableName() string { return "notify_log" }

// ============== v0.4.0 终端用户体系 ==============

// EndUser 终端用户（v0.4.0）
type EndUser struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID     uint64     `gorm:"index;not null" json:"tenant_id"`
	AppID        uint64     `gorm:"index;not null" json:"app_id"`
	Username     string     `gorm:"size:64;not null" json:"username"`
	Phone        string     `gorm:"size:32;not null;default:''" json:"phone"`
	Email        string     `gorm:"size:128;not null;default:''" json:"email"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"` // bcrypt(cost=12)
	Nickname     string     `gorm:"size:64;not null;default:''" json:"nickname"`
	AvatarURL    string     `gorm:"size:512;not null;default:''" json:"avatar_url"`
	Status       string     `gorm:"size:16;not null;default:active" json:"status"` // active/banned/deleted
	LastLoginAt  *time.Time `json:"last_login_at"`
	LastLoginIP  string     `gorm:"size:64;not null;default:''" json:"last_login_ip"`
	LastLoginUA  string     `gorm:"size:512;not null;default:''" json:"last_login_ua"`
	LoginCount   int        `gorm:"not null;default:0" json:"login_count"`
	Remark       string     `gorm:"size:255;not null;default:''" json:"remark"`
	CreatedAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (EndUser) TableName() string { return "end_user" }

// EndUserCard 终端用户-卡密绑定关系（v0.4.0）
type EndUserCard struct {
	ID        uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64     `gorm:"index;not null" json:"user_id"`
	CardID    uint64     `gorm:"uniqueIndex;not null" json:"card_id"` // 一张卡只能绑一个用户
	TenantID  uint64     `gorm:"index;not null" json:"tenant_id"`
	AppID     uint64     `gorm:"index;not null" json:"app_id"`
	BoundAt   time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"bound_at"`
	UnboundAt *time.Time `json:"unbound_at"`
	Status    string     `gorm:"size:16;not null;default:active" json:"status"` // active / unbound
	CreatedAt time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (EndUserCard) TableName() string { return "end_user_card" }

// EndUserToken 终端用户 Refresh Token（v0.4.0，jti 单点踢出兼容）
type EndUserToken struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint64     `gorm:"index;not null" json:"user_id"`
	JTI          string     `gorm:"uniqueIndex;size:64;not null" json:"jti"`
	DeviceName   string     `gorm:"size:128;not null;default:''" json:"device_name"`
	DeviceType   string     `gorm:"size:16;not null;default:''" json:"device_type"`
	IP           string     `gorm:"size:64;not null;default:''" json:"ip"`
	UserAgent    string     `gorm:"size:512;not null;default:''" json:"user_agent"`
	RefreshToken string     `gorm:"size:255;not null" json:"-"` // SHA-512 哈希
	ExpiresAt    time.Time  `gorm:"index;not null" json:"expires_at"`
	RevokedAt    *time.Time `gorm:"index" json:"revoked_at"`
	CreatedAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (EndUserToken) TableName() string { return "end_user_token" }

// ============== v0.4.0 API 开放平台 ==============

// DeveloperAPIToken 开发者 API Token（v0.4.0，SHA-512 哈希存储 + scopes 权限）
type DeveloperAPIToken struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID    uint64     `gorm:"index;not null" json:"tenant_id"`
	Name        string     `gorm:"size:64;not null" json:"name"`
	TokenHash   string     `gorm:"uniqueIndex;size:128;not null" json:"-"` // SHA-512 哈希（不存明文）
	Prefix      string     `gorm:"size:16;not null" json:"prefix"`        // 前 8 位明文（用于展示识别）
	Scopes      string     `gorm:"size:512;not null;default:''" json:"scopes"`
	ExpiresAt   *time.Time `gorm:"index" json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	LastUsedIP  string     `gorm:"size:64;not null;default:''" json:"last_used_ip"`
	Status      string     `gorm:"index;size:16;not null;default:active" json:"status"` // active / revoked
	RevokedAt   *time.Time `json:"revoked_at"`
	CreatedAt   time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (DeveloperAPIToken) TableName() string { return "developer_api_token" }

// WebhookEndpoint Webhook 推送端点（v0.4.0）
type WebhookEndpoint struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID         uint64     `gorm:"index;not null" json:"tenant_id"`
	Name             string     `gorm:"size:64;not null" json:"name"`
	URL              string     `gorm:"size:512;not null" json:"url"`
	SecretEnc        string     `gorm:"size:512;not null;default:''" json:"-"` // AES-256-GCM 加密存储
	Events           string     `gorm:"size:512;not null;default:''" json:"events"`
	Status           string     `gorm:"index;size:16;not null;default:active" json:"status"` // active / disabled
	FailureCount     int        `gorm:"not null;default:0" json:"failure_count"`
	LastResponseCode int        `gorm:"not null;default:0" json:"last_response_code"`
	LastResponseAt   *time.Time `json:"last_response_at"`
	LastError        string     `gorm:"size:512;not null;default:''" json:"last_error"`
	CreatedAt        time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (WebhookEndpoint) TableName() string { return "webhook_endpoint" }

// WebhookDelivery Webhook 推送日志（v0.4.0）
type WebhookDelivery struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID     uint64     `gorm:"index;not null" json:"tenant_id"`
	EndpointID   uint64     `gorm:"index;not null" json:"endpoint_id"`
	EventType    string     `gorm:"index;size:64;not null" json:"event_type"`
	EventID      string     `gorm:"size:64;not null" json:"event_id"` // UUID，防重放
	Payload      string     `gorm:"type:text;not null" json:"payload"`
	Status       string     `gorm:"index;size:16;not null;default:pending" json:"status"` // pending / success / failed
	ResponseCode int        `gorm:"not null;default:0" json:"response_code"`
	ResponseBody string     `gorm:"size:1024;not null;default:''" json:"response_body"`
	AttemptCount int        `gorm:"not null;default:0" json:"attempt_count"`
	MaxRetry     int        `gorm:"not null;default:3" json:"max_retry"`
	NextRetryAt  *time.Time `gorm:"index" json:"next_retry_at"`
	DeliveredAt  *time.Time `json:"delivered_at"`
	CreatedAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (WebhookDelivery) TableName() string { return "webhook_delivery" }

// ============== v0.4.0 高级安全（风控规则引擎 + 异地登录告警） ==============

// RiskRule 风控规则配置表
// - 内置规则由 system 创建（rule_type 固定，仅可调阈值/启停）
// - 自定义规则由管理员创建（rule_type=custom，condition 为 JSON 表达式）
type RiskRule struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:64;not null" json:"name"`
	Description string    `gorm:"size:255;not null;default:''" json:"description"`
	RuleType    string    `gorm:"size:32;index;not null" json:"rule_type"` // geo_login/new_device/abnormal_ua/abnormal_time/high_frequency/custom
	Condition   string    `gorm:"type:text;not null" json:"condition"`     // JSON 条件
	Score       int       `gorm:"not null;default:0" json:"score"`         // 命中加分 0-100
	Action      string    `gorm:"size:32;not null;default:alert" json:"action"` // alert/challenge/block
	Priority    int       `gorm:"index;not null;default:100" json:"priority"`
	Status      string    `gorm:"size:32;not null;default:active" json:"status"` // active/disabled
	CreatedBy   string    `gorm:"size:64;not null;default:system" json:"created_by"`
	CreatedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (RiskRule) TableName() string { return "risk_rule" }

// RiskEvent 风控事件审计表
type RiskEvent struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RuleID         uint64     `gorm:"not null;default:0" json:"rule_id"` // 0=内置规则
	RuleType       string     `gorm:"size:32;index;not null" json:"rule_type"`
	RuleName       string     `gorm:"size:64;not null" json:"rule_name"` // 快照
	UserType       string     `gorm:"size:32;not null;default:''" json:"user_type"` // admin/tenant/agent/enduser
	UserID         uint64     `gorm:"not null;default:0" json:"user_id"`
	Username       string     `gorm:"size:64;not null;default:''" json:"username"`
	ClientIP       string     `gorm:"size:45;index;not null;default:''" json:"client_ip"`
	UserAgent      string     `gorm:"size:512;not null;default:''" json:"user_agent"`
	RiskScore      int        `gorm:"not null;default:0" json:"risk_score"`
	ActionTaken    string     `gorm:"size:32;index;not null;default:alert" json:"action_taken"` // alert/challenge/block
	Detail         string     `gorm:"type:text;not null" json:"detail"`                         // JSON 详情
	Acknowledged   bool       `gorm:"index;not null;default:false" json:"acknowledged"`
	AcknowledgedBy string     `gorm:"size:64;not null;default:''" json:"acknowledged_by"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	CreatedAt      time.Time  `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (RiskEvent) TableName() string { return "risk_event" }

// LoginGeoAlert 异地登录告警表
// 触发：登录 IP 网段（IPv4 /24 或 IPv6 /64）与上次登录不同
type LoginGeoAlert struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserType         string     `gorm:"size:32;index;not null" json:"user_type"` // admin/tenant/agent/enduser
	UserID           uint64     `gorm:"not null" json:"user_id"`
	Username         string     `gorm:"size:64;not null" json:"username"`
	CurrentIP        string     `gorm:"size:45;not null" json:"current_ip"`
	CurrentNetwork   string     `gorm:"size:64;not null" json:"current_network"` // 如 1.2.3.0/24
	PreviousIP       string     `gorm:"size:45;not null" json:"previous_ip"`
	PreviousNetwork  string     `gorm:"size:64;not null" json:"previous_network"`
	UserAgent        string     `gorm:"size:512;not null;default:''" json:"user_agent"`
	AlertStatus      string     `gorm:"size:32;index;not null;default:pending" json:"alert_status"` // pending/acknowledged/closed
	NotifyChannels   string     `gorm:"size:128;not null;default:''" json:"notify_channels"`         // 逗号分隔：inapp,email,sms
	AcknowledgedBy   string     `gorm:"size:64;not null;default:''" json:"acknowledged_by"`
	AcknowledgedAt   *time.Time `json:"acknowledged_at"`
	ClosedAt         *time.Time `json:"closed_at"`
	CreatedAt        time.Time  `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (LoginGeoAlert) TableName() string { return "login_geo_alert" }

// ============== v0.6.0 高级分析（用户行为 / 卡密画像 / 风险评分） ==============

// UserBehaviorProfile 终端用户行为画像（按日聚合）
// 数据源：log_verify 表按 (end_user_id, stat_date) 聚合
// 唯一索引：(end_user_id, stat_date)
type UserBehaviorProfile struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID        uint64    `gorm:"index;not null" json:"tenant_id"`
	AppID           uint64    `gorm:"index;not null" json:"app_id"`
	EndUserID       uint64    `gorm:"index;not null" json:"end_user_id"`
	StatDate        string    `gorm:"size:10;index;not null" json:"stat_date"` // YYYY-MM-DD
	LoginCount      int       `gorm:"not null;default:0" json:"login_count"`
	VerifyCount     int       `gorm:"not null;default:0" json:"verify_count"`
	HeartbeatCount  int       `gorm:"not null;default:0" json:"heartbeat_count"`
	BindCount       int       `gorm:"not null;default:0" json:"bind_count"`
	UnbindCount     int       `gorm:"not null;default:0" json:"unbind_count"`
	SuccessCount    int       `gorm:"not null;default:0" json:"success_count"`
	FailCount       int       `gorm:"not null;default:0" json:"fail_count"`
	BannedCount     int       `gorm:"not null;default:0" json:"banned_count"`
	DistinctIPCount int       `gorm:"not null;default:0" json:"distinct_ip_count"`
	DistinctDevCount int      `gorm:"column:distinct_device_count;not null;default:0" json:"distinct_device_count"`
	FirstActiveAt   *time.Time `json:"first_active_at"`
	LastActiveAt    *time.Time `json:"last_active_at"`
	CreatedAt       time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (UserBehaviorProfile) TableName() string { return "user_behavior_profile" }

// CardUsageProfile 卡密使用画像（按日聚合）
// 数据源：log_verify 表按 (card_id, stat_date) 聚合
// 唯一索引：(card_id, stat_date)
type CardUsageProfile struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID         uint64    `gorm:"index;not null" json:"tenant_id"`
	AppID            uint64    `gorm:"index;not null" json:"app_id"`
	CardID           uint64    `gorm:"index;not null" json:"card_id"`
	StatDate         string    `gorm:"size:10;index;not null" json:"stat_date"` // YYYY-MM-DD
	VerifyCount      int       `gorm:"not null;default:0" json:"verify_count"`
	HeartbeatCount   int       `gorm:"not null;default:0" json:"heartbeat_count"`
	BindCount        int       `gorm:"not null;default:0" json:"bind_count"`
	SuccessCount     int       `gorm:"not null;default:0" json:"success_count"`
	FailCount        int       `gorm:"not null;default:0" json:"fail_count"`
	BannedCount      int       `gorm:"not null;default:0" json:"banned_count"`
	DeviceMismatchCount int    `gorm:"not null;default:0" json:"device_mismatch_count"`
	DistinctIPCount  int       `gorm:"not null;default:0" json:"distinct_ip_count"`
	DistinctDevCount int       `gorm:"column:distinct_device_count;not null;default:0" json:"distinct_device_count"`
	FirstActiveAt    *time.Time `json:"first_active_at"`
	LastActiveAt     *time.Time `json:"last_active_at"`
	CreatedAt        time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt        time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (CardUsageProfile) TableName() string { return "card_usage_profile" }

// UserRiskScore 用户风险评分累计表（实时更新）
// 数据源：risk_event 表 + log_verify 异常模式实时累计
// 唯一索引：(user_type, user_id)
type UserRiskScore struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID      uint64    `gorm:"index;not null;default:0" json:"tenant_id"`
	AppID         uint64    `gorm:"index;not null;default:0" json:"app_id"`
	UserType      string    `gorm:"size:32;index;not null" json:"user_type"` // admin/tenant/agent/enduser
	UserID        uint64    `gorm:"index;not null" json:"user_id"`
	Username      string    `gorm:"size:64;not null;default:''" json:"username"`
	RiskScore     int       `gorm:"not null;default:0" json:"risk_score"` // 累计评分（衰减后）
	RiskLevel     string    `gorm:"size:16;not null;default:low" json:"risk_level"` // low/medium/high/critical
	EventCount    int       `gorm:"not null;default:0" json:"event_count"`
	HighFreqHits  int       `gorm:"not null;default:0" json:"high_freq_hits"`
	GeoAnomalyHits int      `gorm:"not null;default:0" json:"geo_anomaly_hits"`
	NewDeviceHits int       `gorm:"not null;default:0" json:"new_device_hits"`
	AbnormalUAHits int      `gorm:"not null;default:0" json:"abnormal_ua_hits"`
	FailRateHigh  int       `gorm:"not null;default:0" json:"fail_rate_high_hits"` // 失败率超阈值次数
	MultiIPHits   int       `gorm:"not null;default:0" json:"multi_ip_hits"`      // 24h 内多 IP
	MultiDevHits  int       `gorm:"not null;default:0" json:"multi_dev_hits"`     // 24h 内多设备
	LastEventAt   *time.Time `json:"last_event_at"`
	LastEvalAt    *time.Time `json:"last_eval_at"` // 最近一次评分重算时间
	Banned        bool      `gorm:"index;not null;default:false" json:"banned"`
	BannedReason  string    `gorm:"size:255;not null;default:''" json:"banned_reason"`
	BannedAt      *time.Time `json:"banned_at"`
	CreatedAt     time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt     time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (UserRiskScore) TableName() string { return "user_risk_score" }
