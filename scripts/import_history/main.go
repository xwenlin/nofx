package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"nofx/config"
	"nofx/logger"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var (
		logDir   = flag.String("log-dir", "decision_logs", "决策日志目录路径")
		dbPath   = flag.String("db", "config.db", "数据库文件路径")
		traderID = flag.String("trader-id", "", "交易员ID（如果为空，将从目录结构推断）")
		dryRun   = flag.Bool("dry-run", false, "仅显示将要导入的数据，不实际写入数据库")
		verbose  = flag.Bool("verbose", false, "显示详细信息")
	)
	flag.Parse()

	fmt.Println("╔════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   历史数据导入工具                                    ║")
	fmt.Println("║              从日志文件导入决策日志和交易记录到数据库                ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 打开数据库
	db, err := config.NewDatabase(*dbPath)
	if err != nil {
		log.Fatalf("❌ 打开数据库失败: %v", err)
	}
	defer db.Close()

	fmt.Printf("✓ 数据库已打开: %s\n", *dbPath)
	fmt.Println()

	// 扫描日志目录
	logDirPath := *logDir
	if !filepath.IsAbs(logDirPath) {
		cwd, _ := os.Getwd()
		logDirPath = filepath.Join(cwd, logDirPath)
	}

	fmt.Printf("📂 扫描日志目录: %s\n", logDirPath)
	fmt.Println()

	// 统计信息
	stats := struct {
		decisionFiles int
		decisions     int
		trades        int
		errors        int
	}{}

	// 收集所有 JSON 文件
	var jsonFiles []string
	err = filepath.Walk(logDirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 只处理 JSON 文件
		if !strings.HasSuffix(strings.ToLower(path), ".json") {
			return nil
		}

		// 跳过非决策日志文件
		if !strings.Contains(filepath.Base(path), "decision_") {
			return nil
		}

		jsonFiles = append(jsonFiles, path)
		return nil
	})

	if err != nil {
		log.Fatalf("❌ 扫描目录失败: %v", err)
	}

	if len(jsonFiles) == 0 {
		fmt.Println("⚠️  未找到任何决策日志文件")
		return
	}

	fmt.Printf("📄 找到 %d 个日志文件\n", len(jsonFiles))
	fmt.Println()

	// 按文件名排序（确保按时间顺序处理）
	// 文件名格式：decision_YYYYMMDD_HHMMSS_cycleN.json
	// 排序后可以确保按时间顺序处理
	// 注意：Go 的 filepath.Walk 不保证顺序，但通常按文件系统顺序
	// 如果需要严格按时间排序，可以在这里对 jsonFiles 进行排序

	// 处理每个文件
	for _, path := range jsonFiles {
		stats.decisionFiles++

		// 从路径推断 trader_id（如果未指定）
		currentTraderID := *traderID
		if currentTraderID == "" {
			// 尝试从路径中提取：decision_logs/{trader_id}/decision_*.json
			relPath, _ := filepath.Rel(logDirPath, path)
			parts := strings.Split(relPath, string(filepath.Separator))
			if len(parts) >= 2 {
				currentTraderID = parts[0]
			} else {
				// 如果直接在 decision_logs 目录下，尝试从文件名推断
				// 或者使用默认值
				currentTraderID = "default"
			}
		}

		if *verbose {
			fmt.Printf("📄 处理文件: %s (trader_id: %s)\n", path, currentTraderID)
		}

		// 读取 JSON 文件
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("⚠️  读取文件失败 %s: %v\n", path, err)
			stats.errors++
			continue
		}

		// 解析决策记录
		var record logger.DecisionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			fmt.Printf("⚠️  解析JSON失败 %s: %v\n", path, err)
			stats.errors++
			continue
		}

		// 导入决策日志
		if err := importDecisionLog(db, currentTraderID, &record, data, *dryRun, *verbose); err != nil {
			fmt.Printf("⚠️  导入决策日志失败 %s: %v\n", path, err)
			stats.errors++
		} else {
			stats.decisions++
		}

		// 提取并导入交易记录
		tradesImported, err := extractAndImportTrades(db, currentTraderID, &record, *dryRun, *verbose)
		if err != nil {
			fmt.Printf("⚠️  提取交易记录失败 %s: %v\n", path, err)
			stats.errors++
		} else {
			stats.trades += tradesImported
		}
	}

	// 输出统计信息
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                           导入完成                                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════════╝")
	fmt.Printf("📊 统计信息:\n")
	fmt.Printf("  - 处理的文件数: %d\n", stats.decisionFiles)
	fmt.Printf("  - 导入的决策日志: %d\n", stats.decisions)
	fmt.Printf("  - 导入的交易记录: %d\n", stats.trades)
	fmt.Printf("  - 错误数: %d\n", stats.errors)
	if *dryRun {
		fmt.Println()
		fmt.Println("⚠️  这是试运行模式，未实际写入数据库")
		fmt.Println("   使用 --dry-run=false 来实际导入数据")
	}
}

// importDecisionLog 导入决策日志到数据库
func importDecisionLog(db *config.Database, traderID string, record *logger.DecisionRecord, rawData []byte, dryRun bool, verbose bool) error {
	// 检查是否已存在（根据 trader_id, cycle_number, timestamp）
	existingLogs, err := db.GetDecisionLogs(traderID, 10000)
	if err == nil {
		for _, existing := range existingLogs {
			if existing.CycleNumber == record.CycleNumber &&
				existing.Timestamp.Equal(record.Timestamp) {
				if verbose {
					fmt.Printf("  ⏭️  决策日志已存在 (Cycle #%d, %s)\n", record.CycleNumber, record.Timestamp.Format("2006-01-02 15:04:05"))
				}
				return nil // 已存在，跳过
			}
		}
	}

	if dryRun {
		fmt.Printf("  📝 [试运行] 将导入决策日志: Cycle #%d, %s\n", record.CycleNumber, record.Timestamp.Format("2006-01-02 15:04:05"))
		return nil
	}

	// 序列化复杂对象
	execLogJSON, _ := json.Marshal(record.ExecutionLog)
	accountStateJSON, _ := json.Marshal(record.AccountState)
	positionsJSON, _ := json.Marshal(record.Positions)

	logEntry := &config.DecisionLog{
		TraderID:            traderID,
		CycleNumber:         record.CycleNumber,
		Timestamp:           record.Timestamp,
		Content:             string(rawData),
		SystemPrompt:        record.SystemPrompt,
		InputPrompt:         record.InputPrompt,
		CoTTrace:            record.CoTTrace,
		DecisionJSON:        record.DecisionJSON,
		AccountState:        string(accountStateJSON),
		Positions:           string(positionsJSON),
		ExecutionLog:        string(execLogJSON),
		Success:             record.Success,
		Error:               record.ErrorMessage,
		AIRequestDurationMs: record.AIRequestDurationMs,
	}

	if err := db.CreateDecisionLog(logEntry); err != nil {
		return fmt.Errorf("创建决策日志失败: %w", err)
	}

	if verbose {
		fmt.Printf("  ✓ 决策日志已导入 (ID: %d, Cycle #%d)\n", logEntry.ID, record.CycleNumber)
	}

	return nil
}

// extractAndImportTrades 从决策记录中提取交易记录并导入
func extractAndImportTrades(db *config.Database, traderID string, record *logger.DecisionRecord, dryRun bool, verbose bool) (int, error) {
	if len(record.Decisions) == 0 {
		return 0, nil
	}

	tradesImported := 0

	// 获取现有的未平仓交易（用于匹配平仓）
	openTrades, err := db.GetOpenTrades(traderID)
	if err != nil {
		// 如果获取失败，继续处理（可能是第一次导入）
		openTrades = []*config.TradeRecord{}
	}

	// 创建未平仓交易的映射：symbol_side -> TradeRecord（使用最新的开仓记录）
	openTradesMap := make(map[string]*config.TradeRecord)
	for _, trade := range openTrades {
		key := fmt.Sprintf("%s_%s", trade.Symbol, trade.Side)
		// 如果已存在，保留时间更早的（开仓时间更早的）
		if existing, exists := openTradesMap[key]; !exists || trade.OpenTime.Before(existing.OpenTime) {
			openTradesMap[key] = trade
		}
	}

	// 处理每个决策动作
	for _, action := range record.Decisions {
		if !action.Success {
			continue // 跳过失败的交易
		}

		symbol := action.Symbol
		side := ""
		if action.Action == "open_long" || action.Action == "close_long" || action.Action == "partial_close" || action.Action == "auto_close_long" {
			side = "long"
		} else if action.Action == "open_short" || action.Action == "close_short" || action.Action == "auto_close_short" {
			side = "short"
		}

		if side == "" {
			continue // 跳过非交易动作（如 update_stop_loss）
		}

		posKey := fmt.Sprintf("%s_%s", symbol, side)

		switch action.Action {
		case "open_long", "open_short":
			// 检查是否已存在相同的开仓记录（根据 symbol, side, open_time 和 order_id）
			exists := false
			allTrades, _ := db.GetTradesByTrader(traderID, 10000) // 获取所有交易记录用于检查
			for _, trade := range allTrades {
				if trade.Symbol == symbol && trade.Side == side &&
					trade.OpenTime.Equal(action.Timestamp) &&
					trade.OrderIDOpen == action.OrderID {
					exists = true
					break
				}
			}

			if exists {
				if verbose {
					fmt.Printf("  ⏭️  交易记录已存在 (开仓: %s %s, %s)\n", symbol, side, action.Timestamp.Format("2006-01-02 15:04:05"))
				}
				// 更新映射（即使已存在，也要更新映射以便后续匹配）
				key := fmt.Sprintf("%s_%s", symbol, side)
				for _, trade := range allTrades {
					if trade.Symbol == symbol && trade.Side == side && trade.CloseTime.IsZero() {
						openTradesMap[key] = trade
						break
					}
				}
				continue
			}

			// 创建开仓记录
			trade := &config.TradeRecord{
				TraderID:    traderID,
				Symbol:      symbol,
				Side:        side,
				OpenTime:    action.Timestamp,
				OpenPrice:   action.Price,
				Quantity:    action.Quantity,
				Leverage:    action.Leverage,
				OrderIDOpen: action.OrderID,
			}

			if dryRun {
				fmt.Printf("  📝 [试运行] 将创建开仓记录: %s %s @ %.4f, 数量: %.4f, 杠杆: %dx\n",
					symbol, side, action.Price, action.Quantity, action.Leverage)
			} else {
				if err := db.CreateTrade(trade); err != nil {
					return tradesImported, fmt.Errorf("创建交易记录失败: %w", err)
				}
				if verbose {
					fmt.Printf("  ✓ 开仓记录已导入 (ID: %d, %s %s)\n", trade.ID, symbol, side)
				}
			}

			// 更新映射（用于后续匹配）
			openTradesMap[posKey] = trade
			tradesImported++

		case "close_long", "close_short", "auto_close_long", "auto_close_short", "partial_close":
			// 查找对应的开仓记录
			openTrade, exists := openTradesMap[posKey]
			if !exists {
				// 尝试从数据库查找
				allOpenTrades, _ := db.GetOpenTrades(traderID)
				for _, t := range allOpenTrades {
					if t.Symbol == symbol && t.Side == side {
						openTrade = t
						openTradesMap[posKey] = t
						exists = true
						break
					}
				}
			}

			if !exists {
				if verbose {
					fmt.Printf("  ⚠️  未找到对应的开仓记录 (平仓: %s %s, %s)\n", symbol, side, action.Timestamp.Format("2006-01-02 15:04:05"))
				}
				continue
			}

			// 计算盈亏
			var pnl float64
			if side == "long" {
				pnl = (action.Price - openTrade.OpenPrice) * openTrade.Quantity
			} else {
				pnl = (openTrade.OpenPrice - action.Price) * openTrade.Quantity
			}

			marginUsed := (openTrade.Quantity * openTrade.OpenPrice) / float64(openTrade.Leverage)
			pnlPct := 0.0
			if marginUsed > 0 {
				pnlPct = (pnl / marginUsed) * 100
			}

			// 判断平仓原因
			closeReason := "manual"
			wasStopLoss := false
			if action.Action == "auto_close_long" || action.Action == "auto_close_short" {
				if pnl < 0 {
					closeReason = "stop_loss"
					wasStopLoss = true
				} else {
					closeReason = "take_profit"
				}
			}

			if dryRun {
				fmt.Printf("  📝 [试运行] 将更新平仓记录: %s %s, PnL: %.2f USDT (%.2f%%)\n",
					symbol, side, pnl, pnlPct)
			} else {
				if err := db.UpdateTradeClose(traderID, symbol, side, action.Timestamp, action.Price, pnl, pnlPct, closeReason, action.OrderID, wasStopLoss); err != nil {
					return tradesImported, fmt.Errorf("更新交易记录失败: %w", err)
				}
				if verbose {
					fmt.Printf("  ✓ 平仓记录已更新 (%s %s, PnL: %.2f USDT)\n", symbol, side, pnl)
				}
			}

			// 从映射中删除（已平仓）
			delete(openTradesMap, posKey)
			tradesImported++
		}
	}

	return tradesImported, nil
}
