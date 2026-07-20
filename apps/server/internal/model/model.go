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
	TenantID        uint64     `gorm:"index;not null" json:"tenant_id"`
	AppID           uint64     `gorm:"index;not null" json:"app_id"`
	CardID          uint64     `gorm:"index;not null" json:"card_id"`
	HWID            string     `gorm:"size:128;not null" json:"hwid"`
	HWIDRaw         string     `gorm:"type:text" json:"hwid_raw"`
	DeviceName      string     `gorm:"size:128" json:"device_name"`
	DeviceType      string     `gorm:"size:32" json:"device_type"`
	IPAddress       string     `gorm:"size:45" json:"ip_address"`
	Status          string     `gorm:"size:32;index;not null;default:active" json:"status"` // active/offline/banned/unbound
	LastHeartbeatAt *time.Time `gorm:"index" json:"last_heartbeat_at"`
	FirstBoundAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"first_bound_at"`
	UnboundAt       *time.Time `json:"unbound_at"`
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
	TenantID        uint64 `gorm:"index;not null" json:"tenant_id"`
	AppID           uint64 `gorm:"index;not null" json:"app_id"`
	Version         string `gorm:"size:32;not null" json:"version"`
	Channel         string `gorm:"size:32;not null;default:stable" json:"channel"` // stable/beta/dev
	MinVersion      string `gorm:"size:32;not null" json:"min_version"`
	DownloadURL     string `gorm:"size:255" json:"download_url"`
	BackupURL       string `gorm:"size:255" json:"backup_url"`
	ForceUpdate     bool   `gorm:"not null;default:false" json:"force_update"`
	UpdateContent   string `gorm:"type:text" json:"update_content"`
	Status          string `gorm:"size:32;not null;default:active" json:"status"`
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
}

func (AgentRegistrationOrder) TableName() string { return "agent_registration_order" }

// ============== 公告层 ==============

// Notice 统一公告表
type Notice struct {
	BaseModel
	Type      string     `gorm:"size:32;index;not null" json:"type"` // platform/developer/app/agent_notify
	TenantID  *uint64    `gorm:"index" json:"tenant_id"`
	AppID     *uint64    `gorm:"index" json:"app_id"`
	Title     string     `gorm:"size:255;not null" json:"title"`
	Content   string     `gorm:"type:text;not null" json:"content"`
	IsPinned  bool       `gorm:"not null;default:false" json:"is_pinned"`
	IsPopup   bool       `gorm:"not null;default:false" json:"is_popup"`
	ShowBadge bool       `gorm:"not null;default:true" json:"show_badge"`
	StartAt   time.Time  `gorm:"index;not null" json:"start_at"`
	EndAt     *time.Time `gorm:"index" json:"end_at"`
	Status    string     `gorm:"size:32;index;not null;default:draft" json:"status"` // draft/published/offline
	ViewCount int        `gorm:"not null;default:0" json:"view_count"`
	Sort      int        `gorm:"not null;default:0" json:"sort"` // 排序权重，越大越靠前
	CreatedBy uint64     `gorm:"not null" json:"created_by"`
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
