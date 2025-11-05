# 提示词数据需求分析报告

## 📋 提示词要求的数据

根据 `fusion_adaptive_taro.txt`，提示词要求以下数据：

### 1. 四个时间框架序列（每个10个数据点）

#### 3分钟序列 ✅ 已提供
- Mid prices ✅
- EMA20 ✅
- MACD ✅
- RSI7 ✅
- RSI14 ✅
- **Volumes**: 成交量序列 ❌ **缺失**
- **BuySellRatios**: 买卖压力比 ❌ **缺失**

#### 15分钟序列 ❌ **缺失**
- Mid prices ❌
- EMA20 ❌
- MACD ❌
- RSI7 ❌
- RSI14 ❌

#### 1小时序列 ❌ **缺失**
- Mid prices ❌
- EMA20 ❌
- MACD ❌
- RSI7 ❌
- RSI14 ❌

#### 4小时序列 ✅ 已提供（部分）
- Mid prices ✅（通过4h序列）
- EMA20 ✅
- EMA50 ✅
- MACD ✅
- RSI14 ✅
- **RSI7** ❌ **缺失**（只有RSI14）

### 2. BTC状态检查需要的数据

**提示词要求**：
- BTC 15m MACD ❌ **缺失**
- BTC 1h MACD ❌ **缺失**
- BTC 4h MACD ✅ **已提供**

**当前提供**：
- BTC只有当前价格、1h价格变化%、4h价格变化%、当前MACD（3m）、当前RSI7（3m）

### 3. 其他数据

- **持仓量（OI）变化**：✅ 已提供（Latest和Average，但提示词要求"变化>+5%"，需要计算变化百分比）
- **资金费率**：✅ 已提供
- **成交量序列**：❌ **缺失**（只有4h的CurrentVolume和AverageVolume）
- **买卖压力比（BuySellRatios）**：❌ **缺失**（Kline中有TakerBuyBaseVolume和TakerBuyQuoteVolume，但未计算和传递）

---

## 🔍 代码实际提供的数据

### 从 `market/data.go` 分析：

```go
// Get函数只获取：
- klines3m (3分钟K线，最近10个) ✅
- klines4h (4小时K线，最近10个) ✅
- ❌ 没有获取15m和1h的K线数据
```

### 从 `market.Format()` 函数分析：

**实际输出**：
1. **当前指标**（基于3m）：
   - current_price ✅
   - current_ema20 ✅
   - current_macd ✅
   - current_rsi7 ✅

2. **Open Interest**：
   - Latest ✅
   - Average ✅
   - ❌ **没有计算变化百分比**

3. **Funding Rate**：
   - ✅ 已提供

4. **Intraday series (3分钟)**：
   - Mid prices ✅ (10个数据点)
   - EMA20 ✅ (10个数据点)
   - MACD ✅ (10个数据点)
   - RSI7 ✅ (10个数据点)
   - RSI14 ✅ (10个数据点)
   - ❌ **Volumes序列缺失**
   - ❌ **BuySellRatios缺失**

5. **Longer-term context (4小时)**：
   - EMA20 ✅
   - EMA50 ✅
   - ATR3 ✅
   - ATR14 ✅
   - CurrentVolume ✅
   - AverageVolume ✅
   - MACD序列 ✅ (10个数据点)
   - RSI14序列 ✅ (10个数据点)
   - ❌ **RSI7缺失**（提示词要求RSI7）
   - ❌ **Mid prices序列缺失**（只有EMA、MACD、RSI序列）

---

## ❌ 缺失的关键数据

### 1. **15分钟和1小时序列**（严重缺失）

**影响**：
- ❌ 无法进行多周期趋势确认（3m/15m/1h/4h）
- ❌ 无法检查BTC的15m和1h MACD方向
- ❌ 无法识别短期震荡区间（15m）
- ❌ 无法确认中期支撑压力（1h）

**提示词要求**：
```
开仓前必须同时检查 3分钟、15分钟、1小时、4小时 的K线形态
BTC 15m MACD、1h MACD、4h MACD方向
```

### 2. **买卖压力比（BuySellRatios）**（缺失）

**影响**：
- ❌ 无法在多空确认清单中填写BuySellRatio项
- ❌ 无法判断买卖压力比（>0.6多方强，<0.4空方强）

**数据来源**：
- Kline结构中有 `TakerBuyBaseVolume` 和 `TakerBuyQuoteVolume`
- 可以计算：`BuySellRatio = TakerBuyBaseVolume / TotalVolume`
- 但代码中没有计算和传递

### 3. **成交量序列（Volumes）**（缺失）

**影响**：
- ❌ 无法检测放量（>1.5x均量）
- ❌ 无法判断"价格突破但成交量萎缩"

**当前提供**：
- 只有4h的CurrentVolume和AverageVolume（单个值）
- 没有3m/15m/1h的成交量序列

### 4. **OI变化百分比**（需要计算）

**影响**：
- ❌ 无法判断"OI变化>+5%"（提示词要求）

**当前提供**：
- OI Latest和Average
- 但没有计算变化百分比

### 5. **4小时RSI7**（缺失）

**提示词要求**：
- 4小时序列需要RSI7和RSI14
- 当前只提供RSI14

---

## 📊 数据完整性对比表

| 数据项 | 提示词要求 | 实际提供 | 状态 |
|--------|-----------|---------|------|
| **3分钟序列** |
| Mid prices | ✅ 10个数据点 | ✅ 已提供 | ✅ |
| EMA20 | ✅ 10个数据点 | ✅ 已提供 | ✅ |
| MACD | ✅ 10个数据点 | ✅ 已提供 | ✅ |
| RSI7 | ✅ 10个数据点 | ✅ 已提供 | ✅ |
| RSI14 | ✅ 10个数据点 | ✅ 已提供 | ✅ |
| Volumes | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| BuySellRatios | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| **15分钟序列** |
| Mid prices | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| EMA20 | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| MACD | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| RSI7 | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| RSI14 | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| **1小时序列** |
| Mid prices | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| EMA20 | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| MACD | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| RSI7 | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| RSI14 | ✅ 10个数据点 | ❌ 缺失 | ❌ |
| **4小时序列** |
| Mid prices | ✅ 10个数据点 | ⚠️ 部分（通过MACD/RSI序列） | ⚠️ |
| EMA20 | ✅ | ✅ 已提供 | ✅ |
| EMA50 | ✅ | ✅ 已提供 | ✅ |
| MACD | ✅ 10个数据点 | ✅ 已提供 | ✅ |
| RSI7 | ✅ 10个数据点 | ❌ 缺失（只有RSI14） | ❌ |
| RSI14 | ✅ 10个数据点 | ✅ 已提供 | ✅ |
| **BTC状态检查** |
| BTC 15m MACD | ✅ 必需 | ❌ 缺失 | ❌ |
| BTC 1h MACD | ✅ 必需 | ❌ 缺失 | ❌ |
| BTC 4h MACD | ✅ 必需 | ✅ 已提供 | ✅ |
| **其他数据** |
| OI变化百分比 | ✅ >+5% | ❌ 缺失（只有Latest/Average） | ❌ |
| 资金费率 | ✅ | ✅ 已提供 | ✅ |
| 成交量序列 | ✅ 各时间框架 | ⚠️ 只有4h的单个值 | ⚠️ |

---

## 🚨 关键问题

### 问题1：缺少15m和1h数据（最严重）

**影响**：
- 无法进行多周期趋势确认（提示词的核心要求）
- 无法检查BTC的15m和1h MACD（BTC状态检查的关键）
- 无法识别短期震荡区间和中长期趋势

**解决方案**：
```go
// 在 market/data.go 的 Get 函数中添加：
klines15m, err := WSMonitorCli.GetCurrentKlines(symbol, "15m")
klines1h, err := WSMonitorCli.GetCurrentKlines(symbol, "1h")

// 计算15m和1h的序列数据
intraday15mData := calculateIntradaySeries(klines15m)
intraday1hData := calculateIntradaySeries(klines1h)
```

### 问题2：缺少BuySellRatios（买卖压力比）

**影响**：
- 多空确认清单第4项无法填写
- 无法判断买卖压力比（>0.6多方强，<0.4空方强）

**数据来源**：
- Kline结构中有 `TakerBuyBaseVolume` 和 `Volume`
- 计算公式：`BuySellRatio = TakerBuyBaseVolume / Volume`

**解决方案**：
```go
// 在 calculateIntradaySeries 中添加：
BuySellRatios []float64

// 计算：
for i := start; i < len(klines); i++ {
    if klines[i].Volume > 0 {
        ratio := klines[i].TakerBuyBaseVolume / klines[i].Volume
        data.BuySellRatios = append(data.BuySellRatios, ratio)
    }
}
```

### 问题3：缺少成交量序列

**影响**：
- 无法检测放量（>1.5x均量）
- 无法判断"价格突破但成交量萎缩"

**解决方案**：
```go
// 在 IntradayData 中添加：
Volumes []float64

// 在 calculateIntradaySeries 中添加：
for i := start; i < len(klines); i++ {
    data.Volumes = append(data.Volumes, klines[i].Volume)
}
```

### 问题4：缺少OI变化百分比

**影响**：
- 无法判断"OI变化>+5%"（多空确认清单第8项）

**解决方案**：
```go
// 需要获取历史OI数据，计算变化百分比
// 或从OITopData中获取（如果有）
```

### 问题5：4小时缺少RSI7

**影响**：
- 提示词要求4小时序列有RSI7和RSI14，但只有RSI14

**解决方案**：
```go
// 在 LongerTermData 中添加：
RSI7Values []float64

// 在 calculateLongerTermData 中计算
```

---

## 📝 总结

### ✅ 已提供的数据
- 3分钟序列：Mid prices, EMA20, MACD, RSI7, RSI14（完整）
- 4小时序列：EMA20, EMA50, MACD序列, RSI14序列（部分）
- OI数据：Latest和Average
- 资金费率：✅

### ❌ 缺失的关键数据
1. **15分钟序列**（完全缺失）- 最严重
2. **1小时序列**（完全缺失）- 最严重
3. **买卖压力比（BuySellRatios）**（缺失）- 影响多空确认清单
4. **成交量序列（Volumes）**（缺失）- 影响放量检测
5. **OI变化百分比**（缺失）- 影响多空确认清单
6. **4小时RSI7序列**（缺失）- 提示词要求

### 🎯 优先级排序

**P0（必须修复）**：
1. 15分钟序列数据（多周期趋势确认的核心）
2. 1小时序列数据（多周期趋势确认的核心）
3. BTC的15m和1h MACD数据（BTC状态检查的关键）

**P1（重要）**：
4. 买卖压力比（BuySellRatios）- 影响多空确认清单
5. 成交量序列（Volumes）- 影响放量检测和防假突破

**P2（可选）**：
6. OI变化百分比 - 可以通过其他方式估算
7. 4小时RSI7序列 - 可以只用RSI14

---

## 💡 建议

1. **立即修复**：添加15m和1h数据获取，这是提示词的核心要求
2. **重要修复**：添加BuySellRatios和Volumes序列计算
3. **优化**：添加OI变化百分比计算
4. **完善**：添加4小时RSI7序列

需要我帮你实现这些缺失的数据获取和计算吗？

