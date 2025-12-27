package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GetDecisionLogsWithPagination 获取决策日志（支持分页和过滤）
// actionFilter: "all", "has_trading", "wait_only", "open_only", "close_only"
func (d *Database) GetDecisionLogsWithPagination(traderID string, page, pageSize int, actionFilter string) ([]*DecisionLog, int, error) {
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
	query := `
		SELECT id, trader_id, cycle_number, timestamp, content, 
		       COALESCE(system_prompt, '') as system_prompt,
		       COALESCE(input_prompt, '') as input_prompt, 
		       COALESCE(cot_trace, '') as cot_trace, 
		       COALESCE(decision_json, '') as decision_json,
		       COALESCE(account_state, '') as account_state,
		       COALESCE(positions, '') as positions,
		       COALESCE(execution_log, '') as execution_log,
		       success, error, 
		       COALESCE(ai_request_duration_ms, 0) as ai_request_duration_ms,
		       created_at
		FROM decisions
		WHERE trader_id = ?
		ORDER BY timestamp DESC
	`

	rows, err := d.db.Query(query, traderID)
	if err != nil {
		return nil, 0, fmt.Errorf("查询决策日志失败: %w", err)
	}
	defer rows.Close()

	var allLogs []*DecisionLog
	for rows.Next() {
		var l DecisionLog
		err := rows.Scan(
			&l.ID, &l.TraderID, &l.CycleNumber, &l.Timestamp, &l.Content,
			&l.SystemPrompt, &l.InputPrompt, &l.CoTTrace, &l.DecisionJSON,
			&l.AccountState, &l.Positions, &l.ExecutionLog,
			&l.Success, &l.Error, &l.AIRequestDurationMs, &l.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("扫描决策日志失败: %w", err)
		}
		allLogs = append(allLogs, &l)
	}

	// 在应用层过滤
	var filteredLogs []*DecisionLog
	if actionFilter == "all" || actionFilter == "" {
		filteredLogs = allLogs
	} else {
		for _, log := range allLogs {
			// 解析 decision_json 来检查动作类型
			var decisionData struct {
				Decisions []struct {
					Action string `json:"action"`
				} `json:"decisions"`
			}
			if err := json.Unmarshal([]byte(log.DecisionJSON), &decisionData); err != nil {
				// 如果解析失败，跳过这个日志
				continue
			}

			hasWait := false
			hasOpen := false
			hasClose := false

			for _, decision := range decisionData.Decisions {
				action := strings.ToLower(decision.Action)
				if strings.Contains(action, "wait") {
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
					filteredLogs = append(filteredLogs, log)
				}
			case "wait_only":
				if hasWait && !hasTrading {
					filteredLogs = append(filteredLogs, log)
				}
			case "open_only":
				if hasOpen {
					filteredLogs = append(filteredLogs, log)
				}
			case "close_only":
				if hasClose {
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
