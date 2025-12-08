package trader

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"nofx/config"
	"nofx/decision"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AutoTraderConfig 自动交易配置（简化版 - AI全权决策）
type AutoTraderConfig struct {
	// Trader标识
	ID      string // Trader唯一标识（用于日志目录等）
	Name    string // Trader显示名称
	AIModel string // AI模型: "qwen" 或 "deepseek"

	// 交易平台选择
	Exchange string // "binance", "hyperliquid" 或 "aster"

	// 币安API配置
	BinanceAPIKey    string
	BinanceSecretKey string

	// Hyperliquid配置
	HyperliquidPrivateKey string
	HyperliquidWalletAddr string
	HyperliquidTestnet    bool

	// Aster配置
	AsterUser       string // Aster主钱包地址
	AsterSigner     string // Aster API钱包地址
	AsterPrivateKey string // Aster API钱包私钥

	CoinPoolAPIURL string

	// AI配置
	UseQwen     bool
	DeepSeekKey string
	QwenKey     string

	// 自定义AI API配置
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string

	// 扫描配置
	ScanInterval time.Duration // 扫描间隔（建议3分钟）

	// 账户配置
	InitialBalance float64 // 初始金额（用于计算盈亏，需手动设置）

	// 杠杆配置
	BTCETHLeverage  int // BTC和ETH的杠杆倍数
	AltcoinLeverage int // 山寨币的杠杆倍数

	// 风险控制（仅作为提示，AI可自主决定）
	MaxDailyLoss    float64       // 最大日亏损百分比（提示）
	MaxDrawdown     float64       // 最大回撤百分比（提示）
	StopTradingTime time.Duration // 触发风控后暂停时长

	// 仓位模式
	IsCrossMargin bool // true=全仓模式, false=逐仓模式

	// 币种配置
	DefaultCoins []string // 默认币种列表（从数据库获取）
	TradingCoins []string // 实际交易币种列表

	// 系统提示词模板
	SystemPromptTemplate string // 系统提示词模板名称（如 "default", "aggressive"）
}

// AutoTrader 自动交易器
type AutoTrader struct {
	id                     string // Trader唯一标识
	name                   string // Trader显示名称
	aiModel                string // AI模型名称
	exchange               string // 交易平台名称
	config                 AutoTraderConfig
	trader                 Trader // 使用Trader接口（支持多平台）
	mcpClient              *mcp.Client
	decisionLogger         *logger.DecisionLogger // 决策日志记录器
	initialBalance         float64
	dailyPnL               float64
	customPrompt           string   // 自定义交易策略prompt
	overrideBasePrompt     bool     // 是否覆盖基础prompt
	systemPromptTemplate   string   // 系统提示词模板名称
	defaultCoins           []string // 默认币种列表（从数据库获取）
	tradingCoins           []string // 实际交易币种列表
	lastResetTime          time.Time
	stopUntil              time.Time
	isRunning              bool
	startTime              time.Time                   // 系统启动时间
	callCount              int                         // AI调用次数
	positionFirstSeenTime  map[string]int64            // 持仓首次出现时间 (symbol_side -> timestamp毫秒)
	stopMonitorCh          chan struct{}               // 用于停止监控goroutine
	monitorWg              sync.WaitGroup              // 用于等待监控goroutine结束
	lastValidationFeedback *decision.ExecutionFeedback // 上轮校验失败结果
	peakPnLCache           map[string]float64          // 最高收益缓存 (symbol -> 峰值盈亏百分比)
	peakPnLCacheMutex      sync.RWMutex                // 缓存读写锁
	lastBalanceSyncTime    time.Time                   // 上次余额同步时间
	database               interface{}                 // 数据库引用（用于自动更新余额）
	userID                 string                      // 用户ID
	cycleMutex             sync.Mutex                  // 防止周期并发执行的互斥锁
	cycleRunning           bool                        // 周期是否正在执行中
	processedKlines        sync.Map                    // 防抖缓存：已处理的K线OpenTime (symbol -> map[int64]bool)
	lastEventTriggerTime   time.Time                   // 上次事件触发时间（用于防抖）
	eventTriggerMutex      sync.Mutex                  // 保护事件触发相关字段
}

// NewAutoTrader 创建自动交易器
func NewAutoTrader(config AutoTraderConfig, database interface{}, userID string) (*AutoTrader, error) {
	// 设置默认值
	if config.ID == "" {
		config.ID = "default_trader"
	}
	if config.Name == "" {
		config.Name = "Default Trader"
	}
	if config.AIModel == "" {
		if config.UseQwen {
			config.AIModel = "qwen"
		} else {
			config.AIModel = "deepseek"
		}
	}

	mcpClient := mcp.New()

	// 初始化AI
	if config.AIModel == "custom" {
		// 使用自定义API
		mcpClient.SetCustomAPI(config.CustomAPIURL, config.CustomAPIKey, config.CustomModelName)
		log.Printf("🤖 [%s] 使用自定义AI API: %s (模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
	} else if config.UseQwen || config.AIModel == "qwen" {
		// 使用Qwen (支持自定义URL和Model)
		mcpClient.SetQwenAPIKey(config.QwenKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] 使用阿里云Qwen AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] 使用阿里云Qwen AI", config.Name)
		}
	} else {
		// 默认使用DeepSeek (支持自定义URL和Model)
		mcpClient.SetDeepSeekAPIKey(config.DeepSeekKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] 使用DeepSeek AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] 使用DeepSeek AI", config.Name)
		}
	}

	// 初始化币种池API
	if config.CoinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(config.CoinPoolAPIURL)
	}

	// 设置默认交易平台
	if config.Exchange == "" {
		config.Exchange = "binance"
	}

	// 根据配置创建对应的交易器
	var trader Trader
	var err error

	// 记录仓位模式（通用）
	marginModeStr := "全仓"
	if !config.IsCrossMargin {
		marginModeStr = "逐仓"
	}
	log.Printf("📊 [%s] 仓位模式: %s", config.Name, marginModeStr)

	switch config.Exchange {
	case "binance":
		log.Printf("🏦 [%s] 使用币安合约交易", config.Name)
		trader = NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey, userID)
	case "hyperliquid":
		log.Printf("🏦 [%s] 使用Hyperliquid交易", config.Name)
		trader, err = NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet)
		if err != nil {
			return nil, fmt.Errorf("初始化Hyperliquid交易器失败: %w", err)
		}
	case "aster":
		log.Printf("🏦 [%s] 使用Aster交易", config.Name)
		trader, err = NewAsterTrader(config.AsterUser, config.AsterSigner, config.AsterPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("初始化Aster交易器失败: %w", err)
		}
	default:
		return nil, fmt.Errorf("不支持的交易平台: %s", config.Exchange)
	}

	// 验证初始金额配置
	if config.InitialBalance <= 0 {
		return nil, fmt.Errorf("初始金额必须大于0，请在配置中设置InitialBalance")
	}

	// 初始化决策日志记录器（使用trader ID创建独立目录）
	logDir := fmt.Sprintf("decision_logs/%s", config.ID)
	decisionLogger := logger.NewDecisionLogger(logDir)

	// 设置默认系统提示词模板
	systemPromptTemplate := config.SystemPromptTemplate
	if systemPromptTemplate == "" {
		// feature/partial-close-dynamic-tpsl 分支默认使用 adaptive（支持动态止盈止损）
		systemPromptTemplate = "adaptive"
	}

	return &AutoTrader{
		id:                    config.ID,
		name:                  config.Name,
		aiModel:               config.AIModel,
		exchange:              config.Exchange,
		config:                config,
		trader:                trader,
		mcpClient:             mcpClient,
		decisionLogger:        decisionLogger,
		initialBalance:        config.InitialBalance,
		systemPromptTemplate:  systemPromptTemplate,
		defaultCoins:          config.DefaultCoins,
		tradingCoins:          config.TradingCoins,
		lastResetTime:         time.Now(),
		startTime:             time.Now(),
		callCount:             0,
		isRunning:             false,
		positionFirstSeenTime: make(map[string]int64),
		stopMonitorCh:         make(chan struct{}),
		monitorWg:             sync.WaitGroup{},
		peakPnLCache:          make(map[string]float64),
		peakPnLCacheMutex:     sync.RWMutex{},
		lastBalanceSyncTime:   time.Now(), // 初始化为当前时间
		database:              database,
		userID:                userID,
	}, nil
}

// handleNewKlineEvent 处理新K线形成事件（事件驱动决策）
func (at *AutoTrader) handleNewKlineEvent(symbol string, kline market.Kline, duration string) {
	// 防抖检查：避免同一根K线触发多次决策
	at.eventTriggerMutex.Lock()

	// 检查是否已经处理过这根K线
	symbolKlines, exists := at.processedKlines.Load(symbol)
	var processedMap map[int64]bool
	if exists {
		processedMap = symbolKlines.(map[int64]bool)
	} else {
		processedMap = make(map[int64]bool)
		at.processedKlines.Store(symbol, processedMap)
	}

	// 如果已经处理过这根K线，直接返回
	if processedMap[kline.OpenTime] {
		at.eventTriggerMutex.Unlock()
		return
	}

	// 标记为已处理
	processedMap[kline.OpenTime] = true
	at.processedKlines.Store(symbol, processedMap)

	// 检查最小触发间隔（防抖：至少间隔10秒，避免过于频繁）
	const minEventInterval = 10 * time.Second
	now := time.Now()
	if !at.lastEventTriggerTime.IsZero() && now.Sub(at.lastEventTriggerTime) < minEventInterval {
		at.eventTriggerMutex.Unlock()
		log.Printf("⏸ 事件触发过于频繁（距离上次 %.1f 秒），跳过本次触发（防抖）", now.Sub(at.lastEventTriggerTime).Seconds())
		return
	}

	at.lastEventTriggerTime = now
	at.eventTriggerMutex.Unlock()

	// 检查周期是否正在执行
	at.cycleMutex.Lock()
	isRunning := at.cycleRunning
	at.cycleMutex.Unlock()

	if isRunning {
		log.Printf("⏸ 周期正在执行中，跳过事件触发（等待当前周期完成）")
		return
	}

	// 记录事件触发日志
	log.Printf("⚡ 事件驱动：检测到新3分钟K线形成 [%s] OpenTime: %d，立即触发决策", symbol, kline.OpenTime)

	// 触发决策周期
	if err := at.runCycle(); err != nil {
		log.Printf("❌ 事件驱动决策执行失败: %v", err)
	}
}

// Run 运行自动交易主循环
func (at *AutoTrader) Run() error {
	at.isRunning = true
	at.stopMonitorCh = make(chan struct{})
	at.startTime = time.Now()

	log.Println("🚀 AI驱动自动交易系统启动")
	log.Printf("💰 初始余额: %.2f USDT", at.initialBalance)
	log.Printf("⚙️  扫描间隔: %v", at.config.ScanInterval)
	log.Println("🤖 AI将全权决定杠杆、仓位大小、止损止盈等参数")
	at.monitorWg.Add(1)
	defer at.monitorWg.Done()

	// 启动回撤监控
	at.startDrawdownMonitor()

	// 注册新K线回调（事件驱动决策）
	if market.WSMonitorCli != nil {
		market.WSMonitorCli.SetOnNewKlineCallback(at.handleNewKlineEvent)
		log.Println("✅ 已启用事件驱动决策：新3分钟K线形成时将立即触发决策")
	} else {
		log.Println("⚠️  WSMonitor未初始化，事件驱动决策不可用，将仅使用定时器")
	}

	// 保留定时器作为兜底机制（防止WebSocket事件丢失）
	ticker := time.NewTicker(at.config.ScanInterval)
	defer ticker.Stop()
	log.Printf("⏰ 定时器兜底机制已启用（间隔: %v），确保即使事件丢失也能定期决策", at.config.ScanInterval)

	// 首次立即执行
	if err := at.runCycle(); err != nil {
		log.Printf("❌ 执行失败: %v", err)
	}

	for at.isRunning {
		select {
		case <-ticker.C:
			// 定时器触发（兜底机制）
			// 检查上一个周期是否还在执行
			at.cycleMutex.Lock()
			isRunning := at.cycleRunning
			at.cycleMutex.Unlock()

			if isRunning {
				// 上一个周期还在执行，跳过本次触发
				log.Printf("⏸ 上一个周期仍在执行中，跳过本次定时器触发（等待下一个周期）")
				continue
			}

			// 正常执行周期（定时器兜底）
			log.Printf("⏰ 定时器触发决策（兜底机制）")
			if err := at.runCycle(); err != nil {
				log.Printf("❌ 执行失败: %v", err)
			}
		case <-at.stopMonitorCh:
			log.Printf("[%s] ⏹ 收到停止信号，退出自动交易主循环", at.name)
			// 清除回调
			if market.WSMonitorCli != nil {
				market.WSMonitorCli.SetOnNewKlineCallback(nil)
			}
			return nil
		}
	}

	return nil
}

// Stop 停止自动交易
func (at *AutoTrader) Stop() {
	if !at.isRunning {
		return
	}
	at.isRunning = false
	// 清除回调
	if market.WSMonitorCli != nil {
		market.WSMonitorCli.SetOnNewKlineCallback(nil)
	}
	close(at.stopMonitorCh) // 通知监控goroutine停止
	at.monitorWg.Wait()     // 等待监控goroutine结束
	log.Println("⏹ 自动交易系统停止")
}

// runCycle 运行一个交易周期（使用AI全权决策）
func (at *AutoTrader) runCycle() error {
	// 使用互斥锁防止并发执行
	at.cycleMutex.Lock()
	// 检查是否已经在执行
	if at.cycleRunning {
		at.cycleMutex.Unlock()
		log.Printf("⏸ 周期已在执行中，跳过本次调用")
		return nil
	}
	// 标记为正在执行
	at.cycleRunning = true
	at.cycleMutex.Unlock()

	// 确保在函数退出时清除执行标志
	defer func() {
		at.cycleMutex.Lock()
		at.cycleRunning = false
		at.cycleMutex.Unlock()
	}()

	// 记录周期开始时间
	cycleStartTime := time.Now()
	at.callCount++

	separator := strings.Repeat("=", 70)
	log.Printf("\n%s", separator)
	log.Printf("⏰ %s - AI决策周期 #%d", cycleStartTime.Format("2006-01-02 15:04:05"), at.callCount)
	log.Printf("%s", separator)

	// 保存上轮校验失败结果（用于构建上下文），然后清空当前反馈
	lastFeedback := at.lastValidationFeedback
	at.lastValidationFeedback = &decision.ExecutionFeedback{
		HasRejected:       false,
		RejectedDecisions: []decision.RejectedDecision{},
	}

	// 创建决策记录
	record := &logger.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
	}

	// 1. 检查是否需要停止交易
	if !at.stopUntil.IsZero() && time.Now().Before(at.stopUntil) {
		remaining := time.Until(at.stopUntil)
		log.Printf("⏸ 风险控制：暂停交易中，剩余 %.0f 分钟", remaining.Minutes())
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("风险控制暂停中，剩余 %.0f 分钟", remaining.Minutes())
		at.decisionLogger.LogDecision(record)
		return nil
	}

	// 2. 重置日盈亏（每天重置）
	if time.Since(at.lastResetTime) > 24*time.Hour {
		at.dailyPnL = 0
		at.lastResetTime = time.Now()
		log.Println("📅 日盈亏已重置")
	}

	// 4. 收集交易上下文（使用上轮保存的反馈）
	ctx, err := at.buildTradingContextWithFeedback(lastFeedback)
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("构建交易上下文失败: %v", err)
		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("构建交易上下文失败: %w", err)
	}

	// 保存账户状态快照
	record.AccountState = logger.AccountSnapshot{
		TotalBalance:          ctx.Account.TotalEquity - ctx.Account.UnrealizedPnL,
		AvailableBalance:      ctx.Account.AvailableBalance,
		TotalUnrealizedProfit: ctx.Account.UnrealizedPnL,
		PositionCount:         ctx.Account.PositionCount,
		MarginUsedPct:         ctx.Account.MarginUsedPct,
		InitialBalance:        at.initialBalance, // 记录当时的初始余额基准
	}

	// 保存持仓快照
	for _, pos := range ctx.Positions {
		record.Positions = append(record.Positions, logger.PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			PositionAmt:      pos.Quantity,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        pos.MarkPrice,
			UnrealizedProfit: pos.UnrealizedPnL,
			Leverage:         float64(pos.Leverage),
			LiquidationPrice: pos.LiquidationPrice,
		})
	}

	log.Print(strings.Repeat("=", 70))
	for _, coin := range ctx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	log.Printf("📊 账户净值: %.2f USDT | 可用: %.2f USDT | 持仓: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 5. 调用AI获取完整决策
	log.Printf("🤖 正在请求AI分析并决策... [模板: %s]", at.systemPromptTemplate)
	decision, err := decision.GetFullDecisionWithCustomPrompt(ctx, at.mcpClient, at.customPrompt, at.overrideBasePrompt, at.systemPromptTemplate)

	if decision != nil && decision.AIRequestDurationMs > 0 {
		record.AIRequestDurationMs = decision.AIRequestDurationMs
		log.Printf("⏱️ AI调用耗时: %.2f 秒", float64(record.AIRequestDurationMs)/1000)
		record.ExecutionLog = append(record.ExecutionLog,
			fmt.Sprintf("AI调用耗时: %d ms", record.AIRequestDurationMs))
	}

	// 即使有错误，也保存思维链、决策和输入prompt（用于debug）
	if decision != nil {
		record.SystemPrompt = decision.SystemPrompt // 保存系统提示词
		record.InputPrompt = decision.UserPrompt
		record.CoTTrace = decision.CoTTrace
		if len(decision.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(decision.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
	}

	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("获取AI决策失败: %v", err)

		// 检查是否为余额不足错误
		errStr := err.Error()
		if strings.Contains(errStr, "余额不足") || strings.Contains(errStr, "Insufficient Balance") || strings.Contains(errStr, "status 402") {
			warningSeparator := strings.Repeat("!", 70)
			log.Printf("\n%s", warningSeparator)
			log.Printf("⚠️  ⚠️  ⚠️  重要警告：AI API账户余额不足 ⚠️  ⚠️  ⚠️")
			log.Printf("%s", warningSeparator)
			log.Printf("❌ 错误详情: %v", err)
			log.Printf("💡 解决方案:")
			log.Printf("   1. 登录您的AI API提供商账户（DeepSeek/Qwen等）")
			log.Printf("   2. 充值账户余额")
			log.Printf("   3. 确认余额充足后，交易机器人将自动恢复工作")
			log.Printf("%s\n", warningSeparator)
		}

		// 打印系统提示词和AI思维链（即使有错误，也要输出以便调试）
		if decision != nil {
			log.Print("\n" + strings.Repeat("=", 70) + "\n")
			log.Printf("📋 系统提示词 [模板: %s] (错误情况)", at.systemPromptTemplate)
			log.Println(strings.Repeat("=", 70))
			log.Println(decision.SystemPrompt)
			log.Println(strings.Repeat("=", 70))

			if decision.CoTTrace != "" {
				log.Print("\n" + strings.Repeat("-", 70) + "\n")
				log.Println("💭 AI思维链分析（错误情况）:")
				log.Println(strings.Repeat("-", 70))
				log.Println(decision.CoTTrace)
				log.Println(strings.Repeat("-", 70))
			}
		}

		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("获取AI决策失败: %w", err)
	}

	// // 5. 打印系统提示词
	// log.Printf("\n" + strings.Repeat("=", 70))
	// log.Printf("📋 系统提示词 [模板: %s]", at.systemPromptTemplate)
	// log.Println(strings.Repeat("=", 70))
	// log.Println(decision.SystemPrompt)
	// log.Printf(strings.Repeat("=", 70) + "\n")

	// 6. 打印AI思维链
	// log.Printf("\n" + strings.Repeat("-", 70))
	// log.Println("💭 AI思维链分析:")
	// log.Println(strings.Repeat("-", 70))
	// log.Println(decision.CoTTrace)
	// log.Printf(strings.Repeat("-", 70) + "\n")

	// 7. 打印AI决策
	// log.Printf("📋 AI决策列表 (%d 个):\n", len(decision.Decisions))
	// for i, d := range decision.Decisions {
	//     log.Printf("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
	//     if d.Action == "open_long" || d.Action == "open_short" {
	//        log.Printf("      杠杆: %dx | 仓位: %.2f USDT | 止损: %.4f | 止盈: %.4f",
	//           d.Leverage, d.PositionSizeUSD, d.StopLoss, d.TakeProfit)
	//     }
	// }
	log.Println()
	log.Print(strings.Repeat("-", 70))
	// 8. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
	log.Print(strings.Repeat("-", 70))

	// 8. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
	sortedDecisions := sortDecisionsByPriority(decision.Decisions)

	log.Println("🔄 执行顺序（已优化）: 先平仓→后开仓")
	for i, d := range sortedDecisions {
		log.Printf("  [%d] %s %s", i+1, d.Symbol, d.Action)
	}
	log.Println()

	// 执行决策并记录结果
	for _, d := range sortedDecisions {
		actionRecord := logger.DecisionAction{
			Action:    d.Action,
			Symbol:    d.Symbol,
			Quantity:  0,
			Leverage:  d.Leverage,
			Price:     0,
			Timestamp: time.Now(),
			Success:   false,
			Reasoning: d.Reasoning, // 保存AI的决策原因
		}

		// 在执行前校验AI指令是否符合系统执行限制
		currentPositions, err := at.trader.GetPositions()
		if err == nil {
			if validationErr := at.validateDecision(&d, currentPositions); validationErr != nil {
				log.Printf("❌ 决策校验失败 (%s %s): %v", d.Symbol, d.Action, validationErr)
				actionRecord.Error = validationErr.Error()
				record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s 校验失败: %v", d.Symbol, d.Action, validationErr))
				record.Decisions = append(record.Decisions, actionRecord)

				// 保存校验失败结果，供下一轮使用
				at.lastValidationFeedback.HasRejected = true
				// 通过 ExecutionFeedback 的 RejectedDecisions 字段类型来创建
				rejected := struct {
					Symbol   string
					Action   string
					Leverage int
					Reason   string
				}{
					Symbol:   d.Symbol,
					Action:   d.Action,
					Leverage: d.Leverage,
					Reason:   validationErr.Error(),
				}
				// 使用类型断言转换
				at.lastValidationFeedback.RejectedDecisions = append(at.lastValidationFeedback.RejectedDecisions, rejected)

				continue // 跳过执行，继续下一个决策
			}
		} else {
			log.Printf("⚠️ 获取持仓信息失败，跳过校验: %v", err)
		}

		if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			log.Printf("❌ 执行决策失败 (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s 失败: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 成功", d.Symbol, d.Action))
			// 成功执行后短暂延迟
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 9. 检测自动平仓（止盈/止损触发）
	at.detectAutoClosures()

	// 10. 保存决策记录
	if err := at.decisionLogger.LogDecision(record); err != nil {
		log.Printf("⚠ 保存决策记录失败: %v", err)
	}

	return nil
}

// detectAutoClosures 检测自动平仓（止盈/止损触发）
func (at *AutoTrader) detectAutoClosures() {
	if at.database == nil {
		return
	}

	db, ok := at.database.(interface {
		GetOpenTrades(traderID string) ([]*config.TradeRecord, error)
		UpdateTradeClose(traderID, symbol, side string, closeTime time.Time, closePrice, pnl, pnlPct float64, closeReason string, orderIDClose int64, wasStopLoss bool) error
	})
	if !ok {
		return
	}

	// 获取数据库中的未平仓交易
	openTrades, err := db.GetOpenTrades(at.id)
	if err != nil {
		log.Printf("⚠️ 获取未平仓交易失败: %v", err)
		return
	}

	// 获取当前实际持仓
	currentPositions, err := at.trader.GetPositions()
	if err != nil {
		log.Printf("⚠️ 获取当前持仓失败: %v", err)
		return
	}

	// 构建当前持仓的 key 集合 (symbol_side)
	currentPositionKeys := make(map[string]bool)
	for _, pos := range currentPositions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		quantity, _ := pos["positionAmt"].(float64)
		if quantity != 0 {
			key := symbol + "_" + side
			currentPositionKeys[key] = true
		}
	}

	// 检查数据库中的未平仓交易是否还在持仓中
	for _, trade := range openTrades {
		key := trade.Symbol + "_" + trade.Side
		if !currentPositionKeys[key] {
			// 持仓已消失，说明被自动平仓了
			log.Printf("🔍 检测到自动平仓: %s %s (开仓时间: %s)", trade.Symbol, trade.Side, trade.OpenTime.Format("2006-01-02 15:04:05"))

			// 获取当前价格计算盈亏
			marketData, err := market.Get(trade.Symbol)
			if err != nil {
				log.Printf("⚠️ 获取市场价格失败: %v", err)
				continue
			}

			// 计算盈亏
			var pnl, pnlPct float64
			if trade.Side == "long" {
				pnl = (marketData.CurrentPrice - trade.OpenPrice) * trade.Quantity
			} else {
				pnl = (trade.OpenPrice - marketData.CurrentPrice) * trade.Quantity
			}
			marginUsed := (trade.Quantity * trade.OpenPrice) / float64(trade.Leverage)
			if marginUsed > 0 {
				pnlPct = (pnl / marginUsed) * 100
			}

			// 判断是止损还是止盈（根据盈亏判断）
			closeReason := "take_profit"
			wasStopLoss := false
			if pnl < 0 {
				closeReason = "stop_loss"
				wasStopLoss = true
			}

			// 更新数据库记录
			if err := db.UpdateTradeClose(at.id, trade.Symbol, trade.Side, time.Now(), marketData.CurrentPrice, pnl, pnlPct, closeReason, 0, wasStopLoss); err != nil {
				log.Printf("⚠️ 更新自动平仓记录失败: %v", err)
			} else {
				log.Printf("✓ 自动平仓记录已更新: %s %s | %s | PnL: %.2f USDT (%.2f%%)",
					trade.Symbol, trade.Side, closeReason, pnl, pnlPct)
			}
		}
	}
}

// buildTradingContext 构建交易上下文
func (at *AutoTrader) buildTradingContext() (*decision.Context, error) {
	// 1. 获取账户信息
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
	}

	// 获取账户字段
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Total Equity = 钱包余额 + 未实现盈亏
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 2. 获取持仓信息
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var positionInfos []decision.PositionInfo
	totalMarginUsed := 0.0

	// 当前持仓的key集合（用于清理已平仓的记录）
	currentPositionKeys := make(map[string]bool)

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // 空仓数量为负，转为正数
		}

		// 跳过已平仓的持仓（quantity = 0），防止"幽灵持仓"传递给AI
		if quantity == 0 {
			continue
		}

		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		// 计算占用保证金（估算）
		leverage := 10 // 默认值，实际应该从持仓信息获取
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		// 计算盈亏百分比（基于保证金，考虑杠杆）
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		// 跟踪持仓首次出现时间
		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true
		if _, exists := at.positionFirstSeenTime[posKey]; !exists {
			// 新持仓，记录当前时间
			at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
		}
		updateTime := at.positionFirstSeenTime[posKey]

		// 获取该持仓的历史最高收益率
		at.peakPnLCacheMutex.RLock()
		peakPnlPct := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		positionInfos = append(positionInfos, decision.PositionInfo{
			Symbol:           symbol,
			Side:             side,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			Quantity:         quantity,
			Leverage:         leverage,
			UnrealizedPnL:    unrealizedPnl,
			UnrealizedPnLPct: pnlPct,
			PeakPnLPct:       peakPnlPct,
			LiquidationPrice: liquidationPrice,
			MarginUsed:       marginUsed,
			UpdateTime:       updateTime,
		})
	}

	// 清理已平仓的持仓记录
	for key := range at.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			delete(at.positionFirstSeenTime, key)
		}
	}

	// 3. 获取交易员的候选币种池
	candidateCoins, err := at.getCandidateCoins()
	if err != nil {
		return nil, fmt.Errorf("获取候选币种失败: %w", err)
	}

	// 4. 计算总盈亏
	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. 分析历史表现（最近1000个周期，避免长期持仓的交易记录丢失）
	// 假设每3分钟一个周期，1000个周期 = 50小时，足够覆盖大部分交易
	// 即使开仓记录在窗口外，也会从更早的历史记录中查找匹配
	performance, err := at.decisionLogger.AnalyzePerformance(1000, at.database, at.id)
	if err != nil {
		log.Printf("⚠️  分析历史表现失败: %v", err)
		// 不影响主流程，继续执行（但设置performance为nil以避免传递错误数据）
		performance = nil
	}

	// 6. 构建上下文
	ctx := &decision.Context{
		CurrentTime:     time.Now().Format("2006-01-02 15:04:05"),
		RuntimeMinutes:  int(time.Since(at.startTime).Minutes()),
		CallCount:       at.callCount,
		BTCETHLeverage:  at.config.BTCETHLeverage,  // 使用配置的杠杆倍数
		AltcoinLeverage: at.config.AltcoinLeverage, // 使用配置的杠杆倍数
		Account: decision.AccountInfo{
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			UnrealizedPnL:    totalUnrealizedProfit,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			MarginUsed:       totalMarginUsed,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positionInfos),
		},
		Positions:      positionInfos,
		CandidateCoins: candidateCoins,
		Performance:    performance, // 添加历史表现分析
	}

	// 设置上轮校验失败反馈（如果没有传入，使用空的反馈）
	ctx.LastExecutionFeedback = &decision.ExecutionFeedback{
		HasRejected:       false,
		RejectedDecisions: []decision.RejectedDecision{},
	}

	return ctx, nil
}

// buildTradingContextWithFeedback 构建交易上下文（使用指定的反馈信息）
func (at *AutoTrader) buildTradingContextWithFeedback(lastFeedback *decision.ExecutionFeedback) (*decision.Context, error) {
	ctx, err := at.buildTradingContext()
	if err != nil {
		return nil, err
	}

	// 设置上轮校验失败反馈
	if lastFeedback != nil {
		ctx.LastExecutionFeedback = lastFeedback
	}

	return ctx, nil
}

// isBTCETH 判断币种是否为BTC或ETH
func isBTCETH(symbol string) bool {
	symbol = strings.ToUpper(symbol)
	return symbol == "BTCUSDT" || symbol == "ETHUSDT"
}

// validateDecision 校验AI指令是否符合系统执行限制
func (at *AutoTrader) validateDecision(d *decision.Decision, currentPositions []map[string]interface{}) error {
	// 只对开仓操作进行校验
	if d.Action != "open_long" && d.Action != "open_short" {
		return nil
	}

	// 1. 检查最大持仓币种数（3个）
	// 统计当前持仓的币种数量（不包含当前要开的币种）
	positionSymbols := make(map[string]bool)
	for _, pos := range currentPositions {
		if symbol, ok := pos["symbol"].(string); ok {
			if symbol != d.Symbol {
				positionSymbols[symbol] = true
			}
		}
	}

	// 如果当前要开的币种不在持仓中，检查是否会超过3个
	if !positionSymbols[d.Symbol] {
		if len(positionSymbols) >= 3 {
			return fmt.Errorf("❌ 违反规则：最大持仓币种数为3，当前已有%d个币种持仓，无法开新仓", len(positionSymbols))
		}
	}

	// 2. 检查单币种最大杠杆
	maxLeverage := at.config.AltcoinLeverage
	if isBTCETH(d.Symbol) {
		maxLeverage = at.config.BTCETHLeverage
	}

	if d.Leverage > maxLeverage {
		coinType := "山寨币"
		if isBTCETH(d.Symbol) {
			coinType = "BTC/ETH"
		}
		return fmt.Errorf("❌ 违反规则：%s最大杠杆为%dx，AI指令要求%dx", coinType, maxLeverage, d.Leverage)
	}

	// 3. 检查最小开仓名义价值（10 USDT）
	if d.PositionSizeUSD < 10.0 {
		return fmt.Errorf("❌ 违反规则：最小开仓名义价值为10 USDT，AI指令要求%.2f USDT", d.PositionSizeUSD)
	}

	// 4. 检查账户最大保证金使用率（90%）
	// 获取当前账户信息
	balance, err := at.trader.GetBalance()
	if err != nil {
		// 如果获取余额失败，记录警告但不阻止执行（让后续的保证金检查处理）
		log.Printf("⚠️ 校验时获取账户余额失败: %v，跳过保证金使用率检查", err)
	} else {
		totalEquity := 0.0
		marginUsed := 0.0

		if equity, ok := balance["totalEquity"].(float64); ok {
			totalEquity = equity
		} else if equity, ok := balance["totalEquity"].(string); ok {
			if parsed, err := strconv.ParseFloat(equity, 64); err == nil {
				totalEquity = parsed
			}
		}

		if margin, ok := balance["marginUsed"].(float64); ok {
			marginUsed = margin
		} else if margin, ok := balance["marginUsed"].(string); ok {
			if parsed, err := strconv.ParseFloat(margin, 64); err == nil {
				marginUsed = parsed
			}
		}

		// 计算新开仓需要的保证金
		requiredMargin := d.PositionSizeUSD / float64(d.Leverage)
		newMarginUsed := marginUsed + requiredMargin

		// 检查是否超过90%
		if totalEquity > 0 {
			marginUsedPct := (newMarginUsed / totalEquity) * 100
			if marginUsedPct > 90.0 {
				return fmt.Errorf("❌ 违反规则：账户最大保证金使用率为90%%，开仓后预计使用率%.2f%%", marginUsedPct)
			}
		}
	}

	return nil
}

// executeDecisionWithRecord 执行AI决策并记录详细信息
func (at *AutoTrader) executeDecisionWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	switch decision.Action {
	case "open_long":
		return at.executeOpenLongWithRecord(decision, actionRecord)
	case "open_short":
		return at.executeOpenShortWithRecord(decision, actionRecord)
	case "close_long":
		return at.executeCloseLongWithRecord(decision, actionRecord)
	case "close_short":
		return at.executeCloseShortWithRecord(decision, actionRecord)
	case "update_stop_loss":
		return at.executeUpdateStopLossWithRecord(decision, actionRecord)
	case "update_take_profit":
		return at.executeUpdateTakeProfitWithRecord(decision, actionRecord)
	case "partial_close":
		return at.executePartialCloseWithRecord(decision, actionRecord)
	case "hold", "wait":
		// 无需执行，仅记录
		return nil
	default:
		return fmt.Errorf("未知的action: %s", decision.Action)
	}
}

// executeOpenLongWithRecord 执行开多仓并记录详细信息
func (at *AutoTrader) executeOpenLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📈 开多仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种持仓（单一币种，单一持仓规则）
	// 无论方向如何，只要该币种有持仓，就不允许开新仓
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol {
				side := pos["side"].(string)
				return fmt.Errorf("❌ %s 已有%s持仓，禁止开新仓（单一币种，单一持仓规则）。如需换仓，请先给出 close_%s 决策", decision.Symbol, side, side)
			}
		}
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// ⚠️ 保证金验证：防止保证金不足错误（code=-2019）
	requiredMargin := decision.PositionSizeUSD / float64(decision.Leverage)

	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// 手续费估算（Taker费率 0.04%）
	estimatedFee := decision.PositionSizeUSD * 0.0004
	totalRequired := requiredMargin + estimatedFee

	if totalRequired > availableBalance {
		return fmt.Errorf("❌ 保证金不足: 需要 %.2f USDT（保证金 %.2f + 手续费 %.2f），可用 %.2f USDT",
			totalRequired, requiredMargin, estimatedFee, availableBalance)
	}

	// 设置仓位模式
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		// 继续执行，不影响交易
	}

	// 开仓
	order, err := at.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

	// 记录开仓时间
	posKey := decision.Symbol + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// 设置止损止盈
	if err := at.trader.SetStopLoss(decision.Symbol, "LONG", quantity, decision.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "LONG", quantity, decision.TakeProfit); err != nil {
		log.Printf("  ⚠ 设置止盈失败: %v", err)
	}

	// 记录到数据库
	if db, ok := at.database.(interface {
		CreateTrade(trade *config.TradeRecord) error
	}); ok {
		orderID := int64(0)
		if id, ok := order["orderId"].(int64); ok {
			orderID = id
		}
		trade := &config.TradeRecord{
			TraderID:    at.id,
			Symbol:      decision.Symbol,
			Side:        "long",
			OpenTime:    time.Now(),
			OpenPrice:   marketData.CurrentPrice,
			Quantity:    quantity,
			Leverage:    decision.Leverage,
			OrderIDOpen: orderID,
		}
		if err := db.CreateTrade(trade); err != nil {
			log.Printf("  ⚠️ 记录交易到数据库失败: %v", err)
		} else {
			log.Printf("  ✓ 交易记录已保存到数据库 (ID: %d)", trade.ID)
		}
	}

	return nil
}

// executeOpenShortWithRecord 执行开空仓并记录详细信息
func (at *AutoTrader) executeOpenShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📉 开空仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种持仓（单一币种，单一持仓规则）
	// 无论方向如何，只要该币种有持仓，就不允许开新仓
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol {
				side := pos["side"].(string)
				return fmt.Errorf("❌ %s 已有%s持仓，禁止开新仓（单一币种，单一持仓规则）。如需换仓，请先给出 close_%s 决策", decision.Symbol, side, side)
			}
		}
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// ⚠️ 保证金验证：防止保证金不足错误（code=-2019）
	requiredMargin := decision.PositionSizeUSD / float64(decision.Leverage)

	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// 手续费估算（Taker费率 0.04%）
	estimatedFee := decision.PositionSizeUSD * 0.0004
	totalRequired := requiredMargin + estimatedFee

	if totalRequired > availableBalance {
		return fmt.Errorf("❌ 保证金不足: 需要 %.2f USDT（保证金 %.2f + 手续费 %.2f），可用 %.2f USDT",
			totalRequired, requiredMargin, estimatedFee, availableBalance)
	}

	// 设置仓位模式
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		// 继续执行，不影响交易
	}

	// 开仓
	order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

	// 记录开仓时间
	posKey := decision.Symbol + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// 设置止损止盈
	if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, decision.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "SHORT", quantity, decision.TakeProfit); err != nil {
		log.Printf("  ⚠ 设置止盈失败: %v", err)
	}

	// 记录到数据库
	if db, ok := at.database.(interface {
		CreateTrade(trade *config.TradeRecord) error
	}); ok {
		orderID := int64(0)
		if id, ok := order["orderId"].(int64); ok {
			orderID = id
		}
		trade := &config.TradeRecord{
			TraderID:    at.id,
			Symbol:      decision.Symbol,
			Side:        "short",
			OpenTime:    time.Now(),
			OpenPrice:   marketData.CurrentPrice,
			Quantity:    quantity,
			Leverage:    decision.Leverage,
			OrderIDOpen: orderID,
		}
		if err := db.CreateTrade(trade); err != nil {
			log.Printf("  ⚠️ 记录交易到数据库失败: %v", err)
		} else {
			log.Printf("  ✓ 交易记录已保存到数据库 (ID: %d)", trade.ID)
		}
	}

	return nil
}

// executeCloseLongWithRecord 执行平多仓并记录详细信息
func (at *AutoTrader) executeCloseLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平多仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 平仓
	order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	// 记录订单ID
	var orderIDClose int64
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
		orderIDClose = orderID
	}

	// 更新数据库记录
	if db, ok := at.database.(interface {
		UpdateTradeClose(traderID, symbol, side string, closeTime time.Time, closePrice, pnl, pnlPct float64, closeReason string, orderIDClose int64, wasStopLoss bool) error
		GetOpenTrades(traderID string) ([]*config.TradeRecord, error)
	}); ok {
		// 获取开仓记录以计算盈亏
		openTrades, err := db.GetOpenTrades(at.id)
		if err == nil {
			for _, trade := range openTrades {
				if trade.Symbol == decision.Symbol && trade.Side == "long" {
					// 计算盈亏
					pnl := (marketData.CurrentPrice - trade.OpenPrice) * trade.Quantity
					marginUsed := (trade.Quantity * trade.OpenPrice) / float64(trade.Leverage)
					pnlPct := 0.0
					if marginUsed > 0 {
						pnlPct = (pnl / marginUsed) * 100
					}

					// 更新数据库
					if err := db.UpdateTradeClose(at.id, decision.Symbol, "long", time.Now(), marketData.CurrentPrice, pnl, pnlPct, "manual", orderIDClose, false); err != nil {
						log.Printf("  ⚠️ 更新交易记录失败: %v", err)
					} else {
						log.Printf("  ✓ 交易记录已更新到数据库 (PnL: %.2f USDT, %.2f%%)", pnl, pnlPct)
					}
					break
				}
			}
		}
	}

	log.Printf("  ✓ 平仓成功")
	return nil
}

// executeCloseShortWithRecord 执行平空仓并记录详细信息
func (at *AutoTrader) executeCloseShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平空仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 平仓
	order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	// 记录订单ID
	var orderIDClose int64
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
		orderIDClose = orderID
	}

	// 更新数据库记录
	if db, ok := at.database.(interface {
		UpdateTradeClose(traderID, symbol, side string, closeTime time.Time, closePrice, pnl, pnlPct float64, closeReason string, orderIDClose int64, wasStopLoss bool) error
		GetOpenTrades(traderID string) ([]*config.TradeRecord, error)
	}); ok {
		// 获取开仓记录以计算盈亏
		openTrades, err := db.GetOpenTrades(at.id)
		if err == nil {
			for _, trade := range openTrades {
				if trade.Symbol == decision.Symbol && trade.Side == "short" {
					// 计算盈亏
					pnl := (trade.OpenPrice - marketData.CurrentPrice) * trade.Quantity
					marginUsed := (trade.Quantity * trade.OpenPrice) / float64(trade.Leverage)
					pnlPct := 0.0
					if marginUsed > 0 {
						pnlPct = (pnl / marginUsed) * 100
					}

					// 更新数据库
					if err := db.UpdateTradeClose(at.id, decision.Symbol, "short", time.Now(), marketData.CurrentPrice, pnl, pnlPct, "manual", orderIDClose, false); err != nil {
						log.Printf("  ⚠️ 更新交易记录失败: %v", err)
					} else {
						log.Printf("  ✓ 交易记录已更新到数据库 (PnL: %.2f USDT, %.2f%%)", pnl, pnlPct)
					}
					break
				}
			}
		}
	}

	log.Printf("  ✓ 平仓成功")
	return nil
}

// executeUpdateStopLossWithRecord 执行调整止损并记录详细信息
func (at *AutoTrader) executeUpdateStopLossWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 调整止损: %s → %.2f", decision.Symbol, decision.NewStopLoss)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 获取入场价（用于验证止损逻辑）
	var entryPrice float64
	if ep, ok := targetPosition["entryPrice"].(float64); ok {
		entryPrice = ep
	} else if epStr, ok := targetPosition["entryPrice"].(string); ok {
		if ep, err := strconv.ParseFloat(epStr, 64); err == nil {
			entryPrice = ep
		}
	} else if ep, ok := targetPosition["entry_price"].(float64); ok {
		entryPrice = ep
	} else if epStr, ok := targetPosition["entry_price"].(string); ok {
		if ep, err := strconv.ParseFloat(epStr, 64); err == nil {
			entryPrice = ep
		}
	}

	if entryPrice <= 0 {
		return fmt.Errorf("无法获取 %s 的入场价", decision.Symbol)
	}

	// 验证新止损价格合理性
	currentPrice := marketData.CurrentPrice

	if positionSide == "LONG" {
		// 多仓止损逻辑：
		// 1. 开仓时设置初始止损：止损价 < 入场价（限制初始亏损）
		// 2. 持仓盈利后移动（上调）止损：新止损价 > 旧止损价，并且新止损价 >= 入场价（锁定已有利润）
		// 3. 新止损必须 < 当前价格（否则会立即触发止损）
		if decision.NewStopLoss >= currentPrice {
			return fmt.Errorf("多仓止损价格不能高于或等于当前价格: 当前价格 %.2f, 新止损 %.2f", currentPrice, decision.NewStopLoss)
		}
		// 如果当前价格 > 入场价（盈利），允许新止损 >= 入场价（锁定利润）
		// 如果当前价格 <= 入场价（亏损或持平），新止损应该 >= 入场价（限制亏损）
		if currentPrice > entryPrice {
			// 盈利状态：允许新止损 >= 入场价（锁定利润）
			log.Printf("  ✅ 多仓止损调整至 %.2f (入场价: %.2f, 当前价格: %.2f, 盈利状态)", decision.NewStopLoss, entryPrice, currentPrice)
		} else {
			// 亏损或持平状态：新止损应该 >= 入场价（限制亏损）
			if decision.NewStopLoss < entryPrice {
				return fmt.Errorf("多仓止损价格不能低于入场价: 入场价 %.2f, 新止损 %.2f", entryPrice, decision.NewStopLoss)
			}
			log.Printf("  ✅ 多仓止损调整至 %.2f (入场价: %.2f, 当前价格: %.2f)", decision.NewStopLoss, entryPrice, currentPrice)
		}
	} else if positionSide == "SHORT" {
		// 空仓止损逻辑：
		// 1. 开仓时设置初始止损：止损价 > 入场价（限制初始亏损）
		// 2. 持仓盈利后移动（下调）止损：新止损价 < 旧止损价，并且新止损价 < 入场价（锁定已有利润）
		// 3. 新止损必须 > 当前价格（否则会立即触发止损）
		if decision.NewStopLoss <= currentPrice {
			return fmt.Errorf("空仓止损价格不能低于或等于当前价格: 当前价格 %.2f, 新止损 %.2f", currentPrice, decision.NewStopLoss)
		}
		// 如果当前价格 < 入场价（盈利），允许新止损 < 入场价（锁定利润）
		// 如果当前价格 >= 入场价（亏损或持平），新止损应该 >= 入场价（限制亏损）
		if currentPrice < entryPrice {
			// 盈利状态：允许新止损 < 入场价（锁定利润）
			log.Printf("  ✅ 空仓止损下调至 %.2f 锁定利润 (入场价: %.2f, 当前价格: %.2f, 盈利状态)", decision.NewStopLoss, entryPrice, currentPrice)
		} else {
			// 亏损或持平状态：新止损应该 >= 入场价（限制亏损）
			if decision.NewStopLoss < entryPrice {
				return fmt.Errorf("空仓止损价格不能低于入场价: 入场价 %.2f, 新止损 %.2f", entryPrice, decision.NewStopLoss)
			}
			log.Printf("  ✅ 空仓止损调整至 %.2f (入场价: %.2f, 当前价格: %.2f)", decision.NewStopLoss, entryPrice, currentPrice)
		}
	}

	// ⚠️ 防御性检查：检测是否存在双向持仓（不应该出现，但提供保护）
	var hasOppositePosition bool
	oppositeSide := ""
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posSide, _ := pos["side"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 && strings.ToUpper(posSide) != positionSide {
			hasOppositePosition = true
			oppositeSide = strings.ToUpper(posSide)
			break
		}
	}

	if hasOppositePosition {
		log.Printf("  🚨 警告：检测到 %s 存在双向持仓（%s + %s），这违反了策略规则",
			decision.Symbol, positionSide, oppositeSide)
		log.Printf("  🚨 取消止损单将影响两个方向的订单，请检查是否为用户手动操作导致")
		log.Printf("  🚨 建议：手动平掉其中一个方向的持仓，或检查系统是否有BUG")
	}

	// 取消旧的止损单（只删除止损单，不影响止盈单）
	// 注意：如果存在双向持仓，这会删除两个方向的止损单
	if err := at.trader.CancelStopLossOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止损单失败: %v", err)
		// 不中断执行，继续设置新止损
	}

	// 调用交易所 API 修改止损
	quantity := math.Abs(positionAmt)
	err = at.trader.SetStopLoss(decision.Symbol, positionSide, quantity, decision.NewStopLoss)
	if err != nil {
		return fmt.Errorf("修改止损失败: %w", err)
	}

	return nil
}

// executeUpdateTakeProfitWithRecord 执行调整止盈并记录详细信息
func (at *AutoTrader) executeUpdateTakeProfitWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 调整止盈: %s → %.2f", decision.Symbol, decision.NewTakeProfit)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 验证新止盈价格合理性
	if positionSide == "LONG" && decision.NewTakeProfit <= marketData.CurrentPrice {
		return fmt.Errorf("多单止盈必须高于当前价格 (当前: %.2f, 新止盈: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}
	if positionSide == "SHORT" && decision.NewTakeProfit >= marketData.CurrentPrice {
		return fmt.Errorf("空单止盈必须低于当前价格 (当前: %.2f, 新止盈: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}

	// ⚠️ 防御性检查：检测是否存在双向持仓（不应该出现，但提供保护）
	var hasOppositePosition bool
	oppositeSide := ""
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posSide, _ := pos["side"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 && strings.ToUpper(posSide) != positionSide {
			hasOppositePosition = true
			oppositeSide = strings.ToUpper(posSide)
			break
		}
	}

	if hasOppositePosition {
		log.Printf("  🚨 警告：检测到 %s 存在双向持仓（%s + %s），这违反了策略规则",
			decision.Symbol, positionSide, oppositeSide)
		log.Printf("  🚨 取消止盈单将影响两个方向的订单，请检查是否为用户手动操作导致")
		log.Printf("  🚨 建议：手动平掉其中一个方向的持仓，或检查系统是否有BUG")
	}

	// 取消旧的止盈单（只删除止盈单，不影响止损单）
	// 注意：如果存在双向持仓，这会删除两个方向的止盈单
	if err := at.trader.CancelTakeProfitOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止盈单失败: %v", err)
		// 不中断执行，继续设置新止盈
	}

	// 调用交易所 API 修改止盈
	quantity := math.Abs(positionAmt)
	err = at.trader.SetTakeProfit(decision.Symbol, positionSide, quantity, decision.NewTakeProfit)
	if err != nil {
		return fmt.Errorf("修改止盈失败: %w", err)
	}

	log.Printf("  ✓ 止盈已调整: %.2f (当前价格: %.2f)", decision.NewTakeProfit, marketData.CurrentPrice)
	return nil
}

// executePartialCloseWithRecord 执行部分平仓并记录详细信息
func (at *AutoTrader) executePartialCloseWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📊 部分平仓: %s %.1f%%", decision.Symbol, decision.ClosePercentage)

	// 验证百分比范围
	if decision.ClosePercentage <= 0 || decision.ClosePercentage > 100 {
		return fmt.Errorf("平仓百分比必须在 0-100 之间，当前: %.1f", decision.ClosePercentage)
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 计算平仓数量
	totalQuantity := math.Abs(positionAmt)
	closeQuantity := totalQuantity * (decision.ClosePercentage / 100.0)
	actionRecord.Quantity = closeQuantity

	// ✅ Layer 2: 最小仓位检查（防止产生小额剩余）
	markPrice, ok := targetPosition["markPrice"].(float64)
	if !ok || markPrice <= 0 {
		return fmt.Errorf("无法解析当前价格，无法执行最小仓位检查")
	}

	currentPositionValue := totalQuantity * markPrice
	remainingQuantity := totalQuantity - closeQuantity
	remainingValue := remainingQuantity * markPrice

	const MIN_POSITION_VALUE = 10.0 // 最小持仓价值 10 USDT（對齊交易所底线，小仓位建议直接全平）

	if remainingValue > 0 && remainingValue <= MIN_POSITION_VALUE {
		log.Printf("⚠️ 检测到 partial_close 后剩余仓位 %.2f USDT < %.0f USDT",
			remainingValue, MIN_POSITION_VALUE)
		log.Printf("  → 当前仓位价值: %.2f USDT, 平仓 %.1f%%, 剩余: %.2f USDT",
			currentPositionValue, decision.ClosePercentage, remainingValue)
		log.Printf("  → 自动修正为全部平仓，避免产生无法平仓的小额剩余")

		// 🔄 自动修正为全部平仓
		if positionSide == "LONG" {
			decision.Action = "close_long"
			log.Printf("  ✓ 已修正为: close_long")
			return at.executeCloseLongWithRecord(decision, actionRecord)
		} else {
			decision.Action = "close_short"
			log.Printf("  ✓ 已修正为: close_short")
			return at.executeCloseShortWithRecord(decision, actionRecord)
		}
	}

	// 执行平仓
	var order map[string]interface{}
	if positionSide == "LONG" {
		order, err = at.trader.CloseLong(decision.Symbol, closeQuantity)
	} else {
		order, err = at.trader.CloseShort(decision.Symbol, closeQuantity)
	}

	if err != nil {
		return fmt.Errorf("部分平仓失败: %w", err)
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 部分平仓成功: 平仓 %.4f (%.1f%%), 剩余 %.4f",
		closeQuantity, decision.ClosePercentage, remainingQuantity)

	// ✅ Step 4: 恢复止盈止损（防止剩余仓位裸奔）
	// 重要：币安等交易所在部分平仓后会自动取消原有的 TP/SL 订单（因为数量不匹配）
	// 如果 AI 提供了新的止损止盈价格，则为剩余仓位重新设置保护
	if decision.NewStopLoss > 0 {
		log.Printf("  → 为剩余仓位 %.4f 恢复止损单: %.2f", remainingQuantity, decision.NewStopLoss)
		err = at.trader.SetStopLoss(decision.Symbol, positionSide, remainingQuantity, decision.NewStopLoss)
		if err != nil {
			log.Printf("  ⚠️ 恢复止损失败: %v（不影响平仓结果）", err)
		}
	}

	if decision.NewTakeProfit > 0 {
		log.Printf("  → 为剩余仓位 %.4f 恢复止盈单: %.2f", remainingQuantity, decision.NewTakeProfit)
		err = at.trader.SetTakeProfit(decision.Symbol, positionSide, remainingQuantity, decision.NewTakeProfit)
		if err != nil {
			log.Printf("  ⚠️ 恢复止盈失败: %v（不影响平仓结果）", err)
		}
	}

	// 如果 AI 没有提供新的止盈止损，记录警告
	if decision.NewStopLoss <= 0 && decision.NewTakeProfit <= 0 {
		log.Printf("  ⚠️⚠️⚠️ 警告: 部分平仓后AI未提供新的止盈止损价格")
		log.Printf("  → 剩余仓位 %.4f (价值 %.2f USDT) 目前没有止盈止损保护", remainingQuantity, remainingValue)
		log.Printf("  → 建议: 在 partial_close 决策中包含 new_stop_loss 和 new_take_profit 字段")
	}

	return nil
}

// GetID 获取trader ID
func (at *AutoTrader) GetID() string {
	return at.id
}

// GetDatabase 获取数据库引用
func (at *AutoTrader) GetDatabase() interface{} {
	return at.database
}

// GetName 获取trader名称
func (at *AutoTrader) GetName() string {
	return at.name
}

// GetAIModel 获取AI模型
func (at *AutoTrader) GetAIModel() string {
	return at.aiModel
}

// GetExchange 获取交易所
func (at *AutoTrader) GetExchange() string {
	return at.exchange
}

// SetCustomPrompt 设置自定义交易策略prompt
func (at *AutoTrader) SetCustomPrompt(prompt string) {
	at.customPrompt = prompt
}

// SetOverrideBasePrompt 设置是否覆盖基础prompt
func (at *AutoTrader) SetOverrideBasePrompt(override bool) {
	at.overrideBasePrompt = override
}

// SetSystemPromptTemplate 设置系统提示词模板
func (at *AutoTrader) SetSystemPromptTemplate(templateName string) {
	at.systemPromptTemplate = templateName
}

// GetSystemPromptTemplate 获取当前系统提示词模板名称
func (at *AutoTrader) GetSystemPromptTemplate() string {
	return at.systemPromptTemplate
}

// GetDecisionLogger 获取决策日志记录器
func (at *AutoTrader) GetDecisionLogger() *logger.DecisionLogger {
	return at.decisionLogger
}

// GetStatus 获取系统状态（用于API）
func (at *AutoTrader) GetStatus() map[string]interface{} {
	aiProvider := "DeepSeek"
	if at.config.UseQwen {
		aiProvider = "Qwen"
	}

	return map[string]interface{}{
		"trader_id":       at.id,
		"trader_name":     at.name,
		"ai_model":        at.aiModel,
		"exchange":        at.exchange,
		"is_running":      at.isRunning,
		"start_time":      at.startTime.Format(time.RFC3339),
		"runtime_minutes": int(time.Since(at.startTime).Minutes()),
		"call_count":      at.callCount,
		"initial_balance": at.initialBalance,
		"scan_interval":   at.config.ScanInterval.String(),
		"stop_until":      at.stopUntil.Format(time.RFC3339),
		"last_reset_time": at.lastResetTime.Format(time.RFC3339),
		"ai_provider":     aiProvider,
	}
}

// GetAccountInfo 获取账户信息（用于API）
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取余额失败: %w", err)
	}

	// 获取账户字段
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Total Equity = 钱包余额 + 未实现盈亏
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 获取持仓计算总保证金
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	totalMarginUsed := 0.0
	totalUnrealizedPnLCalculated := 0.0
	for _, pos := range positions {
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		totalUnrealizedPnLCalculated += unrealizedPnl

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	// 验证未实现盈亏的一致性（API值 vs 从持仓计算）
	diff := math.Abs(totalUnrealizedProfit - totalUnrealizedPnLCalculated)
	if diff > 0.1 { // 允许0.01 USDT的误差
		log.Printf("⚠️ 未实现盈亏不一致: API=%.4f, 计算=%.4f, 差异=%.4f",
			totalUnrealizedProfit, totalUnrealizedPnLCalculated, diff)
	}

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	} else {
		log.Printf("⚠️ Initial Balance异常: %.2f，无法计算PNL百分比", at.initialBalance)
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	return map[string]interface{}{
		// 核心字段
		"total_equity":      totalEquity,           // 账户净值 = wallet + unrealized
		"wallet_balance":    totalWalletBalance,    // 钱包余额（不含未实现盈亏）
		"unrealized_profit": totalUnrealizedProfit, // 未实现盈亏（交易所API官方值）
		"available_balance": availableBalance,      // 可用余额

		// 盈亏统计
		"total_pnl":       totalPnL,          // 总盈亏 = equity - initial
		"total_pnl_pct":   totalPnLPct,       // 总盈亏百分比
		"initial_balance": at.initialBalance, // 初始余额
		"daily_pnl":       at.dailyPnL,       // 日盈亏

		// 持仓信息
		"position_count":  len(positions),  // 持仓数量
		"margin_used":     totalMarginUsed, // 保证金占用
		"margin_used_pct": marginUsedPct,   // 保证金使用率
	}, nil
}

// GetPositions 获取持仓列表（用于API）
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		// 计算占用保证金
		marginUsed := (quantity * markPrice) / float64(leverage)

		// 计算盈亏百分比（基于保证金）
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		result = append(result, map[string]interface{}{
			"symbol":             symbol,
			"side":               side,
			"entry_price":        entryPrice,
			"mark_price":         markPrice,
			"quantity":           quantity,
			"leverage":           leverage,
			"unrealized_pnl":     unrealizedPnl,
			"unrealized_pnl_pct": pnlPct,
			"liquidation_price":  liquidationPrice,
			"margin_used":        marginUsed,
		})
	}

	return result, nil
}

// calculatePnLPercentage 计算盈亏百分比（基于保证金，自动考虑杠杆）
// 收益率 = 未实现盈亏 / 保证金 × 100%
func calculatePnLPercentage(unrealizedPnl, marginUsed float64) float64 {
	if marginUsed > 0 {
		return (unrealizedPnl / marginUsed) * 100
	}
	return 0.0
}

// sortDecisionsByPriority 对决策排序：先平仓，再开仓，最后hold/wait
// 这样可以避免换仓时仓位叠加超限
func sortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	// 定义优先级
	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short", "partial_close":
			return 1 // 最高优先级：先平仓（包括部分平仓）
		case "update_stop_loss", "update_take_profit":
			return 2 // 调整持仓止盈止损
		case "open_long", "open_short":
			return 3 // 次优先级：后开仓
		case "hold", "wait":
			return 4 // 最低优先级：观望
		default:
			return 999 // 未知动作放最后
		}
	}

	// 复制决策列表
	sorted := make([]decision.Decision, len(decisions))
	copy(sorted, decisions)

	// 按优先级排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// getCandidateCoins 获取交易员的候选币种列表
func (at *AutoTrader) getCandidateCoins() ([]decision.CandidateCoin, error) {
	if len(at.tradingCoins) == 0 {
		// 使用数据库配置的默认币种列表
		var candidateCoins []decision.CandidateCoin

		if len(at.defaultCoins) > 0 {
			// 使用数据库中配置的默认币种
			for _, coin := range at.defaultCoins {
				symbol := normalizeSymbol(coin)
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"default"}, // 标记为数据库默认币种
				})
			}

			log.Printf("📋 [%s] 使用数据库默认币种: %d个币种 %v",
				at.name, len(candidateCoins), at.defaultCoins)
			return candidateCoins, nil
		} else {
			// 如果数据库中没有配置默认币种，则使用AI500+OI Top作为fallback
			const ai500Limit = 20 // AI500取前20个评分最高的币种

			mergedPool, err := pool.GetMergedCoinPool(ai500Limit)
			if err != nil {
				return nil, fmt.Errorf("获取合并币种池失败: %w", err)
			}

			// 构建候选币种列表（包含来源信息）
			for _, symbol := range mergedPool.AllSymbols {
				sources := mergedPool.SymbolSources[symbol]
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: sources, // "ai500" 和/或 "oi_top"
				})
			}

			log.Printf("📋 [%s] 数据库无默认币种配置，使用AI500+OI Top: AI500前%d + OI_Top20 = 总计%d个候选币种",
				at.name, ai500Limit, len(candidateCoins))
			return candidateCoins, nil
		}
	} else {
		// 使用自定义币种列表
		var candidateCoins []decision.CandidateCoin
		for _, coin := range at.tradingCoins {
			// 确保币种格式正确（转为大写USDT交易对）
			symbol := normalizeSymbol(coin)
			candidateCoins = append(candidateCoins, decision.CandidateCoin{
				Symbol:  symbol,
				Sources: []string{"custom"}, // 标记为自定义来源
			})
		}

		log.Printf("📋 [%s] 使用自定义币种: %d个币种 %v",
			at.name, len(candidateCoins), at.tradingCoins)
		return candidateCoins, nil
	}
}

// normalizeSymbol 标准化币种符号（确保以USDT结尾）
func normalizeSymbol(symbol string) string {
	// 转为大写
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// 确保以USDT结尾
	if !strings.HasSuffix(symbol, "USDT") {
		symbol = symbol + "USDT"
	}

	return symbol
}

// 启动回撤监控
func (at *AutoTrader) startDrawdownMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(1 * time.Minute) // 每分钟检查一次
		defer ticker.Stop()

		log.Println("📊 启动持仓回撤监控（每分钟检查一次）")

		for {
			select {
			case <-ticker.C:
				at.checkPositionDrawdown()
			case <-at.stopMonitorCh:
				log.Println("⏹ 停止持仓回撤监控")
				return
			}
		}
	}()
}

// 检查持仓回撤情况
func (at *AutoTrader) checkPositionDrawdown() {
	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		log.Printf("❌ 回撤监控：获取持仓失败: %v", err)
		return
	}

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)

		// 计算当前盈亏百分比
		leverage := 10 // 默认值
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		var currentPnLPct float64
		if side == "long" {
			currentPnLPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			currentPnLPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		// 构造持仓唯一标识（区分多空）
		posKey := symbol + "_" + side

		// 获取该持仓的历史最高收益
		at.peakPnLCacheMutex.RLock()
		peakPnLPct, exists := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		if !exists {
			// 如果没有历史最高记录，使用当前盈亏作为初始值
			peakPnLPct = currentPnLPct
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		} else {
			// 更新峰值缓存
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		}

		// 计算回撤（从最高点下跌的幅度）
		var drawdownPct float64
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}

		// 检查平仓条件：收益大于5%且回撤超过40%
		if currentPnLPct > 5.0 && drawdownPct >= 40.0 {
			log.Printf("🚨 触发回撤平仓条件: %s %s | 当前收益: %.2f%% | 最高收益: %.2f%% | 回撤: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)

			// 执行平仓
			if err := at.emergencyClosePosition(symbol, side); err != nil {
				log.Printf("❌ 回撤平仓失败 (%s %s): %v", symbol, side, err)
			} else {
				log.Printf("✅ 回撤平仓成功: %s %s", symbol, side)
				// 平仓后清理该持仓的缓存
				at.ClearPeakPnLCache(symbol, side)
			}
		} else if currentPnLPct > 5.0 {
			// 记录接近平仓条件的情况（用于调试）
			log.Printf("📊 回撤监控: %s %s | 收益: %.2f%% | 最高: %.2f%% | 回撤: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)
		}
	}
}

// 紧急平仓函数
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	switch side {
	case "long":
		order, err := at.trader.CloseLong(symbol, 0) // 0 = 全部平仓
		if err != nil {
			return err
		}
		log.Printf("✅ 紧急平多仓成功，订单ID: %v", order["orderId"])
	case "short":
		order, err := at.trader.CloseShort(symbol, 0) // 0 = 全部平仓
		if err != nil {
			return err
		}
		log.Printf("✅ 紧急平空仓成功，订单ID: %v", order["orderId"])
	default:
		return fmt.Errorf("未知的持仓方向: %s", side)
	}

	return nil
}

// GetPeakPnLCache 获取最高收益缓存
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// 返回缓存的副本
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

// UpdatePeakPnL 更新最高收益缓存
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	if peak, exists := at.peakPnLCache[posKey]; exists {
		// 更新峰值（如果是多头，取较大值；如果是空头，currentPnLPct为负，也要比较）
		if currentPnLPct > peak {
			at.peakPnLCache[posKey] = currentPnLPct
		}
	} else {
		// 首次记录
		at.peakPnLCache[posKey] = currentPnLPct
	}
}

// ClearPeakPnLCache 清除指定持仓的峰值缓存
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	delete(at.peakPnLCache, posKey)
}
