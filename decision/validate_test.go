package decision

import (
	"nofx/market"
	"testing"
)

// TestLeverageFallback 测试杠杆超限时的自动修正功能
func TestLeverageFallback(t *testing.T) {
	tests := []struct {
		name            string
		decision        Decision
		accountEquity   float64
		btcEthLeverage  int
		altcoinLeverage int
		wantLeverage    int // 期望修正后的杠杆值
		wantError       bool
	}{
		{
			name: "山寨币杠杆超限_自动修正为上限",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        20, // 超过上限
				PositionSizeUSD: 100,
				StopLoss:        80,  // 止损 < 当前价(100)
				TakeProfit:      140, // 止盈 > 当前价(100)，且满足盈亏比≥2: (140-100)/(100-80) = 40/20 = 2.0
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5, // 上限 5x
			wantLeverage:    5, // 应该修正为 5
			wantError:       false,
		},
		{
			name: "BTC杠杆超限_自动修正为上限",
			decision: Decision{
				Symbol:          "BTCUSDT",
				Action:          "open_long",
				Leverage:        20, // 超过上限
				PositionSizeUSD: 1000,
				StopLoss:        93000,  // 止损 < 当前价(95000)
				TakeProfit:      103000, // 止盈 > 当前价(95000)，且满足盈亏比≥2: (103000-95000)/(95000-93000) = 8000/2000 = 4.0
			},
			accountEquity:   100,
			btcEthLeverage:  10, // 上限 10x
			altcoinLeverage: 5,
			wantLeverage:    10, // 应该修正为 10
			wantError:       false,
		},
		{
			name: "杠杆在上限内_不修正",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "open_short",
				Leverage:        5, // 未超限
				PositionSizeUSD: 500,
				StopLoss:        4000, // 止损 > 当前价(3500)
				TakeProfit:      2500, // 止盈 < 当前价(3500)，且满足盈亏比≥2: (3500-2500)/(4000-3500) = 1000/500 = 2.0
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantLeverage:    5, // 保持不变
			wantError:       false,
		},
		{
			name: "杠杆为0_应该报错",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        0, // 无效
				PositionSizeUSD: 100,
				StopLoss:        80,  // 止损 < 当前价(100)
				TakeProfit:      140, // 止盈 > 当前价(100)
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantLeverage:    0,
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 Context 对象以匹配新的函数签名
			ctx := &Context{
				Account: AccountInfo{
					TotalEquity: tt.accountEquity,
				},
				BTCETHLeverage:  tt.btcEthLeverage,
				AltcoinLeverage: tt.altcoinLeverage,
				MarketDataMap:   make(map[string]*market.Data),
			}

			// 为测试的币种添加市场数据（使用合理的价格）
			var currentPrice float64
			if tt.decision.Symbol == "BTCUSDT" {
				currentPrice = 95000.0
			} else if tt.decision.Symbol == "ETHUSDT" {
				currentPrice = 3500.0
			} else {
				currentPrice = 100.0 // SOLUSDT 等
			}

			ctx.MarketDataMap[tt.decision.Symbol] = &market.Data{
				CurrentPrice: currentPrice,
			}

			// 调用更新后的函数签名
			err := validateDecision(&tt.decision, ctx, "正常模式")

			// 检查错误状态
			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

			// 如果不应该报错，检查杠杆是否被正确修正
			if !tt.wantError && tt.decision.Leverage != tt.wantLeverage {
				t.Errorf("Leverage not corrected: got %d, want %d", tt.decision.Leverage, tt.wantLeverage)
			}
		})
	}
}
