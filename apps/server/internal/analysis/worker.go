// v0.6.0 高级分析聚合 Worker
//
// 职责：
//   1. 定时聚合 log_verify 到 user_behavior_profile + card_usage_profile（按日）
//   2. 定时重算所有用户的 user_risk_score（评分衰减 + 异常模式检测）
//
// 调度策略：
//   - 聚合间隔从 sys_config 读取（analysis.aggregate_interval_seconds，默认 3600s）
//   - 每次运行聚合「昨日」+「今日」（覆盖跨日边界）
//   - 风险评分重算紧跟聚合之后
package analysis

import (
	"context"
	"log"
	"time"
)

// StartAggregationWorker 启动聚合 worker（阻塞调用，应放在 goroutine 中）
//
// 调用方应在 main.go 中：
//
//	go analysis.StartAggregationWorker(ctx, mgr)
//
// worker 行为：
//   - 启动后立即执行一次
//   - 之后按 aggregate_interval_seconds 间隔周期执行
//   - 每次执行：聚合昨日 + 今日的行为数据 + 重算所有用户风险评分
//   - ctx 取消时优雅退出
func StartAggregationWorker(ctx context.Context, mgr *Manager) {
	// 首次启动立即执行一次（不等待第一个 tick）
	mgr.runAggregationOnce(ctx)

	for {
		interval := time.Duration(mgr.cfg.GetInt(ctx, CfgKeyAggregateInterval, 3600)) * time.Second
		if interval < 60*time.Second {
			interval = 60 * time.Second // 最小间隔保护
		}
		select {
		case <-ctx.Done():
			log.Println("[analysis] worker 退出")
			return
		case <-time.After(interval):
			mgr.runAggregationOnce(ctx)
		}
	}
}

// runAggregationOnce 执行一次完整聚合
// 步骤：
//  1. 聚合昨日用户行为 + 卡密画像（覆盖跨日边界）
//  2. 聚合今日用户行为 + 卡密画像
//  3. 重算所有用户风险评分
func (m *Manager) runAggregationOnce(ctx context.Context) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	dates := []string{
		statDateStr(yesterday),
		statDateStr(now),
	}

	// 1. 行为聚合
	if m.cfg.GetBool(ctx, CfgKeyBehaviorEnabled, true) {
		for _, date := range dates {
			n, err := m.AggregateUserBehaviorForDate(ctx, date)
			if err != nil {
				log.Printf("[analysis] 聚合用户行为失败 date=%s err=%v", date, err)
				continue
			}
			log.Printf("[analysis] 聚合用户行为完成 date=%s users=%d", date, n)
		}
	}

	// 2. 卡密画像聚合
	if m.cfg.GetBool(ctx, CfgKeyCardProfileEnabled, true) {
		for _, date := range dates {
			n, err := m.AggregateCardProfileForDate(ctx, date)
			if err != nil {
				log.Printf("[analysis] 聚合卡密画像失败 date=%s err=%v", date, err)
				continue
			}
			log.Printf("[analysis] 聚合卡密画像完成 date=%s cards=%d", date, n)
		}
	}

	// 3. 风险评分重算
	if m.cfg.GetBool(ctx, CfgKeyRiskScoreEnabled, true) {
		n, err := m.ReevaluateAllRiskScores(ctx)
		if err != nil {
			log.Printf("[analysis] 风险评分重算失败 err=%v", err)
		} else {
			log.Printf("[analysis] 风险评分重算完成 users=%d", n)
		}
	}
}

// RunAggregationOnceSync 同步执行一次聚合（用于测试或手动触发）
// 返回聚合统计
func (m *Manager) RunAggregationOnceSync(ctx context.Context) (usersAggregated, cardsAggregated, riskReevaluated int, err error) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	dates := []string{statDateStr(yesterday), statDateStr(now)}

	for _, date := range dates {
		n, e := m.AggregateUserBehaviorForDate(ctx, date)
		if e != nil && err == nil {
			err = e
		}
		usersAggregated += n

		cn, e := m.AggregateCardProfileForDate(ctx, date)
		if e != nil && err == nil {
			err = e
		}
		cardsAggregated += cn
	}

	riskReevaluated, e := m.ReevaluateAllRiskScores(ctx)
	if e != nil && err == nil {
		err = e
	}
	return
}
