package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"regexp"
	"strings"
	"time"
)

// 预编译正则表达式（性能优化：避免每次调用时重新编译）
var (
	// ✅ 安全的正則：精確匹配 ```json 代碼塊
	// 使用反引號 + 拼接避免轉義問題
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")

	// 新增：XML标签提取（支持思维链中包含任何字符）
	reReasoningTag = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
	reDecisionTag  = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)
)

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	PeakPnLPct       float64 `json:"peak_pnl_pct"` // 历史最高收益率（百分比）
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // 持仓更新时间戳（毫秒）
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // 未实现盈亏
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长Top数据（用于AI决策参考）
type OITopData struct {
	Rank              int     // OI Top排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// Context 交易上下文（传递给AI的完整信息）
type Context struct {
	CurrentTime           string                  `json:"current_time"`
	RuntimeMinutes        int                     `json:"runtime_minutes"`
	CallCount             int                     `json:"call_count"`
	Account               AccountInfo             `json:"account"`
	Positions             []PositionInfo          `json:"positions"`
	CandidateCoins        []CandidateCoin         `json:"candidate_coins"`
	MarketDataMap         map[string]*market.Data `json:"-"` // 不序列化，但内部使用
	OITopDataMap          map[string]*OITopData   `json:"-"` // OI Top数据映射
	Performance           interface{}             `json:"-"` // 历史表现分析（logger.PerformanceAnalysis）
	BTCETHLeverage        int                     `json:"-"` // BTC/ETH杠杆倍数（从配置读取）
	AltcoinLeverage       int                     `json:"-"` // 山寨币杠杆倍数（从配置读取）
	LastExecutionFeedback *ExecutionFeedback      `json:"-"` // 上轮指令执行反馈
}

// ExecutionFeedback 上轮指令执行反馈
type ExecutionFeedback struct {
	HasRejected       bool               // 是否有被拒绝的指令
	RejectedDecisions []RejectedDecision // 被拒绝的决策列表
}

// RejectedDecision 被拒绝的决策
type RejectedDecision struct {
	Symbol   string // 币种
	Action   string // 操作
	Leverage int    // 杠杆（如果有）
	Reason   string // 拒绝原因
}

// Decision AI的交易决策
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // "open_long", "open_short", "close_long", "close_short", "update_stop_loss", "update_take_profit", "partial_close", "hold", "wait"

	// 开仓参数
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`

	// 调整参数（新增）
	NewStopLoss     float64 `json:"new_stop_loss,omitempty"`    // 用于 update_stop_loss
	NewTakeProfit   float64 `json:"new_take_profit,omitempty"`  // 用于 update_take_profit
	ClosePercentage float64 `json:"close_percentage,omitempty"` // 用于 partial_close (0-100)

	// 通用参数
	Confidence int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning  string  `json:"reasoning"`
}

// FullDecision AI的完整决策（包含思维链）
type FullDecision struct {
	SystemPrompt    string     `json:"system_prompt"`     // 系统提示词（发送给AI的系统prompt）
	UserPrompt      string     `json:"user_prompt"`       // 发送给AI的输入prompt
	CoTTrace        string     `json:"cot_trace"`         // 思维链分析（AI输出）
	Decisions       []Decision `json:"decisions"`         // 具体决策列表（经过验证的）
	RawDecisionJSON string     `json:"raw_decision_json"` // AI返回的原始决策JSON（未验证）
	Timestamp       time.Time  `json:"timestamp"`
	// AIRequestDurationMs 记录 AI API 调用耗时（毫秒）方便排查延迟问题
	AIRequestDurationMs int64 `json:"ai_request_duration_ms,omitempty"`
}

// GetFullDecision 获取AI的完整交易决策（批量分析所有币种和持仓）
func GetFullDecision(ctx *Context, mcpClient *mcp.Client) (*FullDecision, error) {
	return GetFullDecisionWithCustomPrompt(ctx, mcpClient, "", false, "")
}

// GetFullDecisionWithCustomPrompt 获取AI的完整交易决策（支持自定义prompt和模板选择）
func GetFullDecisionWithCustomPrompt(ctx *Context, mcpClient *mcp.Client, customPrompt string, overrideBase bool, templateName string) (*FullDecision, error) {
	// 1. 为所有币种获取市场数据
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	systemPrompt := buildSystemPromptWithCustom(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, customPrompt, overrideBase, templateName)
	userPrompt := buildUserPrompt(ctx)

	// 3. 调用AI API（使用 system + user prompt）
	aiCallStart := time.Now()
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	aiCallDuration := time.Since(aiCallStart)
	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	// 4. 解析AI响应
	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage)

	// 无论是否有错误，都要保存 SystemPrompt 和 UserPrompt（用于调试和决策未执行后的问题定位）
	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt // 保存系统prompt
		decision.UserPrompt = userPrompt     // 保存输入prompt
		decision.AIRequestDurationMs = aiCallDuration.Milliseconds()
	}

	if err != nil {
		return decision, fmt.Errorf("解析AI响应失败: %w", err)
	}

	return decision, nil
}

// fetchMarketDataForContext 为上下文中的所有币种获取市场数据和OI数据
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// 收集所有需要获取数据的币种
	symbolSet := make(map[string]bool)

	// 1. 优先获取持仓币种的数据（这是必须的）
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. 候选币种数量根据账户状态动态调整
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// 并发获取市场数据
	// 持仓币种集合（用于判断是否跳过OI检查）
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	for symbol := range symbolSet {
		data, err := market.Get(symbol)
		if err != nil {
			// 单个币种失败不影响整体，只记录错误
			continue
		}

		// ⚠️ 流动性过滤：持仓价值低于阈值的币种不做（多空都不做）
		// 持仓价值 = 持仓量 × 当前价格
		// 但现有持仓必须保留（需要决策是否平仓）
		// 💡 OI 門檻配置：用戶可根據風險偏好調整
		const minOIThresholdMillions = 15.0 // 可調整：15M(保守) / 10M(平衡) / 8M(寬鬆) / 5M(激進)

		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// 计算持仓价值（USD）= 持仓量 × 当前价格
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // 转换为百万美元单位
			if oiValueInMillions < minOIThresholdMillions {
				log.Printf("⚠️  %s 持仓价值过低(%.2fM USD < %.1fM)，跳过此币种 [持仓量:%.0f × 价格:%.4f]",
					symbol, oiValueInMillions, minOIThresholdMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		ctx.MarketDataMap[symbol] = data
	}

	// 加载OI Top数据（不影响主流程）
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// 标准化符号匹配
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates 根据账户状态计算需要分析的候选币种数量
func calculateMaxCandidates(ctx *Context) int {
	// ⚠️ 重要：限制候选币种数量，避免 Prompt 过大
	// 根据持仓数量动态调整：持仓越少，可以分析更多候选币
	const (
		maxCandidatesWhenEmpty    = 30 // 无持仓时最多分析30个候选币
		maxCandidatesWhenHolding1 = 25 // 持仓1个时最多分析25个候选币
		maxCandidatesWhenHolding2 = 20 // 持仓2个时最多分析20个候选币
		maxCandidatesWhenHolding3 = 15 // 持仓3个时最多分析15个候选币（避免 Prompt 过大）
	)

	positionCount := len(ctx.Positions)
	var maxCandidates int

	switch positionCount {
	case 0:
		maxCandidates = maxCandidatesWhenEmpty
	case 1:
		maxCandidates = maxCandidatesWhenHolding1
	case 2:
		maxCandidates = maxCandidatesWhenHolding2
	default: // 3+ 持仓
		maxCandidates = maxCandidatesWhenHolding3
	}

	// 返回实际候选币数量和上限中的较小值
	return min(len(ctx.CandidateCoins), maxCandidates)
}

// buildSystemPromptWithCustom 构建包含自定义内容的 System Prompt
func buildSystemPromptWithCustom(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string) string {
	// 如果覆盖基础prompt且有自定义prompt，只使用自定义prompt
	if overrideBase && customPrompt != "" {
		return customPrompt
	}

	// 获取基础prompt（使用指定的模板）
	basePrompt := buildSystemPrompt(templateName)

	// 如果没有自定义prompt，直接返回基础prompt
	if customPrompt == "" {
		return basePrompt
	}

	// 添加自定义prompt部分到基础prompt
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📌 个性化交易策略\n\n")
	sb.WriteString(customPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("注意: 以上个性化策略是对基础规则的补充，不能违背基础风险控制原则。\n")

	return sb.String()
}

// buildSystemPrompt 构建 System Prompt（使用模板+动态部分）
func buildSystemPrompt(templateName string) string {
	var sb strings.Builder

	// 1. 加载提示词模板（核心交易策略部分）
	if templateName == "" {
		templateName = "default" // 默认使用 default 模板
	}

	template, err := GetPromptTemplate(templateName)
	if err != nil {
		// 如果模板不存在，记录错误并使用 default
		log.Printf("⚠️  提示词模板 '%s' 不存在，使用 default: %v", templateName, err)
		template, err = GetPromptTemplate("default")
		if err != nil {
			// 如果连 default 都不存在，使用内置的简化版本
			log.Printf("❌ 无法加载任何提示词模板，使用内置简化版本")
			sb.WriteString("你是专业的加密货币交易AI。请根据市场数据做出交易决策。\n\n")
		} else {
			sb.WriteString(template.Content)
			sb.WriteString("\n\n")
		}
	} else {
		sb.WriteString(template.Content)
		sb.WriteString("\n\n")
	}

	// 2. 输出格式 - 动态生成
	sb.WriteString("# 可用动作与输出格式\n\n")
	sb.WriteString("**可用动作**\n")
	sb.WriteString("open_long/open_short, close_long/close_short, wait/hold, update_stop_loss, update_take_profit\n\n")
	sb.WriteString("**输出格式（严格执行）**\n")
	sb.WriteString("```xml\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("<!-- 四层框架分析 -->\n")
	sb.WriteString("1. 战略定调：...\n")
	sb.WriteString("2. 战役部署：...\n")
	sb.WriteString("3. 战术侦察：...\n")
	sb.WriteString("4. 精确瞄准：...\n")
	sb.WriteString("- 止损：基于ATR，[价格]\n")
	sb.WriteString("- 止盈：参考布林带上轨/下轨，确保盈亏比[数值]符合要求\n")
	sb.WriteString("最终检查清单：✅/❌\n")
	sb.WriteString("BTC状态：...\n")
	sb.WriteString("历史表现分析：... (根据历史数据评估策略健康度)\n")
	sb.WriteString("系统状态响应：... (说明如何响应收到的系统状态)\n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("```json\n")
	sb.WriteString("[\n")
	sb.WriteString("    {\n")
	sb.WriteString("        \"symbol\": \"BTCUSDT\",\n")
	sb.WriteString("        \"action\": \"open_long\",\n")
	sb.WriteString("        \"leverage\": 10,\n")
	sb.WriteString("        \"position_size_usd\": 150,\n")
	sb.WriteString("        \"stop_loss\": 104600,\n")
	sb.WriteString("        \"take_profit\": 107200,\n")
	sb.WriteString("        \"confidence\": 85,\n")
	sb.WriteString("        \"risk_usd\": 30,\n")
	sb.WriteString("        \"reasoning\": \"四层框架共振，...，盈亏比2.25符合要求\"\n")
	sb.WriteString("    }\n")
	sb.WriteString("]\n")
	sb.WriteString("```\n")
	sb.WriteString("</decision>\n")
	sb.WriteString("```\n\n")
	sb.WriteString("**字段要求**\n")
	sb.WriteString("- 开仓时必填:leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd, reasoning。\n")
	sb.WriteString("- 更新止损时必填: new_stop_loss\n")
	sb.WriteString("- 更新止盈时必填: new_take_profit\n")

	return sb.String()
}

// SystemStatus 系统状态信息
type SystemStatus struct {
	TradingMode       string    // "正常模式" 或 "保守模式"
	ModeReason        string    // 模式原因：初始状态/连续亏损触发/夏普比率触发/盈利复位触发
	ConsecutiveLosses int       // 连续亏损计数（过去30分钟内）
	LastTradeResult   string    // "盈利" / "亏损" / "无"
	NextOpenTime      time.Time // 下次可开仓时间
	SharpeRatio       float64   // 夏普比率
}

// calculateSystemStatus 计算系统状态（根据 rational_data_driven.txt 新规则）
func calculateSystemStatus(ctx *Context) SystemStatus {
	status := SystemStatus{
		TradingMode:       "正常模式",
		ModeReason:        "初始状态",
		ConsecutiveLosses: 0,
		LastTradeResult:   "无",
		NextOpenTime:      time.Now(),
		SharpeRatio:       0.0,
	}

	// 分析历史表现以确定交易模式和连续亏损
	if ctx.Performance != nil {
		type PerformanceData struct {
			TotalTrades   int     `json:"total_trades"`
			WinningTrades int     `json:"winning_trades"`
			LosingTrades  int     `json:"losing_trades"`
			WinRate       float64 `json:"win_rate"`
			AvgWinPct     float64 `json:"avg_win_pct"`
			AvgLossPct    float64 `json:"avg_loss_pct"`
			ProfitFactor  float64 `json:"profit_factor"`
			SharpeRatio   float64 `json:"sharpe_ratio"`
			RecentTrades  []struct {
				PnLPct    float64   `json:"pnl_pct"`
				CloseTime time.Time `json:"close_time"`
			} `json:"recent_trades"`
		}

		var perfData PerformanceData
		if jsonData, err := json.Marshal(ctx.Performance); err == nil {
			if err := json.Unmarshal(jsonData, &perfData); err == nil {
				now := time.Now()
				thirtyMinutesAgo := now.Add(-30 * time.Minute)

				// 保存夏普比率
				status.SharpeRatio = perfData.SharpeRatio

				// 计算连续亏损（过去30分钟内，最近20笔交易）
				consecutiveLosses := 0
				for _, trade := range perfData.RecentTrades {
					// 只统计最近30分钟内的亏损交易
					if !trade.CloseTime.IsZero() && trade.CloseTime.Before(thirtyMinutesAgo) {
						break // 超过30分钟，停止统计
					}
					if trade.PnLPct < 0 {
						consecutiveLosses++
					} else {
						break // 遇到盈利交易就停止计数
					}
				}
				status.ConsecutiveLosses = consecutiveLosses

				// 确定最后交易结果
				if len(perfData.RecentTrades) > 0 {
					lastTrade := perfData.RecentTrades[0]
					if lastTrade.PnLPct > 0 {
						status.LastTradeResult = "盈利"
					} else if lastTrade.PnLPct < 0 {
						status.LastTradeResult = "亏损"
					}
				}

				// 判断交易模式（根据新规则）
				// 优先级：夏普比率触发 > 连续亏损触发 > 盈利复位触发
				// 夏普比率触发是"熔断机制"，一旦触发必须保持保守模式，不允许盈利复位
				isConservativeMode := false
				modeReason := "初始状态"

				// 1. 检查夏普比率触发（正常→保守）- 最高优先级，熔断机制
				if perfData.SharpeRatio < -0.5 {
					isConservativeMode = true
					modeReason = "夏普比率触发"
					// 熔断机制：无论是否有盈利交易，都必须保持保守模式
				} else {
					// 2. 检查连续亏损触发（正常→保守）：30分钟内2次连续亏损
					if consecutiveLosses >= 2 {
						isConservativeMode = true
						modeReason = "连续亏损触发"
					}

					// 3. 检查盈利复位（保守→正常）：完成1笔盈利交易
					// 注意：盈利复位只能在"连续亏损触发"的保守模式下生效，不能在"夏普比率触发"下生效
					if isConservativeMode && modeReason == "连续亏损触发" && len(perfData.RecentTrades) > 0 {
						lastTrade := perfData.RecentTrades[0]
						if lastTrade.PnLPct > 0 {
							// 有盈利交易，复位到正常模式（仅针对连续亏损触发）
							isConservativeMode = false
							modeReason = "盈利复位触发"
						}
					}
				}

				// 设置交易模式和原因
				if isConservativeMode {
					status.TradingMode = "保守模式"
					status.ModeReason = modeReason
				} else {
					status.TradingMode = "正常模式"
					status.ModeReason = modeReason
				}

				// 注意：下次可开仓时间已改为按币种计算（在 calculateSymbolRhythm 中实现）
				// 系统级别的 NextOpenTime 不再计算，因为交易节奏控制是按币种进行的
			}
		}
	}

	// 系统级别的 NextOpenTime 不再使用，因为交易节奏控制已改为按币种计算
	status.NextOpenTime = time.Now() // 设置为当前时间，表示系统级别无限制

	return status
}

// formatMultiTimeframeTable 格式化多周期战术数据表格
func formatMultiTimeframeTable(symbol string, data *market.Data) string {
	var sb strings.Builder

	// 获取各周期的最新值（数组最后一个元素）
	getLatest := func(arr []float64) float64 {
		if len(arr) > 0 {
			return arr[len(arr)-1]
		}
		return 0
	}

	// 4H数据
	var close4H, ema4H, macd4H, rsi4H, cci4H, bbUpper4H, bbMiddle4H, bbLower4H, takerBuy4H, bsr4H float64
	if data.LongerTermContext != nil {
		close4H = getLatest(data.LongerTermContext.ClosePrices)
		ema4H = getLatest(data.LongerTermContext.EMA20Values)
		macd4H = getLatest(data.LongerTermContext.MACDValues)
		rsi4H = getLatest(data.LongerTermContext.RSI7Values)
		cci4H = getLatest(data.LongerTermContext.CCI20Values)
		bbUpper4H = getLatest(data.LongerTermContext.BBUpperValues)
		bbMiddle4H = getLatest(data.LongerTermContext.BBMiddleValues)
		bbLower4H = getLatest(data.LongerTermContext.BBLowerValues)
		takerBuy4H = getLatest(data.LongerTermContext.TakerBuyRatios)
		bsr4H = getLatest(data.LongerTermContext.BuySellRatios)
	}

	// 1H数据
	var close1H, ema1H, macd1H, rsi1H, cci1H, bbUpper1H, bbMiddle1H, bbLower1H, takerBuy1H, bsr1H float64
	if data.Series1h != nil {
		close1H = getLatest(data.Series1h.ClosePrices)
		ema1H = getLatest(data.Series1h.EMA20Values)
		macd1H = getLatest(data.Series1h.MACDValues)
		rsi1H = getLatest(data.Series1h.RSI7Values)
		cci1H = getLatest(data.Series1h.CCI20Values)
		bbUpper1H = getLatest(data.Series1h.BBUpperValues)
		bbMiddle1H = getLatest(data.Series1h.BBMiddleValues)
		bbLower1H = getLatest(data.Series1h.BBLowerValues)
		takerBuy1H = getLatest(data.Series1h.TakerBuyRatios)
		bsr1H = getLatest(data.Series1h.BuySellRatios)
	}

	// 15m数据
	var close15m, ema15m, macd15m, rsi15m, cci15m, bbUpper15m, bbMiddle15m, bbLower15m, takerBuy15m, bsr15m float64
	if data.Series15m != nil {
		close15m = getLatest(data.Series15m.ClosePrices)
		ema15m = getLatest(data.Series15m.EMA20Values)
		macd15m = getLatest(data.Series15m.MACDValues)
		rsi15m = getLatest(data.Series15m.RSI7Values)
		cci15m = getLatest(data.Series15m.CCI20Values)
		bbUpper15m = getLatest(data.Series15m.BBUpperValues)
		bbMiddle15m = getLatest(data.Series15m.BBMiddleValues)
		bbLower15m = getLatest(data.Series15m.BBLowerValues)
		takerBuy15m = getLatest(data.Series15m.TakerBuyRatios)
		bsr15m = getLatest(data.Series15m.BuySellRatios)
	}

	// 3m数据
	var close3m, ema3m, macd3m, rsi3m, cci3m, bbUpper3m, bbMiddle3m, bbLower3m, takerBuy3m, bsr3m float64
	if data.IntradaySeries != nil {
		close3m = getLatest(data.IntradaySeries.ClosePrices)
		ema3m = getLatest(data.IntradaySeries.EMA20Values)
		macd3m = getLatest(data.IntradaySeries.MACDValues)
		rsi3m = getLatest(data.IntradaySeries.RSI7Values)
		cci3m = getLatest(data.IntradaySeries.CCI20Values)
		bbUpper3m = getLatest(data.IntradaySeries.BBUpperValues)
		bbMiddle3m = getLatest(data.IntradaySeries.BBMiddleValues)
		bbLower3m = getLatest(data.IntradaySeries.BBLowerValues)
		takerBuy3m = getLatest(data.IntradaySeries.TakerBuyRatios)
		bsr3m = getLatest(data.IntradaySeries.BuySellRatios)
	}

	// 格式化表格
	sb.WriteString("周期 | 收盘价 | EMA20 | MACD | RSI(7) | CCI(20) | BB上轨 | BB中轨 | BB下轨 | TakerBuyRatio | BSR\n")
	sb.WriteString(":--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---\n")
	sb.WriteString(fmt.Sprintf("4H | %.4f | %.4f | %.4f | %.2f | %.2f | %.4f | %.4f | %.4f | %.3f | %.3f\n",
		close4H, ema4H, macd4H, rsi4H, cci4H, bbUpper4H, bbMiddle4H, bbLower4H, takerBuy4H, bsr4H))
	sb.WriteString(fmt.Sprintf("1H | %.4f | %.4f | %.4f | %.2f | %.2f | %.4f | %.4f | %.4f | %.3f | %.3f\n",
		close1H, ema1H, macd1H, rsi1H, cci1H, bbUpper1H, bbMiddle1H, bbLower1H, takerBuy1H, bsr1H))
	sb.WriteString(fmt.Sprintf("15m | %.4f | %.4f | %.4f | %.2f | %.2f | %.4f | %.4f | %.4f | %.3f | %.3f\n",
		close15m, ema15m, macd15m, rsi15m, cci15m, bbUpper15m, bbMiddle15m, bbLower15m, takerBuy15m, bsr15m))
	sb.WriteString(fmt.Sprintf("3m | %.4f | %.4f | %.4f | %.2f | %.2f | %.4f | %.4f | %.4f | %.3f | %.3f\n",
		close3m, ema3m, macd3m, rsi3m, cci3m, bbUpper3m, bbMiddle3m, bbLower3m, takerBuy3m, bsr3m))

	return sb.String()
}

// buildUserPrompt 构建 User Prompt（动态数据）- 新格式匹配rational_data_driven.txt策略
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// 计算系统状态
	systemStatus := calculateSystemStatus(ctx)

	// 时间信息
	sb.WriteString(fmt.Sprintf("当前时间: %s | 周期: #%d | 已运行: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// 系统状态
	sb.WriteString("# 系统状态\n")
	sb.WriteString(fmt.Sprintf("- **交易模式**: %s\n", systemStatus.TradingMode))
	sb.WriteString(fmt.Sprintf("- **模式原因**: %s\n", systemStatus.ModeReason))
	sb.WriteString(fmt.Sprintf("- **连续亏损计数**: %d (过去30分钟内)\n", systemStatus.ConsecutiveLosses))
	sb.WriteString(fmt.Sprintf("- **上次交易结果**: %s\n", systemStatus.LastTradeResult))
	sb.WriteString("\n")
	// 注意：下次可开仓时间已改为按币种显示（在每个币种的"状态"部分）

	// 上轮指令执行反馈
	if ctx.LastExecutionFeedback != nil && ctx.LastExecutionFeedback.HasRejected {
		sb.WriteString("# 上轮指令执行反馈\n")
		for _, rejected := range ctx.LastExecutionFeedback.RejectedDecisions {
			var actionDesc string
			if rejected.Leverage > 0 {
				actionDesc = fmt.Sprintf("%s %s @杠杆%dx", rejected.Action, rejected.Symbol, rejected.Leverage)
			} else {
				actionDesc = fmt.Sprintf("%s %s", rejected.Action, rejected.Symbol)
			}
			sb.WriteString("- **状态**: 拒绝\n")
			sb.WriteString(fmt.Sprintf("- **详情**: `%s` 被拒绝，原因: `%s`\n", actionDesc, rejected.Reason))
		}
	} else {
		sb.WriteString("# 上轮指令执行反馈\n")
		sb.WriteString("- **状态**: 无拒绝记录\n")
	}
	sb.WriteString("\n")

	// 账户概览
	sb.WriteString("# 账户概览\n")
	sb.WriteString(fmt.Sprintf("- 净值: %.2f USDT\n", ctx.Account.TotalEquity))
	balancePct := 0.0
	if ctx.Account.TotalEquity > 0 {
		balancePct = (ctx.Account.AvailableBalance / ctx.Account.TotalEquity) * 100
	}
	sb.WriteString(fmt.Sprintf("- 可用余额: %.2f USDT (%.1f%%)\n", ctx.Account.AvailableBalance, balancePct))
	if len(ctx.Positions) > 0 {
		sb.WriteString(fmt.Sprintf("- 当前持仓: %d 个\n", len(ctx.Positions)))
		for _, pos := range ctx.Positions {
			// 计算仓位价值
			positionValue := math.Abs(pos.Quantity) * pos.MarkPrice

			// 计算持仓时长
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60) // 转换为分钟
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf("%d分", durationMin)
				} else {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					if durationMinRemainder > 0 {
						holdingDuration = fmt.Sprintf("%d小时%d分", durationHour, durationMinRemainder)
					} else {
						holdingDuration = fmt.Sprintf("%d小时", durationHour)
					}
				}
			} else {
				holdingDuration = "未知"
			}

			sb.WriteString(fmt.Sprintf("  - %s | %s | 入场价%.4f | 当前价%.4f | 数量%.4f | 仓位价值%.2f USDT | 盈亏%+.2f%% | 盈亏金额%+.2f USDT | 最高收益率%.2f%% | 杠杆%dx | 保证金%.0f | 强平价%.4f | 持仓时长%s\n",
				pos.Symbol, strings.ToUpper(pos.Side), pos.EntryPrice, pos.MarkPrice, pos.Quantity,
				positionValue, pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct,
				pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))
		}
	} else {
		sb.WriteString("- 当前持仓: 0 个\n")
	}
	sb.WriteString(fmt.Sprintf("- 浮动盈亏: %.2f USDT\n", ctx.Account.UnrealizedPnL))

	// 历史表现摘要
	sb.WriteString("- 历史表现摘要 (最近20笔): \n")
	if ctx.Performance != nil {
		type PerformanceData struct {
			TotalTrades   int     `json:"total_trades"`
			WinningTrades int     `json:"winning_trades"`
			LosingTrades  int     `json:"losing_trades"`
			WinRate       float64 `json:"win_rate"`
			AvgWinPct     float64 `json:"avg_win_pct"`
			AvgLossPct    float64 `json:"avg_loss_pct"`
			ProfitFactor  float64 `json:"profit_factor"`
			SharpeRatio   float64 `json:"sharpe_ratio"`
		}

		var perfData PerformanceData
		if jsonData, err := json.Marshal(ctx.Performance); err == nil {
			if err := json.Unmarshal(jsonData, &perfData); err == nil {
				if perfData.TotalTrades > 0 {
					sb.WriteString(fmt.Sprintf("  - 胜率: %.1f%% \n", perfData.WinRate))
					sb.WriteString(fmt.Sprintf("  - 平均盈利: +%.2f%% (盈利交易均值)\n", perfData.AvgWinPct))
					sb.WriteString(fmt.Sprintf("  - 平均亏损: %.2f%% (亏损交易均值)\n", perfData.AvgLossPct))
					sb.WriteString(fmt.Sprintf("  - 盈亏比: %.2f\n", perfData.ProfitFactor))
					sb.WriteString(fmt.Sprintf("  - 夏普比率: %.2f\n", perfData.SharpeRatio))
				} else {
					sb.WriteString("  - 暂无历史交易记录\n")
				}
			}
		}
	} else {
		sb.WriteString("  - 暂无历史交易记录\n")
	}
	sb.WriteString("\n")

	// 系统执行限制
	sb.WriteString("# 系统执行限制\n")
	sb.WriteString("- 最大持仓币种数: 3\n")
	sb.WriteString(fmt.Sprintf("- 单币种最大杠杆: 山寨币 %dx | BTC/ETH %dx\n", ctx.AltcoinLeverage, ctx.BTCETHLeverage))
	sb.WriteString("- 账户最大保证金使用率: 90%\n")
	sb.WriteString("- 最小开仓名义价值 (AI必须严格遵守):\n")
	sb.WriteString("  - BTCUSDT, ETHUSDT: ≥ 60.00 USDT\n")
	sb.WriteString("  - 其他所有山寨币: ≥ 12.00 USDT\n")
	sb.WriteString("- **说明**: 你下达的指令必须符合以上规则，否则将被执行层拒绝。**特别注意**：计算出的`position_size_usd`必须大于或等于对应标的的最小开仓名义价值。\n")
	sb.WriteString("\n")

	// 标的币种数据
	sb.WriteString("# 标的币种数据\n\n")

	// 计算每个币种的节奏锁定状态
	type SymbolRhythmStatus struct {
		IsLocked     bool
		LockUntil    time.Time
		NextOpenTime time.Time
	}

	calculateSymbolRhythm := func(symbol string, ctx *Context) SymbolRhythmStatus {
		now := time.Now()
		status := SymbolRhythmStatus{
			IsLocked:     false,
			LockUntil:    now,
			NextOpenTime: now,
		}

		// 从历史表现中获取该币种的交易记录
		if ctx.Performance != nil {
			type PerformanceData struct {
				RecentTrades []struct {
					Symbol    string    `json:"symbol"`
					PnLPct    float64   `json:"pnl_pct"`
					OpenTime  time.Time `json:"open_time"`
					CloseTime time.Time `json:"close_time"`
				} `json:"recent_trades"`
			}

			var perfData PerformanceData
			if jsonData, err := json.Marshal(ctx.Performance); err == nil {
				if err := json.Unmarshal(jsonData, &perfData); err == nil {
					// 查找该币种最近的交易记录（按时间倒序，最新的在前）
					var lastOpenTime, lastCloseTime time.Time
					var lastPnLPct float64
					var hasOpenTrade bool

					// 检查是否有持仓（正在进行的15分钟周期）
					hasPosition := false
					for _, pos := range ctx.Positions {
						if pos.Symbol == symbol {
							hasPosition = true
							// 如果有持仓，从持仓开始时间计算15分钟周期
							if pos.UpdateTime > 0 {
								lastOpenTime = time.Unix(pos.UpdateTime/1000, 0)
								hasOpenTrade = true
							}
							break
						}
					}

					// 查找最近的交易记录
					for _, trade := range perfData.RecentTrades {
						if trade.Symbol == symbol {
							if !hasOpenTrade {
								lastOpenTime = trade.OpenTime
								lastCloseTime = trade.CloseTime
								lastPnLPct = trade.PnLPct
								hasOpenTrade = true
							}
							break // 只取最新的
						}
					}

					if hasOpenTrade {
						var nextOpenTime time.Time

						if hasPosition {
							// 有持仓：15分钟周期从开仓时间开始
							cycleEndTime := lastOpenTime.Add(15 * time.Minute)
							if cycleEndTime.After(now) {
								status.IsLocked = true
								status.LockUntil = cycleEndTime
								nextOpenTime = cycleEndTime
							} else {
								nextOpenTime = now
							}
						} else if !lastCloseTime.IsZero() {
							// 无持仓：检查提前平仓需要补足的时间
							cycleEndTime := lastOpenTime.Add(15 * time.Minute)

							// 计算交易后冷却期（从平仓时间开始）
							var cooldownPeriod time.Duration
							if lastPnLPct < 0 {
								cooldownPeriod = 3 * time.Minute // 亏损后3分钟
							} else if lastPnLPct > 0 {
								cooldownPeriod = 1 * time.Minute // 盈利后1分钟
							}

							// 计算最小开仓间隔（6分钟）- 从开仓时间开始算
							minIntervalTime := lastOpenTime.Add(6 * time.Minute)

							// 计算下次可开仓时间（取最大值）
							candidates := []time.Time{
								lastCloseTime.Add(cooldownPeriod), // 交易后冷却期（从平仓时间开始）
								minIntervalTime,                   // 最小开仓间隔（从开仓时间开始）
							}

							// 如果提前平仓（平仓时间早于15分钟周期结束时间），需要补足剩余周期时间
							if lastCloseTime.Before(cycleEndTime) {
								// 从平仓时间开始补足剩余周期时间
								remainingCycleTime := cycleEndTime.Sub(lastCloseTime)
								candidates = append(candidates, lastCloseTime.Add(remainingCycleTime))
							}

							nextOpenTime = now
							for _, candidate := range candidates {
								if candidate.After(nextOpenTime) {
									nextOpenTime = candidate
								}
							}

							// 如果下次可开仓时间在未来，说明被锁定
							if nextOpenTime.After(now) {
								status.IsLocked = true
								status.LockUntil = nextOpenTime
							}
						}

						status.NextOpenTime = nextOpenTime
					}
				}
			}
		}

		return status
	}

	// 辅助函数：格式化币种的战场环境
	formatBattlefieldEnv := func(symbol string, data *market.Data, ctx *Context) string {
		var sb strings.Builder
		// 获取4H ATR
		atr4H := 0.0
		if data.LongerTermContext != nil {
			atr4H = data.LongerTermContext.ATR14
		}
		// 获取1H OI变化
		oi1HChange := 0.0
		if data.OpenInterest != nil {
			oi1HChange = data.OpenInterest.DeltaPercent
		}

		// 计算节奏锁定状态
		rhythmStatus := calculateSymbolRhythm(symbol, ctx)

		sb.WriteString("**战场环境**:\n")
		sb.WriteString(fmt.Sprintf("- 当前价格: %.4f\n", data.CurrentPrice))
		sb.WriteString(fmt.Sprintf("- 4H_ATR(14): %.4f\n", atr4H))
		sb.WriteString(fmt.Sprintf("- 资金费率: %.6f\n", data.FundingRate))
		sb.WriteString(fmt.Sprintf("- 1H_OI_Change: %.2f%%\n", oi1HChange))
		sb.WriteString("\n")

		sb.WriteString("**状态**:\n")
		if rhythmStatus.IsLocked {
			sb.WriteString(fmt.Sprintf("- 节奏锁定: 是(%s)\n", rhythmStatus.LockUntil.Format("2006-01-02 15:04:05")))
		} else {
			sb.WriteString("- 节奏锁定: 否\n")
		}

		if rhythmStatus.NextOpenTime.After(time.Now()) {
			sb.WriteString(fmt.Sprintf("- **下次可开仓时间**: %s\n", rhythmStatus.NextOpenTime.Format("2006-01-02 15:04:05")))
		} else {
			sb.WriteString("- **下次可开仓时间**: 立即\n")
		}
		sb.WriteString("\n")

		return sb.String()
	}

	// 辅助函数：输出币种数据（战场环境 + 多周期战术数据）
	// 如果没有数据，则不输出任何内容
	outputSymbolData := func(symbol string, data *market.Data) {
		if data == nil {
			return // 没有数据就不输出
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", symbol))
		sb.WriteString(formatBattlefieldEnv(symbol, data, ctx))
		sb.WriteString("**多周期战术数据**:\n")
		sb.WriteString(formatMultiTimeframeTable(symbol, data))
		sb.WriteString("\n")
	}

	// 记录已输出的币种，避免重复
	displayedSymbols := make(map[string]bool)

	// 1. 先输出BTCUSDT（必须，放在第一位，即使没有数据也要输出）
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		outputSymbolData("BTCUSDT", btcData)
		displayedSymbols["BTCUSDT"] = true
	}

	// 2. 输出所有候选币种（包括持仓币种和候选币种）
	// 收集所有需要输出的币种
	allSymbols := make([]string, 0)

	// 添加持仓币种
	for _, pos := range ctx.Positions {
		if !displayedSymbols[pos.Symbol] {
			allSymbols = append(allSymbols, pos.Symbol)
			displayedSymbols[pos.Symbol] = true
		}
	}

	// 添加候选币种
	for _, coin := range ctx.CandidateCoins {
		if !displayedSymbols[coin.Symbol] {
			allSymbols = append(allSymbols, coin.Symbol)
			displayedSymbols[coin.Symbol] = true
		}
	}

	// 输出所有币种数据（只输出有数据的币种）
	for _, symbol := range allSymbols {
		if data, hasData := ctx.MarketDataMap[symbol]; hasData {
			outputSymbolData(symbol, data)
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("**指令**： 现在，基于以上数据，进行完全自主的交易决策，并输出 `思维链` + `JSON`。\n")

	return sb.String()
}

// parseFullDecisionResponse 解析AI的完整决策响应
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int) (*FullDecision, error) {
	// 1. 提取思维链
	cotTrace := extractCoTTrace(aiResponse)

	// 2. 提取并验证JSON决策列表（同时获取原始JSON）
	rawDecisionJSON, decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:        cotTrace,
			Decisions:       []Decision{},
			RawDecisionJSON: rawDecisionJSON, // 即使验证失败，也保存原始JSON
		}, fmt.Errorf("提取决策失败: %w", err)
	}

	// 3. 规范化决策（自动修正超出限制的杠杆和仓位大小）
	normalizeDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage)

	// 4. 验证决策
	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
		return &FullDecision{
			CoTTrace:        cotTrace,
			Decisions:       decisions,
			RawDecisionJSON: rawDecisionJSON, // 保存原始JSON
		}, fmt.Errorf("决策验证失败: %w", err)
	}

	return &FullDecision{
		CoTTrace:        cotTrace,
		Decisions:       decisions,
		RawDecisionJSON: rawDecisionJSON, // 保存原始JSON
	}, nil
}

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
	// 方法1: 优先尝试提取 <reasoning> 标签内容
	if match := reReasoningTag.FindStringSubmatch(response); match != nil && len(match) > 1 {
		log.Printf("✓ 使用 <reasoning> 标签提取思维链")
		return strings.TrimSpace(match[1])
	}

	// 方法2: 如果没有 <reasoning> 标签，但有 <decision> 标签，提取 <decision> 之前的内容
	if decisionIdx := strings.Index(response, "<decision>"); decisionIdx > 0 {
		log.Printf("✓ 提取 <decision> 标签之前的内容作为思维链")
		return strings.TrimSpace(response[:decisionIdx])
	}

	// 方法3: 后备方案 - 查找JSON数组的开始位置
	jsonStart := strings.Index(response, "[")
	if jsonStart > 0 {
		log.Printf("⚠️  使用旧版格式（[ 字符分离）提取思维链")
		return strings.TrimSpace(response[:jsonStart])
	}

	// 如果找不到任何标记，整个响应都是思维链
	return strings.TrimSpace(response)
}

// extractDecisions 提取JSON决策列表，同时返回原始JSON（用于保存AI原始输出）
// 返回值: (原始JSON, 解析后的决策列表, 错误)
func extractDecisions(response string) (string, []Decision, error) {
	// 预清洗：去零宽/BOM
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)

	// 🔧 关键修复 (Critical Fix)：在正则匹配之前就先修复全角字符！
	// 否则正则表达式 \[ 无法匹配全角的 ［
	s = fixMissingQuotes(s)

	// 方法1: 优先尝试从 <decision> 标签中提取
	var jsonPart string
	if match := reDecisionTag.FindStringSubmatch(s); match != nil && len(match) > 1 {
		jsonPart = strings.TrimSpace(match[1])
		log.Printf("✓ 使用 <decision> 标签提取JSON")
	} else {
		// 后备方案：使用整个响应
		jsonPart = s
		log.Printf("⚠️  未找到 <decision> 标签，使用全文搜索JSON")
	}

	// 修复 jsonPart 中的全角字符
	jsonPart = fixMissingQuotes(jsonPart)

	// 1) 优先从 ```json 代码块中提取
	if m := reJSONFence.FindStringSubmatch(jsonPart); m != nil && len(m) > 1 {
		rawJSON := strings.TrimSpace(m[1]) // 保存原始JSON（在验证和解析之前）
		jsonContent := rawJSON
		jsonContent = compactArrayOpen(jsonContent) // 把 "[ {" 规整为 "[{"
		jsonContent = fixMissingQuotes(jsonContent) // 二次修复（防止 regex 提取后还有残留全角）
		if err := validateJSONFormat(jsonContent); err != nil {
			return rawJSON, nil, fmt.Errorf("JSON格式验证失败: %w\nJSON内容: %s\n完整响应:\n%s", err, jsonContent, response)
		}
		var decisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			return rawJSON, nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
		}
		return rawJSON, decisions, nil
	}

	// 2) 退而求其次 (Fallback)：全文寻找首个对象数组
	// 注意：此时 jsonPart 已经过 fixMissingQuotes()，全角字符已转换为半角
	rawJSON := strings.TrimSpace(reJSONArray.FindString(jsonPart))
	if rawJSON == "" {
		// 🔧 安全回退 (Safe Fallback)：当AI只输出思维链没有JSON时，生成保底决策（避免系统崩溃）
		log.Printf("⚠️  [SafeFallback] AI未输出JSON决策，进入安全等待模式 (AI response without JSON, entering safe wait mode)")

		// 提取思维链摘要（最多 240 字符）
		cotSummary := jsonPart
		if len(cotSummary) > 240 {
			cotSummary = cotSummary[:240] + "..."
		}

		// 生成保底决策：所有币种进入 wait 状态
		fallbackDecision := Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: fmt.Sprintf("模型未输出结构化JSON决策，进入安全等待；摘要：%s", cotSummary),
		}

		return "", []Decision{fallbackDecision}, nil
	}

	// 🔧 规整格式（此时全角字符已在前面修复过）
	jsonContent := rawJSON
	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent) // 二次修复（防止 regex 提取后还有残留全角）

	// 🔧 验证 JSON 格式（检测常见错误）
	if err := validateJSONFormat(jsonContent); err != nil {
		return rawJSON, nil, fmt.Errorf("JSON格式验证失败: %w\nJSON内容: %s\n完整响应:\n%s", err, jsonContent, response)
	}

	// 解析JSON
	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return rawJSON, nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
	}

	return rawJSON, decisions, nil
}

// fixMissingQuotes 替换中文引号和全角字符为英文引号和半角字符（避免AI输出全角JSON字符导致解析失败）
func fixMissingQuotes(jsonStr string) string {
	// 替换中文引号
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '

	// ⚠️ 替换全角括号、冒号、逗号（防止AI输出全角JSON字符）
	jsonStr = strings.ReplaceAll(jsonStr, "［", "[") // U+FF3B 全角左方括号
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]") // U+FF3D 全角右方括号
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{") // U+FF5B 全角左花括号
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}") // U+FF5D 全角右花括号
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":") // U+FF1A 全角冒号
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",") // U+FF0C 全角逗号

	// ⚠️ 替换CJK标点符号（AI在中文上下文中也可能输出这些）
	jsonStr = strings.ReplaceAll(jsonStr, "【", "[") // CJK左方头括号 U+3010
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]") // CJK右方头括号 U+3011
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[") // CJK左龟壳括号 U+3014
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]") // CJK右龟壳括号 U+3015
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",") // CJK顿号 U+3001

	// ⚠️ 替换全角空格为半角空格（JSON中不应该有全角空格）
	jsonStr = strings.ReplaceAll(jsonStr, "　", " ") // U+3000 全角空格

	return jsonStr
}

// validateJSONFormat 验证 JSON 格式，检测常见错误
func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	// 允许 [ 和 { 之间存在任意空白（含零宽）
	if !reArrayHead.MatchString(trimmed) {
		// 检查是否是纯数字/范围数组（常见错误）
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("不是有效的决策数组（必须包含对象 {}），实际内容: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON 必须以 [{ 开头（允许空白），实际: %s", trimmed[:min(20, len(trimmed))])
	}

	// 检查数字值中是否包含范围符号 ~（LLM 常见错误）
	// 注意：只检查数字值，不检查字符串内容（reasoning 等字段可能包含 ~）
	// 使用正则表达式匹配：在JSON值位置（冒号后、引号外）出现的 ~数字 或 数字~
	// 模式：": ~数字" 或 ": 数字~" 或 ":~数字" 或 ":数字~"
	// 排除字符串值中的 ~（字符串值在引号内）
	reTildeInNumber := regexp.MustCompile(`:\s*~[\d.eE+-]+|:\s*[\d.eE+-]+~`)
	if reTildeInNumber.MatchString(jsonStr) {
		return fmt.Errorf("JSON 数字值中不可包含范围符号 ~，所有数字必须是精确的单一值")
	}

	// 检查是否包含千位分隔符（如 98,000）
	// 使用简单的模式匹配：数字+逗号+3位数字
	for i := 0; i < len(jsonStr)-4; i++ {
		if jsonStr[i] >= '0' && jsonStr[i] <= '9' &&
			jsonStr[i+1] == ',' &&
			jsonStr[i+2] >= '0' && jsonStr[i+2] <= '9' &&
			jsonStr[i+3] >= '0' && jsonStr[i+3] <= '9' &&
			jsonStr[i+4] >= '0' && jsonStr[i+4] <= '9' {
			return fmt.Errorf("JSON 数字不可包含千位分隔符逗号，发现: %s", jsonStr[i:min(i+10, len(jsonStr))])
		}
	}

	return nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// removeInvisibleRunes 去除零宽字符和 BOM，避免肉眼看不见的前缀破坏校验
func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

// compactArrayOpen 规整开头的 "[ {" → "[{"
func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// normalizeDecisions 规范化决策（自动修正超出限制的杠杆和仓位大小）
func normalizeDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) {
	for i := range decisions {
		d := &decisions[i]

		// 只处理开仓操作
		if d.Action != "open_long" && d.Action != "open_short" {
			continue
		}

		// 根据币种确定最大杠杆和最大仓位价值
		maxLeverage := altcoinLeverage
		maxPositionValue := accountEquity * 1.5
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage
			maxPositionValue = accountEquity * 10
		}

		// 修正杠杆
		if d.Leverage > maxLeverage {
			log.Printf("⚠️  决策 #%d (%s): AI决策杠杆 %dx 超过配置上限 %dx，自动调整为 %dx",
				i+1, d.Symbol, d.Leverage, maxLeverage, maxLeverage)
			d.Leverage = maxLeverage
		} else if d.Leverage <= 0 {
			log.Printf("⚠️  决策 #%d (%s): AI决策杠杆 %d 无效，自动调整为 %dx",
				i+1, d.Symbol, d.Leverage, maxLeverage)
			d.Leverage = maxLeverage
		}

		// 修正仓位大小
		tolerance := maxPositionValue * 0.01 // 1%容差
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			log.Printf("⚠️  决策 #%d (%s): AI决策仓位大小 %.0f USDT 超过配置上限 %.0f USDT，自动调整为 %.0f USDT",
				i+1, d.Symbol, d.PositionSizeUSD, maxPositionValue, maxPositionValue)
			d.PositionSizeUSD = maxPositionValue
		} else if d.PositionSizeUSD <= 0 {
			// 如果仓位大小无效，设置为最小合理值（账户净值的10%）
			minPositionValue := accountEquity * 0.1
			log.Printf("⚠️  决策 #%d (%s): AI决策仓位大小 %.2f USDT 无效，自动调整为 %.0f USDT（账户净值的10%%）",
				i+1, d.Symbol, d.PositionSizeUSD, minPositionValue)
			d.PositionSizeUSD = minPositionValue
		}
	}
}

// validateDecisions 验证所有决策（需要账户信息和杠杆配置）
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
			return fmt.Errorf("决策 #%d 验证失败: %w", i+1, err)
		}
	}
	return nil
}

// validateDecision 验证单个决策的有效性
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	// 验证action
	validActions := map[string]bool{
		"open_long":          true,
		"open_short":         true,
		"close_long":         true,
		"close_short":        true,
		"update_stop_loss":   true,
		"update_take_profit": true,
		"partial_close":      true,
		"hold":               true,
		"wait":               true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("无效的action: %s", d.Action)
	}

	// 开仓操作必须提供完整参数
	if d.Action == "open_long" || d.Action == "open_short" {
		// 根据币种使用配置的杠杆上限
		maxLeverage := altcoinLeverage          // 山寨币使用配置的杠杆
		maxPositionValue := accountEquity * 1.5 // 山寨币最多1.5倍账户净值
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage          // BTC和ETH使用配置的杠杆
			maxPositionValue = accountEquity * 10 // BTC/ETH最多10倍账户净值
		}

		// ✅ Fallback 机制：杠杆超限时自动修正为上限值（而不是直接拒绝决策）
		if d.Leverage <= 0 {
			return fmt.Errorf("杠杆必须大于0: %d", d.Leverage)
		}
		if d.Leverage > maxLeverage {
			log.Printf("⚠️  [Leverage Fallback] %s 杠杆超限 (%dx > %dx)，自动调整为上限值 %dx",
				d.Symbol, d.Leverage, maxLeverage, maxLeverage)
			d.Leverage = maxLeverage // 自动修正为上限值
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("仓位大小必须大于0: %.2f", d.PositionSizeUSD)
		}

		// ✅ 验证最小开仓金额（防止数量格式化为 0 的错误）
		// Binance 最小名义价值 10 USDT + 安全边际
		const minPositionSizeGeneral = 12.0 // 10 + 20% 安全边际
		const minPositionSizeBTCETH = 60.0  // BTC/ETH 因价格高和精度限制需要更大金额（更灵活）

		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			if d.PositionSizeUSD < minPositionSizeBTCETH {
				return fmt.Errorf("%s 开仓金额过小(%.2f USDT)，必须≥%.2f USDT（因价格高且精度限制，避免数量四舍五入为0）", d.Symbol, d.PositionSizeUSD, minPositionSizeBTCETH)
			}
		} else {
			if d.PositionSizeUSD < minPositionSizeGeneral {
				return fmt.Errorf("开仓金额过小(%.2f USDT)，必须≥%.2f USDT（Binance 最小名义价值要求）", d.PositionSizeUSD, minPositionSizeGeneral)
			}
		}

		// 验证仓位价值上限（加1%容差以避免浮点数精度问题）
		tolerance := maxPositionValue * 0.01 // 1%容差
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
				return fmt.Errorf("BTC/ETH单币种仓位价值不能超过%.0f USDT（10倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
			} else {
				return fmt.Errorf("山寨币单币种仓位价值不能超过%.0f USDT（1.5倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
			}
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("止损和止盈必须大于0")
		}

		// 验证止损止盈的合理性
		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("做多时止损价必须小于止盈价")
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("做空时止损价必须大于止盈价")
			}
		}

		// 验证风险回报比（必须≥1:3）
		// 计算入场价（假设当前市价）
		var entryPrice float64
		if d.Action == "open_long" {
			// 做多：入场价在止损和止盈之间
			entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.2 // 假设在20%位置入场
		} else {
			// 做空：入场价在止损和止盈之间
			entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.2 // 假设在20%位置入场
		}

		var riskPercent, rewardPercent, riskRewardRatio float64
		if d.Action == "open_long" {
			riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
			rewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else {
			riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
			rewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

		// 硬约束：风险回报比必须≥3.0
		if riskRewardRatio < 3.0 {
			return fmt.Errorf("风险回报比过低(%.2f:1)，必须≥3.0:1 [风险:%.2f%% 收益:%.2f%%] [止损:%.2f 止盈:%.2f]",
				riskRewardRatio, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
		}
	}

	// 部分平仓操作验证
	if d.Action == "partial_close" {
		if d.ClosePercentage <= 0 || d.ClosePercentage > 100 {
			return fmt.Errorf("平仓百分比必须在 0-100 之间: %.2f", d.ClosePercentage)
		}
		if d.Symbol == "" {
			return fmt.Errorf("部分平仓必须指定币种")
		}
	}

	// 更新止损操作验证
	if d.Action == "update_stop_loss" {
		if d.NewStopLoss <= 0 {
			return fmt.Errorf("新止损价格必须大于0: %.2f", d.NewStopLoss)
		}
		if d.Symbol == "" {
			return fmt.Errorf("更新止损必须指定币种")
		}
	}

	// 更新止盈操作验证
	if d.Action == "update_take_profit" {
		if d.NewTakeProfit <= 0 {
			return fmt.Errorf("新止盈价格必须大于0: %.2f", d.NewTakeProfit)
		}
		if d.Symbol == "" {
			return fmt.Errorf("更新止盈必须指定币种")
		}
	}

	return nil
}
