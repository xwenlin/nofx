# 用户提示词结构优化分析

## 用户提出的新结构

```
时间: 2025-11-02 13:29:10 | 周期: #8 | 运行: 21分钟

**ALL OF THE PRICE OR SIGNAL DATA BELOW IS ORDERED: OLDEST → NEWEST**

# CURRENT MARKET STATE FOR ALL COINS

## 1. BTCUSDT
[完整市场数据]

## 2. ETHUSDT
[完整市场数据]

# HERE IS YOUR ACCOUNT INFORMATION & PERFORMANCE

账户: 净值378.61 | 余额290.45 (76.7%) | 盈亏-5.35% | 保证金22.8% | 持仓1个

## 当前持仓：

1. HYPEUSDT SHORT | 入场价39.5730 当前价39.3370 | 盈亏+0.60% | 杠杆5x | 保证金65 | 强平价47.0252 | 持仓时长8分钟

2. ETHUSDT SHORT | 入场价3592.8000 当前价3569.5278 | 盈亏+0.65% | 杠杆5x | 保证金124 | 强平价4294.2136 | 持仓时长3小时47分钟

## 历史表现分析：

**夏普比率**: 0.04  (这是你的核心绩效指标，用于调整交易策略)
**总交易数**: 22 (最近1000个周期内，用于判断交易频率是否合理)

现在请分析并输出决策（思维链 + JSON）
```

## 优点分析

### ✅ 1. 数据顺序说明明确
- **优点**：在开头明确说明所有数据都是 OLDEST → NEWEST，让AI清楚理解数据方向
- **改进**：当前提示词中已经有"数据顺序：旧→新，数组最后一个元素=最新数据点"（第215行），但这个说明更醒目

### ✅ 2. 结构清晰分离
- **优点**：将市场数据和账户信息明确分离，结构更清晰
- **逻辑**：先看市场，再看账户，符合交易决策流程

### ✅ 3. BTC放在第一位
- **优点**：BTC作为市场领导者，放在"ALL COINS"的第一位，符合提示词中"BTC优先"的原则
- **符合提示词要求**：提示词第241行明确要求"BTC优先"

## 潜在问题分析

### ❌ 1. 持仓币种的市场数据缺失（严重问题）

**问题**：
- 在"当前持仓"部分，只显示了持仓信息（入场价、当前价、盈亏等）
- **没有显示持仓币种的完整市场数据**（3m/15m/1h/4h序列、技术指标等）

**影响**：
- AI无法对持仓币种进行深入分析
- 无法判断是否需要平仓、调整止损止盈
- 无法分析持仓币种的技术面变化

**示例场景**：
```
持仓：ETHUSDT SHORT
- 如果ETHUSDT在"ALL COINS"部分，AI可以看到ETH的完整市场数据 ✅
- 但如果ETHUSDT不在"ALL COINS"部分（因为已经是持仓），AI就看不到市场数据 ❌
```

**解决方案**：
- **方案A**：在"当前持仓"部分，每个持仓币种下面也显示完整市场数据
- **方案B**：持仓币种如果不在"ALL COINS"部分，单独列出其市场数据（但这样可能重复）

### ❌ 2. 持仓币种和候选币种的数据重复问题

**问题**：
- 如果ETHUSDT既是持仓币种，又在"ALL COINS"部分，市场数据会被输出两次
- 用户已经意识到这个问题，所以才想重新排布结构

**当前代码逻辑**：
```go
// 当前实现：先输出持仓，再输出BTC，最后输出候选币种
// 使用 displayedSymbols 记录已输出的币种，避免重复
```

**用户新结构的处理**：
- 用户的结构中，持仓币种（如ETHUSDT）可能同时在"ALL COINS"部分
- 需要明确：**持仓币种是否应该出现在"ALL COINS"部分？**

**建议**：
- **方案A**：持仓币种**也**出现在"ALL COINS"部分，但在"当前持仓"中只显示持仓信息，不重复市场数据
- **方案B**：持仓币种**不**出现在"ALL COINS"部分，但在"当前持仓"中显示完整市场数据

### ⚠️ 3. BTC的特殊地位问题

**提示词要求**（第230-241行）：
- "BTC是市场领导者，交易任何币种前必须先确认BTC状态"
- "BTC多周期MACD方向（15m/1h/4h）需要检查"
- "BTC优先"（核心原则第6条）

**用户结构的处理**：
- BTC放在"ALL COINS"的第一位 ✅
- 但标题是"CURRENT MARKET STATE FOR ALL COINS"，可能弱化了BTC的特殊地位

**建议**：
- **方案A**：BTC单独一个section，标题为"## BTC市场状态（市场领导者，交易前必须确认）"
- **方案B**：保持现状，但BTC保持在第一位，并在注释中说明其特殊地位

### ⚠️ 4. 候选币种的标识缺失

**问题**：
- 在"ALL COINS"部分，无法区分哪些是候选币种（可能有AI500+OI_Top双重信号）
- 当前代码中有 `sourceTags` 标识（如"AI500+OI_Top双重信号"），但用户的结构中没有体现

**建议**：
- 在币种标题中添加来源标识，例如：
  ```
  ## 1. BTCUSDT
  ## 2. ETHUSDT (AI500+OI_Top双重信号)
  ## 3. SOLUSDT (OI_Top持仓增长)
  ```

### ⚠️ 5. 账户信息的显示时机

**当前结构**：
- 账户信息在"ALL COINS"之后显示

**建议**：
- 账户信息应该在"ALL COINS"**之前**显示，因为：
  1. AI需要先了解账户状态（可用余额、保证金使用率等），才能判断是否能开仓
  2. 账户信息是决策的前提条件

## 改进建议

### 推荐结构（结合用户需求和代码逻辑）

```
时间: 2025-11-02 13:29:10 | 周期: #8 | 运行: 21分钟

**ALL OF THE PRICE OR SIGNAL DATA BELOW IS ORDERED: OLDEST → NEWEST**

# HERE IS YOUR ACCOUNT INFORMATION & PERFORMANCE

账户: 净值378.61 | 余额290.45 (76.7%) | 盈亏-5.35% | 保证金22.8% | 持仓1个

## 当前持仓：

1. HYPEUSDT SHORT | 入场价39.5730 当前价39.3370 | 盈亏+0.60% | 杠杆5x | 保证金65 | 强平价47.0252 | 持仓时长8分钟
[完整市场数据 - 3m/15m/1h/4h序列、技术指标等]

2. ETHUSDT SHORT | 入场价3592.8000 当前价3569.5278 | 盈亏+0.65% | 杠杆5x | 保证金124 | 强平价4294.2136 | 持仓时长3小时47分钟
[完整市场数据 - 3m/15m/1h/4h序列、技术指标等]

## 历史表现分析：

**夏普比率**: 0.04  (这是你的核心绩效指标，用于调整交易策略)
**总交易数**: 22 (最近1000个周期内，用于判断交易频率是否合理)

---

# CURRENT MARKET STATE FOR ALL COINS

## BTC市场状态（市场领导者，交易前必须确认BTC状态）

[完整市场数据 - 3m/15m/1h/4h序列、技术指标等]

## 候选币种：

### 1. ETHUSDT (AI500+OI_Top双重信号)
[完整市场数据 - 仅在ETHUSDT不是持仓币种时显示]

### 2. SOLUSDT (OI_Top持仓增长)
[完整市场数据]

### 3. DOGEUSDT
[完整市场数据]

---

现在请分析并输出决策（思维链 + JSON）
```

### 关键改进点

1. **账户信息前置**：放在最前面，因为它是决策的前提条件
2. **持仓币种包含完整市场数据**：每个持仓币种下面都显示完整市场数据，便于AI分析是否需要平仓
3. **BTC单独强调**：BTC单独一个section，标题明确其特殊地位
4. **候选币种标识**：明确标注来源（AI500、OI_Top等）
5. **避免重复**：
   - 持仓币种的市场数据在"当前持仓"部分显示
   - 候选币种的市场数据在"候选币种"部分显示
   - BTC如果不是持仓币种，单独显示
   - 使用 `displayedSymbols` map 确保不重复

## 实现建议

### 修改后的代码逻辑

```go
func buildUserPrompt(ctx *Context) string {
    var sb strings.Builder
    
    // 1. 时间信息
    sb.WriteString(fmt.Sprintf("时间: %s | 周期: #%d | 运行: %d分钟\n\n",
        ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))
    
    // 2. 数据顺序说明
    sb.WriteString("**ALL OF THE PRICE OR SIGNAL DATA BELOW IS ORDERED: OLDEST → NEWEST**\n\n")
    
    // 3. 账户信息（前置）
    sb.WriteString("# HERE IS YOUR ACCOUNT INFORMATION & PERFORMANCE\n\n")
    sb.WriteString(fmt.Sprintf("账户: 净值%.2f | 余额%.2f (%.1f%%) | 盈亏%+.2f%% | 保证金%.1f%% | 持仓%d个\n\n",
        ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ...))
    
    // 记录已输出市场数据的币种（避免重复输出）
    displayedSymbols := make(map[string]bool)
    
    // 4. 当前持仓（包含完整市场数据）
    if len(ctx.Positions) > 0 {
        sb.WriteString("## 当前持仓：\n\n")
        for i, pos := range ctx.Positions {
            // 持仓信息
            sb.WriteString(fmt.Sprintf("%d. %s %s | ...\n\n", i+1, pos.Symbol, ...))
            
            // 完整市场数据
            if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
                sb.WriteString(market.Format(marketData))
                sb.WriteString("\n")
                displayedSymbols[pos.Symbol] = true
            }
        }
    }
    
    // 5. 历史表现分析
    if ctx.Performance != nil {
        // ...
    }
    
    sb.WriteString("---\n\n")
    
    // 6. BTC市场状态（单独强调，仅当不是持仓币种时）
    if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC && !displayedSymbols["BTCUSDT"] {
        sb.WriteString("# CURRENT MARKET STATE FOR ALL COINS\n\n")
        sb.WriteString("## BTC市场状态（市场领导者，交易前必须确认BTC状态）\n\n")
        sb.WriteString(market.Format(btcData))
        sb.WriteString("\n")
        displayedSymbols["BTCUSDT"] = true
    }
    
    // 7. 候选币种（排除已输出的持仓币种）
    displayedCount := 0
    for _, coin := range ctx.CandidateCoins {
        if displayedSymbols[coin.Symbol] {
            continue
        }
        // ...
    }
    
    return sb.String()
}
```

## 总结

### 用户结构的优点
1. ✅ 数据顺序说明明确
2. ✅ 结构清晰分离
3. ✅ BTC放在第一位

### 需要改进的地方
1. ❌ **持仓币种必须包含完整市场数据**（最关键）
2. ⚠️ 账户信息应该前置
3. ⚠️ BTC应该单独强调其特殊地位
4. ⚠️ 候选币种需要标识来源

### 最终建议
- 采用**推荐结构**，既保持了用户提出的清晰分离，又确保了所有必要数据的完整性
- 关键原则：**持仓币种必须有完整市场数据**，否则AI无法做出合理的平仓决策

