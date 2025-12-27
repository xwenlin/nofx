package config

import (
	"database/sql"
	"fmt"
)

// GetTradesByTraderWithPagination 获取交易记录（支持分页）
func (d *Database) GetTradesByTraderWithPagination(traderID string, page, pageSize int) ([]*TradeRecord, int, error) {
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

	// 先获取总数
	countQuery := `SELECT COUNT(*) FROM trades WHERE trader_id = ?`
	var totalCount int
	err := d.db.QueryRow(countQuery, traderID).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("查询交易记录总数失败: %w", err)
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 获取分页数据
	query := `
		SELECT id, trader_id, symbol, side, open_time, close_time, open_price, close_price, 
		       quantity, leverage, pnl, pnl_pct, close_reason, order_id_open, order_id_close, 
		       was_stop_loss, created_at, updated_at
		FROM trades
		WHERE trader_id = ?
		ORDER BY open_time DESC
		LIMIT ? OFFSET ?
	`

	rows, err := d.db.Query(query, traderID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询交易记录失败: %w", err)
	}
	defer rows.Close()

	var trades []*TradeRecord
	for rows.Next() {
		var trade TradeRecord
		var closeTime sql.NullTime
		var closePrice, pnl, pnlPct sql.NullFloat64
		var closeReason sql.NullString
		var orderIDClose sql.NullInt64
		var wasStopLoss sql.NullBool

		err := rows.Scan(
			&trade.ID, &trade.TraderID, &trade.Symbol, &trade.Side,
			&trade.OpenTime, &closeTime, &trade.OpenPrice, &closePrice,
			&trade.Quantity, &trade.Leverage, &pnl, &pnlPct,
			&closeReason, &trade.OrderIDOpen, &orderIDClose, &wasStopLoss,
			&trade.CreatedAt, &trade.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("扫描交易记录失败: %w", err)
		}

		if closeTime.Valid {
			trade.CloseTime = closeTime.Time
		}
		if closePrice.Valid {
			trade.ClosePrice = closePrice.Float64
		}
		if pnl.Valid {
			trade.PnL = pnl.Float64
		}
		if pnlPct.Valid {
			trade.PnLPct = pnlPct.Float64
		}
		if closeReason.Valid {
			trade.CloseReason = closeReason.String
		}
		if orderIDClose.Valid {
			trade.OrderIDClose = orderIDClose.Int64
		}
		if wasStopLoss.Valid {
			trade.WasStopLoss = wasStopLoss.Bool
		}

		trades = append(trades, &trade)
	}

	return trades, totalCount, nil
}
