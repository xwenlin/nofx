package logger

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/config"
	"os"
	"path/filepath"
	"time"
)

// DecisionRecord 决策记录
type DecisionRecord struct {
	ID                  int64              `json:"id,omitempty"`                     // 决策日志ID（数据库主键，用于按需加载）
	Timestamp           time.Time          `json:"timestamp"`                        // 决策时间
	CycleNumber         int                `json:"cycle_number"`                     // 周期编号
	SystemPrompt        string             `json:"system_prompt"`                    // 系统提示词（发送给AI的系统prompt）
	InputPrompt         string             `json:"input_prompt"`                     // 发送给AI的输入prompt
	CoTTrace            string             `json:"cot_trace"`                        // AI思维链（输出）
	DecisionJSON        string             `json:"decision_json"`                    // 决策JSON
	AccountState        AccountSnapshot    `json:"account_state"`                    // 账户状态快照
	Positions           []PositionSnapshot `json:"positions"`                        // 持仓快照
	CandidateCoins      []string           `json:"candidate_coins"`                  // 候选币种列表
	Decisions           []DecisionAction   `json:"decisions"`                        // 执行的决策
	ExecutionLog        []string           `json:"execution_log"`                    // 执行日志
	Success             bool               `json:"success"`                          // 是否成功
	ErrorMessage        string             `json:"error_message"`                    // 错误信息（如果有）
	AIRequestDurationMs int64              `json:"ai_request_duration_ms,omitempty"` // AI API 调用耗时（毫秒）
}

// AccountSnapshot 账户状态快照
type AccountSnapshot struct {
	TotalBalance          float64 `json:"total_balance"`
	AvailableBalance      float64 `json:"available_balance"`
	TotalUnrealizedProfit float64 `json:"total_unrealized_profit"`
	PositionCount         int     `json:"position_count"`
	MarginUsedPct         float64 `json:"margin_used_pct"`
	InitialBalance        float64 `json:"initial_balance"` // 记录当时的初始余额基准
}

// PositionSnapshot 持仓快照
type PositionSnapshot struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"`
	PositionAmt      float64 `json:"position_amt"`
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	UnrealizedProfit float64 `json:"unrealized_profit"`
	Leverage         float64 `json:"leverage"`
	LiquidationPrice float64 `json:"liquidation_price"`
	StopLoss         float64 `json:"stop_loss,omitempty"`   // 当前止损价格
	TakeProfit       float64 `json:"take_profit,omitempty"` // 当前止盈价格
}

// DecisionAction 决策动作
type DecisionAction struct {
	Action        string    `json:"action"`                    // open_long, open_short, close_long, close_short, update_stop_loss, update_take_profit, partial_close
	Symbol        string    `json:"symbol"`                    // 币种
	Quantity      float64   `json:"quantity"`                  // 数量（部分平仓时使用）
	Leverage      int       `json:"leverage"`                  // 杠杆（开仓时）
	Price         float64   `json:"price"`                     // 执行价格
	OrderID       int64     `json:"order_id"`                  // 订单ID
	Timestamp     time.Time `json:"timestamp"`                 // 执行时间
	Success       bool      `json:"success"`                   // 是否成功
	Error         string    `json:"error"`                     // 错误信息
	Reasoning     string    `json:"reasoning"`                 // 决策原因（AI提供的reasoning）
	NewStopLoss   float64   `json:"new_stop_loss,omitempty"`   // 新止损价格（用于 update_stop_loss）
	NewTakeProfit float64   `json:"new_take_profit,omitempty"` // 新止盈价格（用于 update_take_profit）
	OldStopLoss   float64   `json:"old_stop_loss,omitempty"`   // 旧止损价格（用于 update_stop_loss，记录移动前的止损价）
}

// DecisionLogger 决策日志记录器
type DecisionLogger struct {
	logDir      string
	cycleNumber int
	db          config.DatabaseInterface
	traderID    string
}

// NewDecisionLogger 创建决策日志记录器
func NewDecisionLogger(logDir string) *DecisionLogger {
	if logDir == "" {
		logDir = "decision_logs"
	}

	// 确保日志目录存在（使用安全权限：只有所有者可访问）
	if err := os.MkdirAll(logDir, 0700); err != nil {
		fmt.Printf("⚠ 创建日志目录失败: %v\n", err)
	}

	// 强制设置目录权限（即使目录已存在）- 确保安全
	if err := os.Chmod(logDir, 0700); err != nil {
		fmt.Printf("⚠ 设置日志目录权限失败: %v\n", err)
	}

	return &DecisionLogger{
		logDir:      logDir,
		cycleNumber: 0,
	}
}

// SetDatabase 设置数据库
func (l *DecisionLogger) SetDatabase(db config.DatabaseInterface, traderID string) {
	l.db = db
	l.traderID = traderID
}

// LogDecision 记录决策
func (l *DecisionLogger) LogDecision(record *DecisionRecord) error {
	l.cycleNumber++
	record.CycleNumber = l.cycleNumber
	// 使用 UTC 时间确保时区一致性
	record.Timestamp = time.Now().UTC()

	// 生成文件名：decision_YYYYMMDD_HHMMSS_cycleN.json
	filename := fmt.Sprintf("decision_%s_cycle%d.json",
		record.Timestamp.Format("20060102_150405"),
		record.CycleNumber)

	filepath := filepath.Join(l.logDir, filename)

	// 序列化为JSON（带缩进，方便阅读）
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化决策记录失败: %w", err)
	}

	// 1. 写入文件（保持原有逻辑作为备份）
	// 写入文件（使用安全权限：只有所有者可读写）
	if err := os.WriteFile(filepath, data, 0600); err != nil {
		// 记录错误但继续尝试写入数据库
		fmt.Printf("⚠ 写入决策日志文件失败: %v\n", err)
	} else {
		fmt.Printf("📝 决策记录已保存到文件: %s\n", filename)
	}

	// 2. 写入数据库（如果已配置）
	if l.db != nil && l.traderID != "" {
		// 序列化复杂对象
		execLogJSON, _ := json.Marshal(record.ExecutionLog)
		accountStateJSON, _ := json.Marshal(record.AccountState)
		positionsJSON, _ := json.Marshal(record.Positions)
		decisionsJSON, _ := json.Marshal(record.Decisions)

		log := &config.DecisionLog{
			TraderID:            l.traderID,
			CycleNumber:         record.CycleNumber,
			Timestamp:           record.Timestamp,
			Content:             "", // 不再存储完整Content，各字段已单独存储
			SystemPrompt:        record.SystemPrompt,
			InputPrompt:         record.InputPrompt,
			CoTTrace:            record.CoTTrace,
			DecisionJSON:        record.DecisionJSON,
			Decisions:           string(decisionsJSON), // 单独存储decisions
			AccountState:        string(accountStateJSON),
			Positions:           string(positionsJSON),
			ExecutionLog:        string(execLogJSON),
			Success:             record.Success,
			Error:               record.ErrorMessage,
			AIRequestDurationMs: record.AIRequestDurationMs,
		}

		if err := l.db.CreateDecisionLog(log); err != nil {
			fmt.Printf("⚠ 写入决策日志到数据库失败: %v\n", err)
			return fmt.Errorf("写入决策日志到数据库失败: %w", err)
		} else {
			fmt.Printf("📝 决策记录已保存到数据库 (ID: %d)\n", log.ID)
		}
	}

	return nil
}

// GetLatestRecords 获取最近N条记录（按时间正序：从旧到新）
func (l *DecisionLogger) GetLatestRecords(n int) ([]*DecisionRecord, error) {
	if l.db == nil || l.traderID == "" {
		return nil, fmt.Errorf("数据库未配置，无法获取决策记录")
	}

	logs, err := l.db.GetDecisionLogs(l.traderID, n)
	if err != nil {
		return nil, fmt.Errorf("从数据库读取决策日志失败: %w", err)
	}

	var records []*DecisionRecord
	// 数据库返回的是按时间倒序（最新的在前）
	for _, logEntry := range logs {
		var record DecisionRecord
		if err := json.Unmarshal([]byte(logEntry.Content), &record); err != nil {
			continue
		}
		records = append(records, &record)
	}

	// 反转数组，让时间从旧到新排列（用于图表显示等需要时间正序的场景）
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// GetRecordByDate 获取指定日期的所有记录
func (l *DecisionLogger) GetRecordByDate(date time.Time) ([]*DecisionRecord, error) {
	if l.db == nil || l.traderID == "" {
		return nil, fmt.Errorf("数据库未配置，无法获取决策记录")
	}

	// 获取该日期的开始和结束时间
	startTime := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endTime := startTime.AddDate(0, 0, 1)

	// 获取足够多的记录（假设一天最多1000条）
	logs, err := l.db.GetDecisionLogs(l.traderID, 1000)
	if err != nil {
		return nil, fmt.Errorf("从数据库读取决策日志失败: %w", err)
	}

	var records []*DecisionRecord
	for _, logEntry := range logs {
		// 过滤出指定日期的记录
		if logEntry.Timestamp.After(startTime) && logEntry.Timestamp.Before(endTime) {
			var record DecisionRecord
			if err := json.Unmarshal([]byte(logEntry.Content), &record); err != nil {
				continue
			}
			records = append(records, &record)
		}
	}

	return records, nil
}

// CleanOldRecords 清理N天前的旧记录（数据库记录由数据库自动管理，此方法保留用于清理文件备份）
func (l *DecisionLogger) CleanOldRecords(days int) error {
	// 数据库记录由数据库自动管理，不需要手动清理
	// 此方法保留用于清理文件备份（如果需要）
	cutoffTime := time.Now().AddDate(0, 0, -days)

	files, err := os.ReadDir(l.logDir)
	if err != nil {
		return fmt.Errorf("读取日志目录失败: %w", err)
	}

	removedCount := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			filepath := filepath.Join(l.logDir, file.Name())
			if err := os.Remove(filepath); err != nil {
				fmt.Printf("⚠ 删除旧文件备份失败 %s: %v\n", file.Name(), err)
				continue
			}
			removedCount++
		}
	}

	if removedCount > 0 {
		fmt.Printf("🗑️ 已清理 %d 个旧文件备份（%d天前）\n", removedCount, days)
	}

	return nil
}

// GetStatistics 获取统计信息
func (l *DecisionLogger) GetStatistics() (*Statistics, error) {
	if l.db == nil || l.traderID == "" {
		return nil, fmt.Errorf("数据库未配置，无法获取统计信息")
	}

	// 获取所有记录（使用足够大的limit）
	logs, err := l.db.GetDecisionLogs(l.traderID, 10000)
	if err != nil {
		return nil, fmt.Errorf("从数据库读取决策日志失败: %w", err)
	}

	stats := &Statistics{}

	for _, logEntry := range logs {
		var record DecisionRecord
		if err := json.Unmarshal([]byte(logEntry.Content), &record); err != nil {
			continue
		}

		stats.TotalCycles++

		for _, action := range record.Decisions {
			if action.Success {
				switch action.Action {
				case "open_long", "open_short":
					stats.TotalOpenPositions++
				case "close_long", "close_short", "auto_close_long", "auto_close_short":
					stats.TotalClosePositions++
					// 🔧 BUG FIX：partial_close 不計入 TotalClosePositions，避免重複計數
					// case "partial_close": // 不計數，因為只有完全平倉才算一次
					// update_stop_loss 和 update_take_profit 不計入統計
				}
			}
		}

		if record.Success {
			stats.SuccessfulCycles++
		} else {
			stats.FailedCycles++
		}
	}

	return stats, nil
}

// Statistics 统计信息
type Statistics struct {
	TotalCycles         int `json:"total_cycles"`
	SuccessfulCycles    int `json:"successful_cycles"`
	FailedCycles        int `json:"failed_cycles"`
	TotalOpenPositions  int `json:"total_open_positions"`
	TotalClosePositions int `json:"total_close_positions"`
}

// TradeOutcome 单笔交易结果
type TradeOutcome struct {
	Symbol        string    `json:"symbol"`         // 币种
	Side          string    `json:"side"`           // long/short
	Quantity      float64   `json:"quantity"`       // 仓位数量
	Leverage      int       `json:"leverage"`       // 杠杆倍数
	OpenPrice     float64   `json:"open_price"`     // 开仓价
	ClosePrice    float64   `json:"close_price"`    // 平仓价
	PositionValue float64   `json:"position_value"` // 仓位价值（quantity × openPrice）
	MarginUsed    float64   `json:"margin_used"`    // 保证金使用（positionValue / leverage）
	PnL           float64   `json:"pn_l"`           // 盈亏（USDT）
	PnLPct        float64   `json:"pn_l_pct"`       // 盈亏百分比（相对保证金）
	Duration      string    `json:"duration"`       // 持仓时长
	OpenTime      time.Time `json:"open_time"`      // 开仓时间
	CloseTime     time.Time `json:"close_time"`     // 平仓时间
	WasStopLoss   bool      `json:"was_stop_loss"`  // 是否止损
}

// PerformanceAnalysis 交易表现分析
type PerformanceAnalysis struct {
	TotalTrades   int                           `json:"total_trades"`   // 总交易数
	WinningTrades int                           `json:"winning_trades"` // 盈利交易数
	LosingTrades  int                           `json:"losing_trades"`  // 亏损交易数
	WinRate       float64                       `json:"win_rate"`       // 胜率
	AvgWin        float64                       `json:"avg_win"`        // 平均盈利（USDT）
	AvgLoss       float64                       `json:"avg_loss"`       // 平均亏损（USDT）
	AvgWinPct     float64                       `json:"avg_win_pct"`    // 平均盈利百分比
	AvgLossPct    float64                       `json:"avg_loss_pct"`   // 平均亏损百分比
	ProfitFactor  float64                       `json:"profit_factor"`  // 盈亏比
	SharpeRatio   float64                       `json:"sharpe_ratio"`   // 夏普比率（风险调整后收益）
	RecentTrades  []TradeOutcome                `json:"recent_trades"`  // 最近N笔交易
	SymbolStats   map[string]*SymbolPerformance `json:"symbol_stats"`   // 各币种表现
	BestSymbol    string                        `json:"best_symbol"`    // 表现最好的币种
	WorstSymbol   string                        `json:"worst_symbol"`   // 表现最差的币种
}

// SymbolPerformance 币种表现统计
type SymbolPerformance struct {
	Symbol        string  `json:"symbol"`         // 币种
	TotalTrades   int     `json:"total_trades"`   // 交易次数
	WinningTrades int     `json:"winning_trades"` // 盈利次数
	LosingTrades  int     `json:"losing_trades"`  // 亏损次数
	WinRate       float64 `json:"win_rate"`       // 胜率
	TotalPnL      float64 `json:"total_pn_l"`     // 总盈亏
	AvgPnL        float64 `json:"avg_pn_l"`       // 平均盈亏
}

// AnalyzePerformance 分析最近N个周期的交易表现（从数据库查询）
func (l *DecisionLogger) AnalyzePerformance(lookbackCycles int, database interface{}, traderID string) (*PerformanceAnalysis, error) {
	if database == nil || traderID == "" {
		return nil, fmt.Errorf("数据库未配置，无法分析交易表现")
	}
	return l.analyzePerformanceFromDB(database, traderID)
}

// analyzePerformanceFromDB 从数据库分析交易表现
func (l *DecisionLogger) analyzePerformanceFromDB(database interface{}, traderID string) (*PerformanceAnalysis, error) {
	db, ok := database.(interface {
		GetTradesByTrader(traderID string, limit int) ([]*config.TradeRecord, error)
	})
	if !ok {
		return nil, fmt.Errorf("数据库接口不支持 GetTradesByTrader")
	}

	// 获取最近20笔已平仓的交易（用于计算所有指标）
	trades, err := db.GetTradesByTrader(traderID, 20)
	if err != nil {
		return nil, fmt.Errorf("从数据库查询交易记录失败: %w", err)
	}

	analysis := &PerformanceAnalysis{
		RecentTrades: []TradeOutcome{},
		SymbolStats:  make(map[string]*SymbolPerformance),
	}

	// 转换数据库记录为 TradeOutcome
	for _, trade := range trades {
		if trade.CloseTime.IsZero() {
			continue // 跳过未平仓的交易
		}

		duration := trade.CloseTime.Sub(trade.OpenTime).String()
		outcome := TradeOutcome{
			Symbol:        trade.Symbol,
			Side:          trade.Side,
			Quantity:      trade.Quantity,
			Leverage:      trade.Leverage,
			OpenPrice:     trade.OpenPrice,
			ClosePrice:    trade.ClosePrice,
			PositionValue: trade.Quantity * trade.OpenPrice,
			MarginUsed:    (trade.Quantity * trade.OpenPrice) / float64(trade.Leverage),
			PnL:           trade.PnL,
			PnLPct:        trade.PnLPct,
			Duration:      duration,
			OpenTime:      trade.OpenTime,
			CloseTime:     trade.CloseTime,
			WasStopLoss:   trade.WasStopLoss,
		}
		analysis.RecentTrades = append(analysis.RecentTrades, outcome)
	}

	// 反转数组，让最新的在前
	if len(analysis.RecentTrades) > 0 {
		for i, j := 0, len(analysis.RecentTrades)-1; i < j; i, j = i+1, j-1 {
			analysis.RecentTrades[i], analysis.RecentTrades[j] = analysis.RecentTrades[j], analysis.RecentTrades[i]
		}
	}

	// 计算统计指标（使用最近20笔交易）
	tradesForStats := analysis.RecentTrades
	if len(tradesForStats) > 20 {
		tradesForStats = tradesForStats[:20]
	}

	// 计算总交易数、胜率等
	analysis.TotalTrades = len(tradesForStats)
	analysis.WinningTrades = 0
	analysis.LosingTrades = 0
	totalWinAmount := 0.0
	totalLossAmount := 0.0

	for _, trade := range tradesForStats {
		if trade.PnLPct > 0 {
			analysis.WinningTrades++
			totalWinAmount += trade.PnL
		} else if trade.PnLPct < 0 {
			analysis.LosingTrades++
			totalLossAmount += math.Abs(trade.PnL)
		}
	}

	if analysis.TotalTrades > 0 {
		analysis.WinRate = (float64(analysis.WinningTrades) / float64(analysis.TotalTrades)) * 100
		if analysis.WinningTrades > 0 {
			analysis.AvgWin = totalWinAmount / float64(analysis.WinningTrades)
			analysis.AvgWinPct = 0.0
			for _, trade := range tradesForStats {
				if trade.PnLPct > 0 {
					analysis.AvgWinPct += trade.PnLPct
				}
			}
			analysis.AvgWinPct = analysis.AvgWinPct / float64(analysis.WinningTrades)
		}
		if analysis.LosingTrades > 0 {
			analysis.AvgLoss = totalLossAmount / float64(analysis.LosingTrades)
			analysis.AvgLossPct = 0.0
			for _, trade := range tradesForStats {
				if trade.PnLPct < 0 {
					analysis.AvgLossPct += math.Abs(trade.PnLPct)
				}
			}
			analysis.AvgLossPct = analysis.AvgLossPct / float64(analysis.LosingTrades)
		}
		if totalLossAmount > 0 {
			analysis.ProfitFactor = totalWinAmount / totalLossAmount
		}
	}

	// 计算币种统计
	for _, trade := range tradesForStats {
		if _, exists := analysis.SymbolStats[trade.Symbol]; !exists {
			analysis.SymbolStats[trade.Symbol] = &SymbolPerformance{
				Symbol:        trade.Symbol,
				TotalTrades:   0,
				WinningTrades: 0,
				LosingTrades:  0,
				TotalPnL:      0.0,
			}
		}
		stats := analysis.SymbolStats[trade.Symbol]
		stats.TotalTrades++
		if trade.PnLPct > 0 {
			stats.WinningTrades++
		} else if trade.PnLPct < 0 {
			stats.LosingTrades++
		}
		stats.TotalPnL += trade.PnL
	}

	// 计算最佳和最差币种
	bestPnL := -math.MaxFloat64
	worstPnL := math.MaxFloat64
	for symbol, stats := range analysis.SymbolStats {
		if stats.TotalPnL > bestPnL {
			bestPnL = stats.TotalPnL
			analysis.BestSymbol = symbol
		}
		if stats.TotalPnL < worstPnL {
			worstPnL = stats.TotalPnL
			analysis.WorstSymbol = symbol
		}
		if stats.TotalTrades > 0 {
			stats.WinRate = (float64(stats.WinningTrades) / float64(stats.TotalTrades)) * 100
			stats.AvgPnL = stats.TotalPnL / float64(stats.TotalTrades)
		}
	}

	// 计算夏普比率
	analysis.SharpeRatio = l.calculateRollingSharpeRatio(tradesForStats)

	// 限制 RecentTrades 为最近10笔（用于显示）
	if len(analysis.RecentTrades) > 10 {
		analysis.RecentTrades = analysis.RecentTrades[:10]
	}

	return analysis, nil
}

// calculateRollingSharpeRatio 计算滚动夏普比率
// 基于过去N笔交易的收益率序列计算风险调整后收益
// 交易按时间顺序排列（旧到新）
func (l *DecisionLogger) calculateRollingSharpeRatio(trades []TradeOutcome) float64 {
	if len(trades) < 2 {
		return 0.0
	}

	// 提取每笔交易的收益率（PnLPct转换为小数形式）
	// PnLPct是相对于保证金的盈亏百分比，需要转换为收益率
	var returns []float64
	for _, trade := range trades {
		// 将百分比转换为小数形式（例如：5.0% -> 0.05）
		// 收益率基于保证金，反映每笔交易的风险调整收益
		returnRate := trade.PnLPct / 100.0
		returns = append(returns, returnRate)
	}

	if len(returns) == 0 {
		return 0.0
	}

	// 计算平均收益率
	sumReturns := 0.0
	for _, r := range returns {
		sumReturns += r
	}
	meanReturn := sumReturns / float64(len(returns))

	// 计算收益率标准差
	sumSquaredDiff := 0.0
	for _, r := range returns {
		diff := r - meanReturn
		sumSquaredDiff += diff * diff
	}
	variance := sumSquaredDiff / float64(len(returns))
	stdDev := math.Sqrt(variance)

	// 避免除以零
	if stdDev == 0 {
		if meanReturn > 0 {
			return 999.0 // 无波动的正收益
		} else if meanReturn < 0 {
			return -999.0 // 无波动的负收益
		}
		return 0.0
	}

	// 计算夏普比率（假设无风险利率为0）
	// 注：基于交易收益率的夏普比率，反映每笔交易的风险调整收益
	// 正常范围 -2 到 +2，值越高表示风险调整后的收益越好
	sharpeRatio := meanReturn / stdDev
	return sharpeRatio
}
