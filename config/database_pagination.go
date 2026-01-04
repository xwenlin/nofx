package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// GetDecisionLogsWithPagination 获取决策日志（支持分页和过滤）
// actionFilter: "all", "has_trading", "wait_only", "open_only", "close_only"
// statusFilter: "all", "decision_failed", "action_failed"
// startTime, endTime: 时间过滤，如果为空则不过滤
func (d *Database) GetDecisionLogsWithPagination(traderID string, page, pageSize int, actionFilter string, statusFilter string, startTime *time.Time, endTime *time.Time) ([]*DecisionLog, int, error) {
	// 计算分页
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	// 先获取所有数据用于过滤（因为过滤需要在应用层进行）
	// 构建SQL查询，添加时间过滤条件
	// 注意：列表查询时不加载长文本字段（system_prompt, input_prompt, cot_trace）以提高性能
	query := `
		SELECT id, trader_id, cycle_number, timestamp, 
		       COALESCE(decision_json, '') as decision_json,
		       COALESCE(decisions, '') as decisions,
		       COALESCE(account_state, '') as account_state,
		       COALESCE(positions, '') as positions,
		       COALESCE(execution_log, '') as execution_log,
		       success, error, 
		       COALESCE(ai_request_duration_ms, 0) as ai_request_duration_ms,
		       created_at
		FROM decisions
		WHERE trader_id = ?
	`
	args := []interface{}{traderID}

	// 添加时间过滤条件
	if startTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, *startTime)
	}
	if endTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, *endTime)
	}

	query += " ORDER BY timestamp DESC"

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询决策日志失败: %w", err)
	}
	defer rows.Close()

	var allLogs []*DecisionLog
	for rows.Next() {
		var l DecisionLog
		err := rows.Scan(
			&l.ID, &l.TraderID, &l.CycleNumber, &l.Timestamp,
			&l.DecisionJSON, &l.Decisions,
			&l.AccountState, &l.Positions, &l.ExecutionLog,
			&l.Success, &l.Error, &l.AIRequestDurationMs, &l.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("扫描决策日志失败: %w", err)
		}
		allLogs = append(allLogs, &l)
	}

	// 在应用层过滤（先按动作过滤）
	var actionFilteredLogs []*DecisionLog
	if actionFilter == "all" || actionFilter == "" {
		actionFilteredLogs = allLogs
	} else {
		for _, log := range allLogs {
			// 解析 decision_json 来检查动作类型
			// DecisionJSON 存储的是 Decision[] 数组格式，不是 {"decisions": [...]} 格式
			var decisions []struct {
				Action string `json:"action"`
			}

			// 尝试解析为数组格式
			if err := json.Unmarshal([]byte(log.DecisionJSON), &decisions); err != nil {
				// 如果解析失败，尝试解析为对象格式（兼容旧数据）
				var decisionData struct {
					Decisions []struct {
						Action string `json:"action"`
					} `json:"decisions"`
				}
				if err2 := json.Unmarshal([]byte(log.DecisionJSON), &decisionData); err2 != nil {
					// 两种格式都解析失败，跳过这个日志
					continue
				} else {
					decisions = decisionData.Decisions
				}
			}

			hasWait := false
			hasOpen := false
			hasClose := false

			for _, decision := range decisions {
				action := strings.ToLower(decision.Action)
				if strings.Contains(action, "wait") || strings.Contains(action, "hold") {
					hasWait = true
				}
				if strings.Contains(action, "open") {
					hasOpen = true
				}
				if strings.Contains(action, "close") || strings.Contains(action, "partial_close") {
					hasClose = true
				}
			}

			hasTrading := hasOpen || hasClose

			switch actionFilter {
			case "has_trading":
				if hasTrading {
					actionFilteredLogs = append(actionFilteredLogs, log)
				}
			case "wait_only":
				if hasWait && !hasTrading {
					actionFilteredLogs = append(actionFilteredLogs, log)
				}
			case "open_only":
				if hasOpen {
					actionFilteredLogs = append(actionFilteredLogs, log)
				}
			case "close_only":
				if hasClose {
					actionFilteredLogs = append(actionFilteredLogs, log)
				}
			}
		}
	}

	// 再按状态过滤
	var filteredLogs []*DecisionLog
	if statusFilter == "all" || statusFilter == "" {
		filteredLogs = actionFilteredLogs
	} else {
		for _, log := range actionFilteredLogs {
			switch statusFilter {
			case "decision_failed":
				// 整个决策失败
				if !log.Success {
					filteredLogs = append(filteredLogs, log)
				}
			case "action_failed":
				// 决策动作执行失败（至少有一个动作失败）
				hasFailedAction := false
				hasChecked := false

				// 优先从 Content 解析（包含正确的 Success 字段）
				if log.Content != "" {
					var fullRecord struct {
						Decisions []struct {
							Success bool `json:"success"`
						} `json:"decisions"`
					}
					if err := json.Unmarshal([]byte(log.Content), &fullRecord); err == nil {
						hasChecked = true
						for _, decision := range fullRecord.Decisions {
							if !decision.Success {
								hasFailedAction = true
								break
							}
						}
					}
				}

				// 如果 Content 解析失败，尝试从 DecisionJSON 解析
				// 注意：DecisionJSON 可能不包含 Success 字段（AI 原始返回），所以需要检查字段是否存在
				if !hasChecked && log.DecisionJSON != "" {
					var decisions []struct {
						Success *bool `json:"success"` // 使用指针，因为字段可能不存在
					}
					if err := json.Unmarshal([]byte(log.DecisionJSON), &decisions); err == nil {
						hasChecked = true
						for _, decision := range decisions {
							// 只有当 Success 字段存在且为 false 时才算失败
							if decision.Success != nil && !*decision.Success {
								hasFailedAction = true
								break
							}
						}
					} else {
						// 尝试解析为对象格式（兼容旧数据）
						var decisionData struct {
							Decisions []struct {
								Success *bool `json:"success"`
							} `json:"decisions"`
						}
						if err2 := json.Unmarshal([]byte(log.DecisionJSON), &decisionData); err2 == nil {
							hasChecked = true
							for _, decision := range decisionData.Decisions {
								if decision.Success != nil && !*decision.Success {
									hasFailedAction = true
									break
								}
							}
						}
					}
				}

				// 只有当成功检查到有失败的动作时才添加
				if hasChecked && hasFailedAction {
					filteredLogs = append(filteredLogs, log)
				}
			}
		}
	}

	// 应用分页（在过滤后）
	totalCount := len(filteredLogs)
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= totalCount {
		return []*DecisionLog{}, totalCount, nil
	}
	if end > totalCount {
		end = totalCount
	}

	return filteredLogs[start:end], totalCount, nil
}
