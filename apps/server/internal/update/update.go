// Package update v0.4.0 在线更新核心包
// 严格遵循铁律 04/05/06：
//
//	04 - 无硬编码：webhook 密钥 / 分支 / 自动开关 / 部署脚本路径 / 健康检查 URL 全部从 sys_config 读取
//	05 - 配置走后端：8 项 update.* 配置可通过后台「系统配置」实时调整
//	06 - 反幻觉：所有 shell 命令显式组合不拼接用户输入；测试覆盖正/负/锁/状态机/边界全场景
//
// 核心能力：
//  1. VerifyWebhookSignature - GitHub HMAC-SHA256 签名校验（X-Hub-Signature-256）
//  2. ParsePushEvent - 解析 GitHub push event payload（提取 ref / head_commit / sender / repository）
//  3. Manager.AcquireLock / ReleaseLock - Redis 互斥锁（防并发触发）
//  4. Manager.ExecuteUpdate - 执行更新流程（git pull + deploy script + health check + 回滚）
//  5. Manager.HealthCheck - 更新后健康检查
//  6. Manager.Rollback - 失败回滚（git reset --hard <prev_commit> + 重跑脚本）
package update

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/keyauth-saas/apps/server/internal/config"
	"github.com/your-org/keyauth-saas/apps/server/internal/model"
	"gorm.io/gorm"
)

// ============== 常量 ==============

// 配置键常量（铁律 04：禁止硬编码配置键名）
const (
	CfgKeyWebhookSecret      = "update.webhook.secret"
	CfgKeyWebhookBranch      = "update.webhook.branch"
	CfgKeyAutoUpdate         = "update.webhook.auto_update"
	CfgKeyDeployScript       = "update.deploy.script_path"
	CfgKeyHealthCheckURL     = "update.healthcheck.url"
	CfgKeyHealthCheckTimeout = "update.healthcheck.timeout"
	CfgKeyRollbackEnabled    = "update.rollback.enabled"
	CfgKeyLockTimeout        = "update.lock.timeout"
	CfgKeyPollEnabled        = "update.poll.enabled"          // v0.4.0 弹窗通知总开关
	CfgKeyPollInterval       = "update.poll.interval_seconds" // v0.4.0 弹窗通知轮询间隔（秒）
)

// PollIntervalMin 轮询间隔下限（秒），防止配置错误导致前端打爆后端
const PollIntervalMin = 10

// TriggerSource 触发源
const (
	TriggerSourceWebhook  = "webhook"
	TriggerSourceManual   = "manual"
	TriggerSourceRollback = "rollback"
)

// Status 状态
const (
	StatusPending    = "pending"
	StatusRunning    = "running"
	StatusSuccess    = "success"
	StatusFailed     = "failed"
	StatusRolledBack = "rolled_back"
)

// Step 状态
const (
	StepStatusSuccess = "success"
	StepStatusFailed  = "failed"
	StepStatusSkipped = "skipped"
)

// 步骤名常量
const (
	StepGitPull      = "git_pull"
	StepDeployScript = "deploy_script"
	StepHealthCheck  = "health_check"
	StepRollback     = "rollback"
)

// ============== 类型 ==============

// PushEvent GitHub push event payload（仅提取关键字段）
// 完整字段见 https://docs.github.com/en/webhooks/webhook-events-and-payloads#push
type PushEvent struct {
	Ref        string `json:"ref"` // refs/heads/main
	Before     string `json:"before"`
	After      string `json:"after"`
	Repository struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
	HeadCommit struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		URL     string `json:"url"`
	} `json:"head_commit"`
}

// StepResult 单步执行结果
type StepResult struct {
	Step       string `json:"step"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}

// UpdateOptions 更新选项
type UpdateOptions struct {
	TriggerSource string // webhook / manual / rollback
	TriggerBy     uint64 // admin id
	TriggerIP     string
	Branch        string // 目标分支（webhook 来自 payload，manual 来自请求）
}

// UpdateResult 更新结果
type UpdateResult struct {
	LogID         uint64
	Status        string
	TriggerSource string
	Branch        string
	CommitBefore  string
	CommitAfter   string
	Steps         []StepResult
	DurationMs    int64
	ErrorMessage  string
	RolledBack    bool
}

// Manager 更新管理器（单例）
type Manager struct {
	db      *gorm.DB
	cache   *config.ConfigCache
	mu      sync.Mutex // 进程内互斥（Redis 锁之外的二次保险）
	lockKey string     // Redis 锁键
}

var (
	managerOnce sync.Once
	managerInst *Manager
)

// NewManager 创建或获取更新管理器单例
func NewManager(db *gorm.DB, cache *config.ConfigCache) *Manager {
	managerOnce.Do(func() {
		managerInst = &Manager{
			db:      db,
			cache:   cache,
			lockKey: "keyauth:update:lock",
		}
	})
	return managerInst
}

// ============== 1. Webhook 签名校验 ==============

// VerifyWebhookSignature 校验 GitHub X-Hub-Signature-256 头
// GitHub 算法：HMAC-SHA256(secret, body) → hex 编码 → 前缀 "sha256="
// 铁律 06：使用 hmac.Equal 防止时序攻击；空 secret 时跳过校验（仅用于本地开发）
func VerifyWebhookSignature(signature string, body []byte, secret string) bool {
	// 空 secret 时跳过校验（仅本地开发；生产必须配置）
	if secret == "" {
		return true
	}
	if signature == "" {
		return false
	}
	// 必须以 "sha256=" 前缀开头
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return false
	}
	provided := signature[len(prefix):]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(provided), []byte(expected))
}

// ============== 2. push event 解析 ==============

// ParsePushEvent 解析 push event JSON
// 铁律 06：严格 JSON 解析；非 push event 返回错误
func ParsePushEvent(body []byte) (*PushEvent, error) {
	var event PushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("解析 push event 失败: %w", err)
	}
	// ref 必填
	if event.Ref == "" {
		return nil, fmt.Errorf("invalid push event: ref 为空")
	}
	return &event, nil
}

// BranchMatches 判断 push event 的 ref 是否匹配目标分支
// ref 格式：refs/heads/<branch>；允许传 "main" 或 "refs/heads/main"
func BranchMatches(ref, branch string) bool {
	if branch == "" {
		return false
	}
	// 规范化 branch 为 refs/heads/<branch> 形式
	expected := branch
	if !strings.HasPrefix(expected, "refs/heads/") {
		expected = "refs/heads/" + expected
	}
	return ref == expected
}

// ============== 3. Redis 互斥锁 ==============

// releaseLockScript 释放锁的 Lua 脚本：仅当 Redis 中锁值等于 token 时才删除
// 铁律 06：避免多实例部署下实例 A 锁过期后实例 B 抢锁、A 完成后误删 B 的锁
const releaseLockScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
`

// AcquireLock 获取更新互斥锁（防止并发触发）
// 铁律 06：使用 SET NX EX 模式原子加锁；锁值为 UUID token，释放时通过 Lua 脚本原子比较并删除
// 返回 token（成功时）与 ok；调用方需保存 token 并在 ReleaseLock 时传入
func (m *Manager) AcquireLock(ctx context.Context) (string, bool) {
	// 进程内互斥（双重保险）
	if !m.mu.TryLock() {
		return "", false
	}
	// Redis 分布式锁：value 使用 UUID token，便于释放时校验归属
	lockTimeout := m.cache.GetInt(ctx, CfgKeyLockTimeout, 600)
	token := uuid.NewString()
	ok, err := m.cache.RedisClient().SetNX(ctx, m.lockKey, token, time.Duration(lockTimeout)*time.Second).Result()
	if err != nil {
		m.mu.Unlock()
		return "", false
	}
	if !ok {
		m.mu.Unlock()
		return "", false
	}
	return token, true
}

// ReleaseLock 释放更新互斥锁
// 铁律 06：使用 Lua 脚本原子比较 token 后删除，避免误删其他实例持有的锁
func (m *Manager) ReleaseLock(ctx context.Context, token string) {
	if token != "" {
		_, _ = m.cache.RedisClient().Eval(ctx, releaseLockScript, []string{m.lockKey}, token).Result()
	}
	m.mu.Unlock()
}

// ============== 4. 健康检查 ==============

// HealthCheck 执行健康检查
// 铁律 06：超时控制 + 状态码白名单（2xx/3xx 视为成功）；禁用重定向跟随以捕获原始状态码
func (m *Manager) HealthCheck(ctx context.Context) error {
	url := m.cache.GetString(ctx, CfgKeyHealthCheckURL, "http://localhost:8080/health")
	timeoutSec := m.cache.GetInt(ctx, CfgKeyHealthCheckTimeout, 30)

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("构造健康检查请求失败: %w", err)
	}
	// 自定义 client：不跟随重定向（捕获原始 3xx 状态码）
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("健康检查请求失败: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("健康检查状态码异常: %d", resp.StatusCode)
	}
	return nil
}

// ============== 5. 执行更新 ==============

// ExecuteUpdate 执行完整更新流程
// 流程：1) 加锁 2) 记录 pending 日志 3) git fetch + reset --hard origin/<branch> 4) 跑部署脚本 5) 健康检查 6) 失败则回滚
// 铁律 06：每步记录 StepResult；失败立即中止并触发回滚（若启用）
func (m *Manager) ExecuteUpdate(ctx context.Context, opts UpdateOptions) (*UpdateResult, error) {
	startTime := time.Now()
	result := &UpdateResult{
		Status:        StatusRunning,
		TriggerSource: opts.TriggerSource,
		Steps:         []StepResult{},
	}

	// 1. 加锁
	token, acquired := m.AcquireLock(ctx)
	if !acquired {
		result.Status = StatusFailed
		result.ErrorMessage = "已有更新在进行中（locked）"
		return result, fmt.Errorf("update locked")
	}
	defer m.ReleaseLock(ctx, token)

	// 2. 读取目标分支
	branch := opts.Branch
	if branch == "" {
		branch = m.cache.GetString(ctx, CfgKeyWebhookBranch, "main")
	}
	result.Branch = branch

	// 3. 记录前 commit hash
	commitBefore, _ := m.getCurrentCommit(ctx)
	result.CommitBefore = commitBefore

	// 4. 创建审计日志
	log := &model.SystemUpdateLog{
		TriggerSource: opts.TriggerSource,
		TriggerBy:     opts.TriggerBy,
		TriggerIP:     opts.TriggerIP,
		CommitBefore:  commitBefore,
		Branch:        branch,
		Status:        StatusRunning,
	}
	if err := m.db.Create(log).Error; err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "创建审计日志失败: " + err.Error()
		return result, err
	}
	result.LogID = log.ID

	// 5. 执行 git pull / reset
	stepGit := m.runStep(ctx, StepGitPull, func(ctx context.Context) (string, error) {
		return m.gitPullReset(ctx, branch)
	})
	result.Steps = append(result.Steps, stepGit)

	if stepGit.Status == StepStatusFailed {
		result.Status = StatusFailed
		result.ErrorMessage = stepGit.Error
		m.finalizeLog(ctx, log, result, startTime)
		m.maybeRollback(ctx, log, result, opts)
		return result, fmt.Errorf("git pull 失败: %s", stepGit.Error)
	}

	// 6. 跑部署脚本
	stepDeploy := m.runStep(ctx, StepDeployScript, func(ctx context.Context) (string, error) {
		return m.runDeployScript(ctx)
	})
	result.Steps = append(result.Steps, stepDeploy)

	if stepDeploy.Status == StepStatusFailed {
		result.Status = StatusFailed
		result.ErrorMessage = stepDeploy.Error
		m.finalizeLog(ctx, log, result, startTime)
		m.maybeRollback(ctx, log, result, opts)
		return result, fmt.Errorf("deploy script 失败: %s", stepDeploy.Error)
	}

	// 7. 健康检查
	stepHealth := m.runStep(ctx, StepHealthCheck, func(ctx context.Context) (string, error) {
		if err := m.HealthCheck(ctx); err != nil {
			return "", err
		}
		return "OK", nil
	})
	result.Steps = append(result.Steps, stepHealth)

	if stepHealth.Status == StepStatusFailed {
		result.Status = StatusFailed
		result.ErrorMessage = stepHealth.Error
		m.finalizeLog(ctx, log, result, startTime)
		m.maybeRollback(ctx, log, result, opts)
		return result, fmt.Errorf("health check 失败: %s", stepHealth.Error)
	}

	// 8. 成功
	commitAfter, _ := m.getCurrentCommit(ctx)
	result.CommitAfter = commitAfter
	result.Status = StatusSuccess
	result.DurationMs = time.Since(startTime).Milliseconds()
	m.finalizeLog(ctx, log, result, startTime)
	return result, nil
}

// runStep 执行单步并捕获耗时与错误
func (m *Manager) runStep(ctx context.Context, name string, fn func(ctx context.Context) (string, error)) StepResult {
	start := time.Now()
	step := StepResult{Step: name, Status: StepStatusSuccess}
	output, err := fn(ctx)
	step.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		step.Status = StepStatusFailed
		step.Error = err.Error()
	}
	step.Output = output
	return step
}

// gitPullReset 执行 git fetch + reset --hard origin/<branch>
// 铁律 06：命令参数显式组合，禁止 shell 拼接用户输入
func (m *Manager) gitPullReset(ctx context.Context, branch string) (string, error) {
	// git fetch origin <branch>
	fetchCmd := exec.CommandContext(ctx, "git", "fetch", "origin", branch)
	fetchOut, err := fetchCmd.CombinedOutput()
	if err != nil {
		return string(fetchOut), fmt.Errorf("git fetch 失败: %w (output: %s)", err, string(fetchOut))
	}
	// git reset --hard origin/<branch>
	resetCmd := exec.CommandContext(ctx, "git", "reset", "--hard", "origin/"+branch)
	resetOut, err := resetCmd.CombinedOutput()
	if err != nil {
		return string(resetOut), fmt.Errorf("git reset 失败: %w (output: %s)", err, string(resetOut))
	}
	return string(resetOut), nil
}

// runDeployScript 执行部署脚本
// 铁律 06：脚本路径从 sys_config 读取；用 bash 显式调用，禁止 eval/exec 任意命令
func (m *Manager) runDeployScript(ctx context.Context) (string, error) {
	scriptPath := m.cache.GetString(ctx, CfgKeyDeployScript, "scripts/deploy_update.sh")
	if scriptPath == "" {
		return "skipped (no script configured)", nil
	}
	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("脚本执行失败: %w (output: %s)", err, string(out))
	}
	return string(out), nil
}

// getCurrentCommit 获取当前 commit hash
func (m *Manager) getCurrentCommit(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// maybeRollback 失败时自动回滚（若启用）
func (m *Manager) maybeRollback(ctx context.Context, log *model.SystemUpdateLog, result *UpdateResult, opts UpdateOptions) {
	if !m.cache.GetBool(ctx, CfgKeyRollbackEnabled, true) {
		return
	}
	if result.CommitBefore == "" {
		return
	}
	// 执行回滚
	rollbackResult, err := m.Rollback(ctx, log.ID, opts)
	if err != nil {
		// 回滚失败：在日志中追加错误，但不改变原 status
		result.ErrorMessage += " | 回滚失败: " + err.Error()
		return
	}
	result.RolledBack = true
	result.Status = StatusRolledBack
	result.Steps = append(result.Steps, rollbackResult.Steps...)
	m.finalizeLog(ctx, log, result, time.Unix(0, 0))
}

// Rollback 回滚到指定 commit（默认回滚到 log.CommitBefore）
// 铁律 06：显式 git reset --hard <commit> + 重跑脚本；记录独立审计日志
func (m *Manager) Rollback(ctx context.Context, failedLogID uint64, opts UpdateOptions) (*UpdateResult, error) {
	startTime := time.Now()
	result := &UpdateResult{
		Status:        StatusRunning,
		TriggerSource: TriggerSourceRollback,
		Steps:         []StepResult{},
	}

	// 加锁
	token, acquired := m.AcquireLock(ctx)
	if !acquired {
		result.Status = StatusFailed
		result.ErrorMessage = "已有更新在进行中（locked）"
		return result, fmt.Errorf("update locked")
	}
	defer m.ReleaseLock(ctx, token)

	// 读取原失败日志的 commit_before 作为回滚目标
	var failedLog model.SystemUpdateLog
	if err := m.db.First(&failedLog, failedLogID).Error; err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "查询原失败日志失败: " + err.Error()
		return result, err
	}
	targetCommit := failedLog.CommitBefore
	if targetCommit == "" {
		result.Status = StatusFailed
		result.ErrorMessage = "原失败日志未记录 commit_before，无法回滚"
		return result, fmt.Errorf("no commit_before to rollback to")
	}

	commitBefore, _ := m.getCurrentCommit(ctx)
	result.CommitBefore = commitBefore

	// 创建回滚审计日志
	log := &model.SystemUpdateLog{
		TriggerSource:  TriggerSourceRollback,
		TriggerBy:      opts.TriggerBy,
		TriggerIP:      opts.TriggerIP,
		CommitBefore:   commitBefore,
		CommitAfter:    targetCommit,
		Branch:         failedLog.Branch,
		Status:         StatusRunning,
		RolledBackFrom: failedLogID,
	}
	if err := m.db.Create(log).Error; err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = "创建回滚审计日志失败: " + err.Error()
		return result, err
	}
	result.LogID = log.ID

	// 执行 git reset --hard <commit>
	stepReset := m.runStep(ctx, StepRollback, func(ctx context.Context) (string, error) {
		cmd := exec.CommandContext(ctx, "git", "reset", "--hard", targetCommit)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("git reset --hard %s 失败: %w (output: %s)", targetCommit, err, string(out))
		}
		return string(out), nil
	})
	result.Steps = append(result.Steps, stepReset)

	if stepReset.Status == StepStatusFailed {
		result.Status = StatusFailed
		result.ErrorMessage = stepReset.Error
		m.finalizeLog(ctx, log, result, startTime)
		return result, fmt.Errorf("回滚失败: %s", stepReset.Error)
	}

	// 重跑部署脚本
	stepDeploy := m.runStep(ctx, StepDeployScript, func(ctx context.Context) (string, error) {
		return m.runDeployScript(ctx)
	})
	result.Steps = append(result.Steps, stepDeploy)

	if stepDeploy.Status == StepStatusFailed {
		result.Status = StatusFailed
		result.ErrorMessage = stepDeploy.Error
		m.finalizeLog(ctx, log, result, startTime)
		return result, fmt.Errorf("回滚后部署脚本失败: %s", stepDeploy.Error)
	}

	// 健康检查
	stepHealth := m.runStep(ctx, StepHealthCheck, func(ctx context.Context) (string, error) {
		if err := m.HealthCheck(ctx); err != nil {
			return "", err
		}
		return "OK", nil
	})
	result.Steps = append(result.Steps, stepHealth)

	if stepHealth.Status == StepStatusFailed {
		result.Status = StatusFailed
		result.ErrorMessage = stepHealth.Error
		m.finalizeLog(ctx, log, result, startTime)
		return result, fmt.Errorf("回滚后健康检查失败: %s", stepHealth.Error)
	}

	result.Status = StatusSuccess
	result.DurationMs = time.Since(startTime).Milliseconds()
	m.finalizeLog(ctx, log, result, startTime)
	return result, nil
}

// finalizeLog 写入最终日志到 DB
func (m *Manager) finalizeLog(ctx context.Context, log *model.SystemUpdateLog, result *UpdateResult, startTime time.Time) {
	if log == nil || log.ID == 0 {
		return
	}
	stepsJSON, _ := json.Marshal(result.Steps)
	updates := map[string]interface{}{
		"status":        result.Status,
		"commit_after":  result.CommitAfter,
		"steps_json":    string(stepsJSON),
		"error_message": truncate(result.ErrorMessage, 512),
		"duration_ms":   time.Since(startTime).Milliseconds(),
		"log_text":      buildLogText(result),
	}
	_ = m.db.Model(&model.SystemUpdateLog{}).Where("id = ?", log.ID).Updates(updates).Error
}

// truncate 截断字符串到指定长度
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// buildLogText 拼接人类可读日志
func buildLogText(result *UpdateResult) string {
	var sb strings.Builder
	for _, s := range result.Steps {
		sb.WriteString(fmt.Sprintf("[%s] %s (%dms)\n", s.Status, s.Step, s.DurationMs))
		if s.Output != "" {
			sb.WriteString("  output: " + truncate(s.Output, 500) + "\n")
		}
		if s.Error != "" {
			sb.WriteString("  error: " + s.Error + "\n")
		}
	}
	if result.ErrorMessage != "" {
		sb.WriteString("final_error: " + result.ErrorMessage + "\n")
	}
	return sb.String()
}

// ============== 6. 查询接口 ==============

// GetLatestCommit 获取当前部署的 commit hash（用于状态展示）
func (m *Manager) GetLatestCommit(ctx context.Context) string {
	commit, _ := m.getCurrentCommit(ctx)
	return commit
}

// IsAutoUpdateEnabled 是否启用了 webhook 自动更新
func (m *Manager) IsAutoUpdateEnabled(ctx context.Context) bool {
	return m.cache.GetBool(ctx, CfgKeyAutoUpdate, false)
}

// IsLocked 当前是否处于更新锁状态
func (m *Manager) IsLocked(ctx context.Context) bool {
	val, err := m.cache.RedisClient().Get(ctx, m.lockKey).Result()
	if err != nil {
		return false
	}
	return val != ""
}

// ============== 7. GitHub Release 检查更新（v0.9.0 新增） ==============

// GitHub release 检查更新相关配置键（铁律 04：禁止硬编码配置键名）
const (
	CfgKeyGithubOwner   = "update.github.owner"           // GitHub 仓库 owner（如 laobi465）
	CfgKeyGithubRepo    = "update.github.repo"            // GitHub 仓库名（如 wlyz）
	CfgKeyGithubToken   = "update.github.token"           // GitHub Personal Access Token（可选，避免匿名限流）
	CfgKeyGithubCurrent = "update.github.current_version" // 当前部署版本号（如 v0.9.0，由部署时写入）
)

// GitHubAPILimitMinInterval GitHub API 调用最小间隔（秒），防止频繁调用触发限流
// 匿名调用限流：60次/小时；带 token：5000次/小时
const GitHubAPILimitMinInterval = 60

// GitHubRelease GitHub releases/latest API 响应（仅提取关键字段）
// 完整字段见 https://docs.github.com/en/rest/releases/releases#get-the-latest-release
type GitHubRelease struct {
	TagName     string `json:"tag_name"`     // 版本号，如 v0.9.0
	Name        string `json:"name"`         // release 标题
	Body        string `json:"body"`         // release notes（markdown）
	HTMLURL     string `json:"html_url"`     // release 页面 URL
	PublishedAt string `json:"published_at"` // ISO 8601 发布时间
	Prerelease  bool   `json:"prerelease"`   // 是否预发布
	Draft       bool   `json:"draft"`        // 是否草稿
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
}

// CheckUpdateResult 检查更新结果
type CheckUpdateResult struct {
	CurrentVersion string `json:"current_version"` // 当前部署版本
	LatestVersion  string `json:"latest_version"`  // GitHub 最新 release 版本
	HasUpdate      bool   `json:"has_update"`      // 是否有新版本
	ReleaseNotes   string `json:"release_notes"`   // 更新内容（markdown）
	ReleaseURL     string `json:"release_url"`     // release 页面 URL
	PublishedAt    string `json:"published_at"`    // 发布时间（ISO 8601）
	Author         string `json:"author"`          // 发布者
	Prerelease     bool   `json:"prerelease"`      // 是否预发布
	RepoOwner      string `json:"repo_owner"`      // 仓库 owner（用于前端跳转）
	RepoName       string `json:"repo_name"`       // 仓库名
	CurrentCommit  string `json:"current_commit"`  // 当前部署 commit hash
	CheckedAt      int64  `json:"checked_at"`      // 检查时间戳（秒）
	IsLocked       bool   `json:"is_locked"`       // 当前是否有更新锁
}

// githubCheckCache 进程内缓存（避免短时间内重复调用 GitHub API 触发限流）
// 缓存策略：GitHubAPILimitMinInterval 秒内的重复请求直接返回缓存结果
var (
	githubCheckCache     *CheckUpdateResult
	githubCheckCacheAt   time.Time
	githubCheckCacheLock sync.Mutex
)

// CheckUpdate 主动调用 GitHub API 检查是否有新版本 release
//
// 严格遵循铁律：
//
//	04 - owner/repo/token/current_version 全部从 sys_config 读取，无硬编码
//	05 - 4 项 update.github.* 配置可通过后台「系统配置」实时调整
//	06 - token 仅用于 Authorization 头不回显；网络错误显式返回 error 不编造数据
//
// 缓存策略：进程内缓存 GitHubAPILimitMinInterval 秒，防止管理员频繁点击触发 GitHub 限流
// 限流参考：匿名 60次/小时；带 token 5000次/小时
func (m *Manager) CheckUpdate(ctx context.Context) (*CheckUpdateResult, error) {
	// 1. 进程内缓存检查（防短时间重复调用）
	githubCheckCacheLock.Lock()
	if githubCheckCache != nil && time.Since(githubCheckCacheAt) < time.Duration(GitHubAPILimitMinInterval)*time.Second {
		cached := *githubCheckCache
		cached.CheckedAt = githubCheckCacheAt.Unix()
		cached.IsLocked = m.IsLocked(ctx)
		githubCheckCacheLock.Unlock()
		return &cached, nil
	}
	githubCheckCacheLock.Unlock()

	// 2. 从 sys_config 读取 GitHub 仓库信息
	owner := m.cache.GetString(ctx, CfgKeyGithubOwner, "")
	repo := m.cache.GetString(ctx, CfgKeyGithubRepo, "")
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("未配置 GitHub 仓库信息，请在「系统配置 > update.github.owner/repo」中设置")
	}
	token := m.cache.GetString(ctx, CfgKeyGithubToken, "")
	currentVersion := m.cache.GetString(ctx, CfgKeyGithubCurrent, "")

	// 3. 当前部署 commit hash（用于辅助判断）
	currentCommit := m.GetLatestCommit(ctx)

	// 4. 调用 GitHub API: GET /repos/{owner}/{repo}/releases/latest
	//    官方文档：https://docs.github.com/en/rest/releases/releases#get-the-latest-release
	//    注意：releases/latest 不返回 prerelease 和 draft
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 GitHub API 请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "KeyAuth-SaaS-UpdateChecker")
	if token != "" {
		// 铁律 06：token 仅用于 Authorization 头，不记录到日志、不返回给前端
		req.Header.Set("Authorization", "Bearer "+token)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// 仓库无 release（新仓库或未发布 release）
		return nil, fmt.Errorf("GitHub 仓库 %s/%s 暂无 release，请先在 GitHub 上发布一个 release", owner, repo)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		// 限流或 token 无效
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API 鉴权/限流失败（HTTP %d）: %s；如未配置 token 建议在「系统配置」中设置 update.github.token", resp.StatusCode, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API 返回异常状态码 %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 GitHub API 响应失败: %w", err)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("解析 GitHub release 响应失败: %w", err)
	}

	// 5. 版本对比（规范化 tag 前缀 v/V 后做字符串比较）
	latestVer := normalizeVersionTag(release.TagName)
	currentVer := normalizeVersionTag(currentVersion)
	hasUpdate := false
	if currentVer == "" {
		// 当前版本未配置：默认认为有更新（提示管理员去更新）
		hasUpdate = true
	} else if latestVer != "" && latestVer != currentVer {
		hasUpdate = true
	}

	result := &CheckUpdateResult{
		CurrentVersion: currentVersion,
		LatestVersion:  release.TagName,
		HasUpdate:      hasUpdate,
		ReleaseNotes:   release.Body,
		ReleaseURL:     release.HTMLURL,
		PublishedAt:    release.PublishedAt,
		Author:         release.Author.Login,
		Prerelease:     release.Prerelease,
		RepoOwner:      owner,
		RepoName:       repo,
		CurrentCommit:  currentCommit,
		CheckedAt:      time.Now().Unix(),
		IsLocked:       m.IsLocked(ctx),
	}

	// 6. 写入进程内缓存
	githubCheckCacheLock.Lock()
	githubCheckCache = result
	githubCheckCacheAt = time.Now()
	githubCheckCacheLock.Unlock()

	return result, nil
}

// normalizeVersionTag 规范化版本号用于比较
// 去除前缀 v/V，仅保留语义化版本字符串
// 例：v0.9.0 → 0.9.0；V1.0.0 → 1.0.0；0.9.0 → 0.9.0
func normalizeVersionTag(tag string) string {
	if tag == "" {
		return ""
	}
	// 去除前后空格
	tag = strings.TrimSpace(tag)
	// 去除前缀 v 或 V
	if len(tag) > 0 && (tag[0] == 'v' || tag[0] == 'V') {
		tag = tag[1:]
	}
	return tag
}
