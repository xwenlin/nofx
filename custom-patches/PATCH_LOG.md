# 修复日志

本文档记录了对代码的修复和补丁，用于在合并新版本时参考。

---

## 2025-11-03 - 修复交易员提示词模板更新不生效的问题

### 问题描述
修改了交易员的系统提示词模板名称（`system_prompt_template`），但修改后没有生效，重启后仍然使用旧的模板。

### 根本原因
1. **API请求结构体缺少字段**：`UpdateTraderRequest` 中没有 `SystemPromptTemplate` 字段，导致前端传递的模板名称无法被后端接收
2. **数据库更新不完整**：即使添加了字段，在 `handleUpdateTrader` 中构建 `trader` 对象时也没有设置 `SystemPromptTemplate` 字段
3. **内存实例未更新**：在 `LoadUserTraders` 中，如果 trader 已经存在于内存中，会直接跳过，不会更新其配置，导致内存中的 trader 实例仍使用旧的模板名称
4. **前端保存时缺少字段**：在 `AITradersPage.tsx` 的 `handleSaveEditTrader` 函数中，构建 `request` 对象时缺少 `system_prompt_template` 字段，导致编辑交易员时选择的模板没有被提交到后端
5. **获取配置时缺少字段**：在 `handleGetTraderConfig` 函数中，返回的配置对象中缺少 `system_prompt_template` 字段，导致重新打开编辑界面时无法显示已保存的模板名称
6. **数据库读取缺少字段**：在 `GetTraderConfig` 方法中，SQL查询语句中没有包含 `system_prompt_template` 字段，导致从数据库读取时该字段为空

### 修改文件
- `api/server.go`
- `manager/trader_manager.go`
- `web/src/components/AITradersPage.tsx`
- `config/database.go`

### 具体修改

#### 1. 修改 `api/server.go` - 添加 SystemPromptTemplate 字段支持

**修改 `UpdateTraderRequest` 结构体（第380-393行）：**

**修改前：**
```go
type UpdateTraderRequest struct {
	Name            string  `json:"name" binding:"required"`
	AIModelID       string  `json:"ai_model_id" binding:"required"`
	ExchangeID      string  `json:"exchange_id" binding:"required"`
	InitialBalance  float64 `json:"initial_balance"`
	BTCETHLeverage  int     `json:"btc_eth_leverage"`
	AltcoinLeverage int     `json:"altcoin_leverage"`
	TradingSymbols  string  `json:"trading_symbols"`
	CustomPrompt    string  `json:"custom_prompt"`
	OverrideBasePrompt bool `json:"override_base_prompt"`
	IsCrossMargin   *bool   `json:"is_cross_margin"`
}
```

**修改后：**
```go
type UpdateTraderRequest struct {
	Name            string  `json:"name" binding:"required"`
	AIModelID       string  `json:"ai_model_id" binding:"required"`
	ExchangeID      string  `json:"exchange_id" binding:"required"`
	InitialBalance  float64 `json:"initial_balance"`
	BTCETHLeverage  int     `json:"btc_eth_leverage"`
	AltcoinLeverage int     `json:"altcoin_leverage"`
	TradingSymbols  string  `json:"trading_symbols"`
	CustomPrompt    string  `json:"custom_prompt"`
	OverrideBasePrompt bool `json:"override_base_prompt"`
	SystemPromptTemplate string `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin   *bool   `json:"is_cross_margin"`
}
```

**修改 `handleUpdateTrader` 方法（第426-465行）：**

**修改前：**
```go
// 设置默认值
isCrossMargin := existingTrader.IsCrossMargin // 保持原值
if req.IsCrossMargin != nil {
	isCrossMargin = *req.IsCrossMargin
}

// 设置杠杆默认值
btcEthLeverage := req.BTCETHLeverage
altcoinLeverage := req.AltcoinLeverage
if btcEthLeverage <= 0 {
	btcEthLeverage = existingTrader.BTCETHLeverage // 保持原值
}
if altcoinLeverage <= 0 {
	altcoinLeverage = existingTrader.AltcoinLeverage // 保持原值
}

// 更新交易员配置
trader := &config.TraderRecord{
	ID:                  traderID,
	UserID:              userID,
	Name:                req.Name,
	AIModelID:           req.AIModelID,
	ExchangeID:          req.ExchangeID,
	InitialBalance:      req.InitialBalance,
	BTCETHLeverage:      btcEthLeverage,
	AltcoinLeverage:     altcoinLeverage,
	TradingSymbols:      req.TradingSymbols,
	CustomPrompt:        req.CustomPrompt,
	OverrideBasePrompt:  req.OverrideBasePrompt,
	IsCrossMargin:       isCrossMargin,
	ScanIntervalMinutes: existingTrader.ScanIntervalMinutes, // 保持原值
	IsRunning:           existingTrader.IsRunning,           // 保持原值
}
```

**修改后：**
```go
// 设置默认值
isCrossMargin := existingTrader.IsCrossMargin // 保持原值
if req.IsCrossMargin != nil {
	isCrossMargin = *req.IsCrossMargin
}

// 设置杠杆默认值
btcEthLeverage := req.BTCETHLeverage
altcoinLeverage := req.AltcoinLeverage
if btcEthLeverage <= 0 {
	btcEthLeverage = existingTrader.BTCETHLeverage // 保持原值
}
if altcoinLeverage <= 0 {
	altcoinLeverage = existingTrader.AltcoinLeverage // 保持原值
}

// 设置系统提示词模板（如果没有提供，保持原值）
systemPromptTemplate := existingTrader.SystemPromptTemplate // 保持原值
if req.SystemPromptTemplate != "" {
	systemPromptTemplate = req.SystemPromptTemplate
}

// 更新交易员配置
trader := &config.TraderRecord{
	ID:                  traderID,
	UserID:              userID,
	Name:                req.Name,
	AIModelID:           req.AIModelID,
	ExchangeID:          req.ExchangeID,
	InitialBalance:      req.InitialBalance,
	BTCETHLeverage:      btcEthLeverage,
	AltcoinLeverage:     altcoinLeverage,
	TradingSymbols:      req.TradingSymbols,
	CustomPrompt:        req.CustomPrompt,
	OverrideBasePrompt:  req.OverrideBasePrompt,
	SystemPromptTemplate: systemPromptTemplate,
	IsCrossMargin:       isCrossMargin,
	ScanIntervalMinutes: existingTrader.ScanIntervalMinutes, // 保持原值
	IsRunning:           existingTrader.IsRunning,           // 保持原值
}
```

#### 2. 修改 `manager/trader_manager.go` - 更新已存在的 trader 配置

**修改 `LoadUserTraders` 方法（第618-626行）：**

**修改前：**
```go
// 为每个交易员获取AI模型和交易所配置
for _, traderCfg := range traders {
	// 检查是否已经加载过这个交易员
	if _, exists := tm.traders[traderCfg.ID]; exists {
		log.Printf("⚠️ 交易员 %s 已经加载，跳过", traderCfg.Name)
		continue
	}
```

**修改后：**
```go
// 为每个交易员获取AI模型和交易所配置
for _, traderCfg := range traders {
	// 检查是否已经加载过这个交易员
	if existingTrader, exists := tm.traders[traderCfg.ID]; exists {
		// 如果已经存在，更新配置（特别是 SystemPromptTemplate）
		existingTrader.SetSystemPromptTemplate(traderCfg.SystemPromptTemplate)
		log.Printf("✓ 已更新交易员 %s 的配置（模板: %s）", traderCfg.Name, traderCfg.SystemPromptTemplate)
		continue
	}
```

#### 3. 修改 `web/src/components/AITradersPage.tsx` - 修复前端保存时缺少字段

**修改 `handleSaveEditTrader` 函数（第206-220行）：**

**修改前：**
```typescript
const request = {
  name: data.name,
  ai_model_id: finalAIModelId,
  exchange_id: data.exchange_id,
  initial_balance: data.initial_balance,
  btc_eth_leverage: data.btc_eth_leverage,
  altcoin_leverage: data.altcoin_leverage,
  trading_symbols: data.trading_symbols,
  custom_prompt: data.custom_prompt,
  override_base_prompt: data.override_base_prompt,
  is_cross_margin: data.is_cross_margin,
  use_coin_pool: data.use_coin_pool,
  use_oi_top: data.use_oi_top
};
```

**修改后：**
```typescript
const request = {
  name: data.name,
  ai_model_id: finalAIModelId,
  exchange_id: data.exchange_id,
  initial_balance: data.initial_balance,
  btc_eth_leverage: data.btc_eth_leverage,
  altcoin_leverage: data.altcoin_leverage,
  trading_symbols: data.trading_symbols,
  custom_prompt: data.custom_prompt,
  override_base_prompt: data.override_base_prompt,
  system_prompt_template: data.system_prompt_template, // 系统提示词模板名称
  is_cross_margin: data.is_cross_margin,
  use_coin_pool: data.use_coin_pool,
  use_oi_top: data.use_oi_top
};
```

### 修改说明
1. **添加字段支持**：
   - 在 `UpdateTraderRequest` 中添加 `SystemPromptTemplate` 字段，允许前端传递模板名称
   - 在 `handleUpdateTrader` 中处理该字段，如果没有提供则保持原值，如果提供了则更新

2. **数据库更新**：
   - 确保 `SystemPromptTemplate` 字段被正确保存到数据库
   - 通过 `UpdateTrader` 方法更新数据库记录

3. **内存实例更新**：
   - 修改 `LoadUserTraders` 方法，如果 trader 已存在于内存中，更新其 `SystemPromptTemplate` 配置
   - 这样即使 trader 正在运行，也能立即使用新的模板名称

4. **前端保存修复**：
   - 在 `handleSaveEditTrader` 函数中添加 `system_prompt_template` 字段到请求对象
   - 确保前端选择的下拉框值能正确提交到后端

#### 4. 修改 `api/server.go` - 修复返回配置时缺少字段

**修改 `handleGetTraderConfig` 方法（第821-837行）：**

**修改前：**
```go
result := map[string]interface{}{
	"trader_id":            traderConfig.ID,
	"trader_name":          traderConfig.Name,
	"ai_model":             aiModelID,
	"exchange_id":          traderConfig.ExchangeID,
	"initial_balance":      traderConfig.InitialBalance,
	"btc_eth_leverage":     traderConfig.BTCETHLeverage,
	"altcoin_leverage":     traderConfig.AltcoinLeverage,
	"trading_symbols":      traderConfig.TradingSymbols,
	"custom_prompt":        traderConfig.CustomPrompt,
	"override_base_prompt": traderConfig.OverrideBasePrompt,
	"is_cross_margin":      traderConfig.IsCrossMargin,
	"use_coin_pool":        traderConfig.UseCoinPool,
	"use_oi_top":           traderConfig.UseOITop,
	"is_running":           isRunning,
}
```

**修改后：**
```go
result := map[string]interface{}{
	"trader_id":             traderConfig.ID,
	"trader_name":           traderConfig.Name,
	"ai_model":              aiModelID,
	"exchange_id":           traderConfig.ExchangeID,
	"initial_balance":       traderConfig.InitialBalance,
	"btc_eth_leverage":      traderConfig.BTCETHLeverage,
	"altcoin_leverage":      traderConfig.AltcoinLeverage,
	"trading_symbols":       traderConfig.TradingSymbols,
	"custom_prompt":         traderConfig.CustomPrompt,
	"override_base_prompt": traderConfig.OverrideBasePrompt,
	"system_prompt_template": traderConfig.SystemPromptTemplate, // 系统提示词模板名称
	"is_cross_margin":       traderConfig.IsCrossMargin,
	"use_coin_pool":         traderConfig.UseCoinPool,
	"use_oi_top":            traderConfig.UseOITop,
	"is_running":            isRunning,
}
```

5. **获取配置修复（API层）**：
   - 在 `handleGetTraderConfig` 函数中添加 `system_prompt_template` 字段到返回对象
   - 确保重新打开编辑界面时能正确显示已保存的模板名称

#### 5. 修改 `config/database.go` - 修复数据库读取时缺少字段

**修改 `GetTraderConfig` 方法（第855-871行）：**

**修改前：**
```go
// 先获取交易员基本信息
err := d.db.QueryRow(`
	SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
	       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
	       COALESCE(trading_symbols, '') as trading_symbols,
	       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
	       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
	       COALESCE(is_cross_margin, 1) as is_cross_margin, created_at, updated_at
	FROM traders
	WHERE id = ? AND user_id = ?
`, traderID, userID).Scan(
	&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
	&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
	&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
	&trader.UseCoinPool, &trader.UseOITop, &trader.CustomPrompt, &trader.OverrideBasePrompt,
	&trader.IsCrossMargin, &trader.CreatedAt, &trader.UpdatedAt,
)
```

**修改后：**
```go
// 先获取交易员基本信息
err := d.db.QueryRow(`
	SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
	       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
	       COALESCE(trading_symbols, '') as trading_symbols,
	       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
	       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
	       COALESCE(system_prompt_template, 'default') as system_prompt_template,
	       COALESCE(is_cross_margin, 1) as is_cross_margin, created_at, updated_at
	FROM traders
	WHERE id = ? AND user_id = ?
`, traderID, userID).Scan(
	&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
	&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
	&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
	&trader.UseCoinPool, &trader.UseOITop, &trader.CustomPrompt, &trader.OverrideBasePrompt,
	&trader.SystemPromptTemplate, &trader.IsCrossMargin, &trader.CreatedAt, &trader.UpdatedAt,
)
```

6. **获取配置修复（数据库层）**：
   - 在 `GetTraderConfig` 方法的 SQL 查询中添加 `system_prompt_template` 字段
   - 在 `Scan` 语句中添加 `&trader.SystemPromptTemplate` 参数
   - 确保从数据库读取时能正确获取该字段的值

### 验证要点
- ✅ 更新交易员配置时，`system_prompt_template` 字段可以被正确接收和保存
- ✅ 数据库中的 `system_prompt_template` 字段会被正确更新
- ✅ 从数据库读取时，`system_prompt_template` 字段能被正确读取
- ✅ 内存中的 trader 实例会立即更新，无需重启
- ✅ 重新打开编辑界面时，能正确显示已保存的模板名称
- ✅ 下次 AI 决策时会使用新的模板名称

### 测试建议
1. 修改交易员的 `system_prompt_template` 字段（例如从 "default" 改为 "nof1"）
2. 确认前端下拉框选择了新的模板
3. 点击保存，检查浏览器网络请求，确认请求体中包含 `system_prompt_template` 字段
4. 确认数据库中的字段已更新
5. 检查日志，确认内存中的 trader 实例已更新
6. 等待下一次 AI 决策周期，确认使用了新的模板

---

## 2025-11-02 - 精简历史表现数据传递（优化AI决策）

### 问题描述
在之前的改进中，我们在 `buildUserPrompt` 中增加了大量历史表现数据（胜率、盈亏比、各币种表现、最近交易详情等）传递给AI。但经过分析发现：
1. **信息过载**：传递了过多数据，占用大量 token，可能分散AI注意力
2. **逻辑不一致**：系统提示词明确说"夏普比率是唯一指标"，但传递了很多其他指标
3. **选币逻辑冲突**：系统要求AI根据当前市场数据选币，但传递了历史币种表现统计，可能误导AI
4. **冗余数据**：胜率、盈亏比等指标已经隐含在夏普比率中

### 根本原因
系统提示词（`prompts/default.txt`）明确要求：
- **核心目标**：最大化夏普比率
- **关键认知**：夏普比率是唯一指标
- **币种选择**：应根据当前市场数据（价格、指标、OI等）选择，而非历史表现

但之前的实现传递了：
- 胜率、盈亏比（已在夏普比率中体现）
- 最佳/最差币种（与选币逻辑不一致）
- 各币种详细统计（占用大量token，且逻辑不一致）
- 最近5笔交易详情（信息量过大，可能分散注意力）

### 修改文件
- `decision/engine.go`

### 具体修改

#### 修改 `buildUserPrompt` 方法中的历史表现分析部分（第369-397行）

**修改前：**
```go
// 历史表现分析（详细数据）
if ctx.Performance != nil {
    sb.WriteString("## 📊 历史表现分析\n\n")
    
    // 传递了大量详细数据：
    // - 总交易数、胜率、盈亏比、夏普比率
    // - 平均盈利/亏损
    // - 最佳/最差币种
    // - 各币种详细统计（前10个）
    // - 最近5笔交易详情
    // ...（约140行代码）
}
```

**修改后：**
```go
// 历史表现分析（精简版本：只传递核心指标，符合系统提示词"夏普比率是唯一指标"的要求）
if ctx.Performance != nil {
    sb.WriteString("## 📊 历史表现分析\n\n")
    
    // 定义精简的Performance数据结构
    type PerformanceData struct {
        TotalTrades int     `json:"total_trades"`
        SharpeRatio float64 `json:"sharpe_ratio"`
    }
    
    var perfData PerformanceData
    if jsonData, err := json.Marshal(ctx.Performance); err == nil {
        if err := json.Unmarshal(jsonData, &perfData); err == nil {
            if perfData.TotalTrades > 0 {
                // 核心指标：夏普比率（系统提示词明确要求的唯一指标）
                sb.WriteString(fmt.Sprintf("**夏普比率**: %.2f (这是你的核心绩效指标，用于调整交易策略)\n\n",
                    perfData.SharpeRatio))
                
                // 交易频率提醒（帮助AI判断是否过度交易）
                // 假设分析窗口是1000个周期（约50小时），帮助AI判断交易频率是否合理
                sb.WriteString(fmt.Sprintf("**总交易数**: %d (最近1000个周期内，用于判断交易频率是否合理)\n\n",
                    perfData.TotalTrades))
            } else {
                // 如果没有交易记录，只显示提示
                sb.WriteString("**当前无历史交易记录**\n\n")
            }
        }
    }
}
```

### 修改说明
1. **精简数据传递**：
   - 只传递核心指标：夏普比率和总交易数
   - 移除冗余数据：胜率、盈亏比、平均盈利/亏损（已在夏普比率中体现）
   - 移除不必要数据：最佳/最差币种、各币种统计、最近交易详情

2. **符合系统设计**：
   - 严格按照系统提示词"夏普比率是唯一指标"的要求
   - 保持选币逻辑一致性（基于当前市场数据，而非历史表现）
   - 减少信息过载，让AI聚焦核心指标

3. **优化性能**：
   - 大幅减少 prompt 长度（从约140行减少到约30行）
   - 降低 token 消耗和 API 成本
   - 提高AI决策效率

### 验证要点
- ✅ 只传递夏普比率和总交易数
- ✅ 符合系统提示词要求
- ✅ 保持选币逻辑一致性
- ✅ 减少 token 消耗
- ✅ 不影响历史表现数据的计算（数据仍然完整，只是不全部传递给AI）

### 测试建议
1. 验证AI仍然能正确使用夏普比率调整交易策略
2. 验证AI仍然能判断是否过度交易（通过总交易数）
3. 确认 prompt 长度明显减少
4. 确认AI决策质量不受影响（甚至可能更好，因为聚焦核心指标）

---

## 2025-11-02 - 修复交易决策中历史表现分析窗口不一致的问题

### 问题描述
在修复"AI学习与反思只显示少量交易"问题时，已经将 `api/server.go` 中的分析窗口从100个周期调整为1000个周期，但 `trader/auto_trader.go` 中用于交易决策的历史表现分析仍然使用100个周期，导致不一致。这会影响AI在做交易决策时看到的历史数据范围。

### 根本原因
在之前的修复中：
- `api/server.go` 的 `handlePerformance` 已经改为分析最近1000个周期（用于界面展示）
- `trader/auto_trader.go` 的 `buildTradingContext` 仍在使用100个周期（用于AI决策）

这导致两个问题：
1. **数据不一致**：界面展示的历史数据和AI决策使用的历史数据范围不同
2. **决策不准确**：AI只能看到最近5小时的表现，而界面能看到50小时的数据，导致AI可能基于不完整的历史数据做决策

### 修改文件
- `trader/auto_trader.go`

### 具体修改

#### 修改 `buildTradingContext` 方法中的分析窗口（第552-555行）

**修改前：**
```go
// 5. 分析历史表现（最近100个周期，避免长期持仓的交易记录丢失）
// 假设每3分钟一个周期，100个周期 = 5小时，足够覆盖大部分交易
performance, err := at.decisionLogger.AnalyzePerformance(100)
```

**修改后：**
```go
// 5. 分析历史表现（最近1000个周期，避免长期持仓的交易记录丢失）
// 假设每3分钟一个周期，1000个周期 = 50小时，足够覆盖大部分交易
// 即使开仓记录在窗口外，也会从更早的历史记录中查找匹配
performance, err := at.decisionLogger.AnalyzePerformance(1000)
```

### 修改说明
1. **统一分析窗口**：
   - 将 `auto_trader.go` 中的分析窗口从100个周期增加到1000个周期
   - 与 `api/server.go` 保持一致，都使用1000个周期

2. **数据一致性**：
   - AI交易决策和界面展示使用相同的历史数据范围
   - 确保AI能看到完整的历史表现数据

3. **利用已有优化**：
   - 配合之前修复的匹配逻辑，能从更早的历史记录中查找开仓记录
   - 确保能匹配到更多完整的交易记录

### 影响范围
- ✅ 统一了交易决策和界面展示的数据分析窗口
- ✅ AI决策时能看到更多历史表现数据（从5小时增加到50小时）
- ✅ 夏普比率计算更准确（基于更多历史数据）
- ✅ 交易匹配更完整（能匹配到更多完整交易）
- ✅ 与之前修复的匹配逻辑优化配合使用

### 测试建议
1. 运行系统一段时间，完成多笔交易
2. 检查AI决策时的历史表现分析是否包含更多数据
3. 验证界面展示的"AI学习与反思"数据和AI决策使用的数据范围一致
4. 确认能正确匹配更多完整的历史交易记录

---

## 2025-11-02 - 修复Binance API时间戳错误（-1021）问题

### 问题描述
偶尔会出现Binance API调用失败，错误信息显示：
```
获取账户信息失败: <APIError> code=-1021, msg=Timestamp for this request is outside of the recvWindow.
```
导致交易决策记录失败，账户状态和持仓信息无法获取。

### 根本原因
Binance API 错误代码 `-1021` 表示请求的时间戳在接收窗口（recvWindow）之外。这通常由以下原因导致：
1. **系统时间不同步**：本地系统时间与Binance服务器时间不同步
2. **网络延迟**：请求在传输过程中延迟，导致到达服务器时时间戳已过期
3. **时间窗口限制**：Binance API要求请求时间戳必须在服务器时间的一定窗口内（默认通常是5秒）

### 修改文件
- `trader/binance_futures.go`

### 具体修改

#### 1. 添加 strings 包导入（第8行）

**修改前：**
```go
import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)
```

**修改后：**
```go
import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)
```

#### 2. 为 GetBalance 方法添加重试机制（第41-94行）

**修改前：**
```go
// GetBalance 获取账户余额（带缓存）
func (t *FuturesTrader) GetBalance() (map[string]interface{}, error) {
	// ... 缓存检查 ...
	
	// 缓存过期或不存在，调用API
	log.Printf("🔄 缓存过期，正在调用币安API获取账户余额...")
	account, err := t.client.NewGetAccountService().Do(context.Background())
	if err != nil {
		log.Printf("❌ 币安API调用失败: %v", err)
		return nil, fmt.Errorf("获取账户信息失败: %w", err)
	}
	// ...
}
```

**修改后：**
```go
// GetBalance 获取账户余额（带缓存和重试机制）
func (t *FuturesTrader) GetBalance() (map[string]interface{}, error) {
	// ... 缓存检查 ...
	
	// 缓存过期或不存在，调用API（带重试机制）
	log.Printf("🔄 缓存过期，正在调用币安API获取账户余额...")
	
	// 重试机制：专门处理时间戳错误
	maxRetries := 3
	var lastErr error
	var account *futures.Account
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			// 时间戳错误时，等待一小段时间后重试
			waitTime := time.Duration(attempt-1) * time.Second
			log.Printf("⚠️  币安API调用失败，等待%v后重试 (%d/%d)...", waitTime, attempt, maxRetries)
			time.Sleep(waitTime)
		}
		
		acc, err := t.client.NewGetAccountService().Do(context.Background())
		if err == nil {
			account = acc
			break
		}
		
		lastErr = err
		errStr := err.Error()
		
		// 检查是否是时间戳错误（-1021）
		if strings.Contains(errStr, "-1021") || strings.Contains(errStr, "outside of the recvWindow") || strings.Contains(errStr, "Timestamp") {
			log.Printf("⚠️  检测到时间戳错误，将在重试时生成新的时间戳")
			if attempt < maxRetries {
				continue // 重试
			}
		}
		
		// 其他错误不重试，直接返回
		log.Printf("❌ 币安API调用失败: %v", err)
		return nil, fmt.Errorf("获取账户信息失败: %w", err)
	}
	
	// 如果所有重试都失败
	if account == nil {
		return nil, fmt.Errorf("获取账户信息失败（已重试%d次）: %w", maxRetries, lastErr)
	}
	// ...
}
```

#### 3. 为 GetPositions 方法添加重试机制（第113-163行）

**修改前：**
```go
// GetPositions 获取所有持仓（带缓存）
func (t *FuturesTrader) GetPositions() ([]map[string]interface{}, error) {
	// ... 缓存检查 ...
	
	// 缓存过期或不存在，调用API
	log.Printf("🔄 缓存过期，正在调用币安API获取持仓信息...")
	positions, err := t.client.NewGetPositionRiskService().Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}
	// ...
}
```

**修改后：**
```go
// GetPositions 获取所有持仓（带缓存和重试机制）
func (t *FuturesTrader) GetPositions() ([]map[string]interface{}, error) {
	// ... 缓存检查 ...
	
	// 缓存过期或不存在，调用API（带重试机制）
	log.Printf("🔄 缓存过期，正在调用币安API获取持仓信息...")
	
	// 重试机制：专门处理时间戳错误
	maxRetries := 3
	var lastErr error
	var positions []*futures.PositionRisk
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			// 时间戳错误时，等待一小段时间后重试
			waitTime := time.Duration(attempt-1) * time.Second
			log.Printf("⚠️  币安API调用失败，等待%v后重试 (%d/%d)...", waitTime, attempt, maxRetries)
			time.Sleep(waitTime)
		}
		
		pos, err := t.client.NewGetPositionRiskService().Do(context.Background())
		if err == nil {
			positions = pos
			break
		}
		
		lastErr = err
		errStr := err.Error()
		
		// 检查是否是时间戳错误（-1021）
		if strings.Contains(errStr, "-1021") || strings.Contains(errStr, "outside of the recvWindow") || strings.Contains(errStr, "Timestamp") {
			log.Printf("⚠️  检测到时间戳错误，将在重试时生成新的时间戳")
			if attempt < maxRetries {
				continue // 重试
			}
		}
		
		// 其他错误不重试，直接返回
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}
	
	if lastErr != nil && len(positions) == 0 {
		return nil, fmt.Errorf("获取持仓失败（已重试%d次）: %w", maxRetries, lastErr)
	}
	// ...
}
```

### 修改说明
1. **重试机制**：
   - 检测到时间戳错误（-1021）时自动重试，最多重试3次
   - 每次重试前等待1-2秒，给系统时间同步机会
   - 每次重试时会生成新的时间戳（由库自动处理）

2. **错误识别**：
   - 通过检查错误消息中包含 "-1021"、"outside of the recvWindow" 或 "Timestamp" 来识别时间戳错误
   - 仅对时间戳错误进行重试，其他错误立即返回

3. **智能处理**：
   - 非时间戳错误不重试，避免不必要的延迟
   - 所有重试失败后返回详细的错误信息

4. **同时修复两个方法**：
   - `GetBalance()`：获取账户余额
   - `GetPositions()`：获取持仓信息

### 影响范围
- ✅ 修复了Binance API时间戳错误（-1021）导致的API调用失败
- ✅ 提高了API调用的成功率，特别是在网络延迟或时间同步不准确的情况下
- ✅ 增强了系统的容错能力，自动处理临时的时间戳问题
- ✅ 不影响其他功能的正常运行
- ✅ 对于非时间戳错误，保持原有的立即返回行为

### 测试建议
1. 在时间不同步的系统上测试，验证重试机制是否正常工作
2. 在网络延迟较高的环境下测试，验证是否能正确处理时间戳过期
3. 检查日志，确认重试机制在遇到时间戳错误时正确触发
4. 验证其他类型的API错误不会被误判为时间戳错误而重试
5. 长期运行测试，观察时间戳错误的频率是否降低

---

## 2025-11-02 - 修复AI学习与反思只显示少量交易的问题

### 问题描述
实际运行了一段时间，完整的开仓到平仓的交易已经很多了，但在"AI学习与反思"界面中只看到3笔交易，无法看到完整的历史交易记录。

### 根本原因
问题有两个层面：

1. **分析窗口太小**：
   - `api/server.go` 的 `handlePerformance` 函数只分析最近100个周期（约5小时）
   - 如果开仓发生在窗口外，即使平仓在窗口内，也无法匹配到完整的交易记录

2. **开仓记录查找范围受限**：
   - `logger/decision_logger.go` 的 `AnalyzePerformance` 函数虽然尝试扩大窗口，但只扩大到3倍（300个周期，约15小时）
   - 如果交易持仓时间超过15小时，就无法找到对应的开仓记录，导致交易无法匹配

### 修改文件
- `api/server.go`
- `logger/decision_logger.go`

### 具体修改

#### 1. 增加分析窗口大小（`api/server.go` 第1112-1115行）

**修改前：**
```go
// 分析最近100个周期的交易表现（避免长期持仓的交易记录丢失）
// 假设每3分钟一个周期，100个周期 = 5小时，足够覆盖大部分交易
performance, err := trader.GetDecisionLogger().AnalyzePerformance(100)
```

**修改后：**
```go
// 分析最近1000个周期的交易表现（避免长期持仓的交易记录丢失）
// 假设每3分钟一个周期，1000个周期 = 50小时，足够覆盖大部分交易
// 即使开仓记录在窗口外，也会从更早的历史记录中查找匹配
performance, err := trader.GetDecisionLogger().AnalyzePerformance(1000)
```

#### 2. 改进开仓记录查找逻辑（`logger/decision_logger.go` 第338-391行）

**修改前：**
```go
// 为了避免开仓记录在窗口外导致匹配失败，需要先从所有历史记录中找出未平仓的持仓
// 获取更多历史记录来构建完整的持仓状态（使用更大的窗口）
allRecords, err := l.GetLatestRecords(lookbackCycles * 3) // 扩大3倍窗口
if err == nil && len(allRecords) > len(records) {
    // 先从扩大的窗口中收集所有开仓记录
    for _, record := range allRecords {
        // ... 处理开仓和平仓记录
    }
}
```

**修改后：**
```go
// 为了避免开仓记录在窗口外导致匹配失败，需要从所有历史记录中查找开仓记录
// 使用足够大的窗口（10000个周期，约500小时）来查找开仓记录，确保能匹配到所有可能的开仓
// 这样即使交易持仓时间很长，也能正确匹配开仓和平仓
allRecords, err := l.GetLatestRecords(10000) // 从所有历史记录中查找（最多10000个周期）

// 确定分析窗口的起始位置（在allRecords中的索引）
// records是分析窗口内的记录（最近的lookbackCycles个周期）
// allRecords包含所有历史记录（最多10000个周期），按时间从旧到新排序
windowStartIdx := 0
if len(allRecords) > len(records) {
    windowStartIdx = len(allRecords) - len(records)
}

if err == nil && len(allRecords) > 0 {
    // 从所有历史记录中收集开仓记录（按时间顺序，从旧到新）
    // 关键：只删除分析窗口外的平仓记录，保留窗口内的平仓对应的开仓记录
    for i, record := range allRecords {
        for _, action := range record.Decisions {
            // ... 处理开仓记录
            
            switch action.Action {
            case "open_long", "open_short":
                // 记录开仓（后续的开仓会覆盖之前的，确保使用最新的开仓记录）
                openPositions[posKey] = map[string]interface{}{
                    "side":      side,
                    "openPrice": action.Price,
                    "openTime":  action.Timestamp,
                    "quantity":  action.Quantity,
                    "leverage":  action.Leverage,
                }
            case "close_long", "close_short":
                // 只删除分析窗口外的平仓记录对应的开仓
                // 如果平仓在分析窗口外，说明这个交易已经在窗口前完成，不需要保留开仓记录
                // 如果平仓在分析窗口内，需要保留开仓记录，以便在窗口内匹配
                if i < windowStartIdx {
                    // 这个平仓在分析窗口外，可以安全删除对应的开仓记录
                    delete(openPositions, posKey)
                }
                // 如果平仓在分析窗口内，不删除，保留开仓记录供后续匹配使用
            }
        }
    }
}
```

### 修改说明
1. **增加分析窗口**：
   - 将分析窗口从100个周期增加到1000个周期（从5小时增加到50小时）
   - 这样可以覆盖更多最近完成的交易

2. **扩大开仓记录查找范围**：
   - 从所有历史记录（最多10000个周期，约500小时）中查找开仓记录
   - 之前只扩大3倍窗口（300个周期），现在可以查找最多500小时前的开仓记录
   - 确保即使开仓发生很久之前，只要平仓在分析窗口内，都能正确匹配

3. **优化匹配逻辑**：
   - 在构建开仓记录映射时，只删除分析窗口外的平仓记录对应的开仓
   - 保留分析窗口内的平仓对应的开仓记录，确保在分析窗口内能正确匹配
   - 这样避免了过早删除还在分析窗口内的平仓对应的开仓记录

4. **保持性能**：
   - 虽然查找范围扩大了，但只分析窗口内的平仓记录来生成交易结果
   - 这样既能找到所有开仓记录，又不会因为分析所有历史记录而影响性能

### 影响范围
- ✅ 修复了"AI学习与反思"中只显示少量交易的问题
- ✅ 现在可以显示最近50小时内完成的完整交易记录
- ✅ 支持匹配持仓时间超过50小时的交易（开仓在窗口外，平仓在窗口内）
- ✅ 提高了交易匹配的准确性和完整性
- ✅ 不影响其他功能的正常运行

### 测试建议
1. 运行交易系统一段时间（超过5小时）
2. 完成多笔完整的开仓到平仓的交易
3. 检查"AI学习与反思"界面，应该能看到最近50小时内完成的所有交易
4. 验证交易记录的完整性（开仓价、平仓价、盈亏等信息）
5. 检查是否有长时间持仓（超过50小时）的交易也能正确显示

---

## 2025-11-02 - 修复编辑交易员时提示"AI模型配置不存在"的问题

### 问题描述
在编辑交易员时，系统提示错误：`{"error":"获取交易员配置失败: AI模型配置不存在 (provider: admin_deepseek, user_id: admin): sql: no rows in result set"}`，导致无法编辑交易员。

### 根本原因
在 `GetTraderConfig` 函数中，代码假设 `traders` 表中的 `ai_model_id` 字段存储的是 provider（如 `"deepseek"`），直接使用 `provider = ?` 来查找 AI 模型。但实际数据库中 `ai_model_id` 可能存储的是用户特定的 ID（如 `"admin_deepseek"`），导致查询失败。

**数据格式不一致：**
- `traders.ai_model_id` 可能存储：`"admin_deepseek"`（用户特定ID）或 `"deepseek"`（provider）
- `ai_models.id` 存储：`"admin_deepseek"`（用户特定ID）
- `ai_models.provider` 存储：`"deepseek"`（标准provider）

### 修改文件
- `config/database.go`

### 具体修改

#### 修改 `GetTraderConfig` 函数中的 AI 模型查找逻辑（第 876-929 行）

**修改前：**
```go
// ai_model_id 存储的是 provider（如 "deepseek"），使用 provider 来查找 AI 模型
err = d.db.QueryRow(`
    SELECT id, user_id, name, provider, enabled, api_key, created_at, updated_at
    FROM ai_models
    WHERE provider = ? AND user_id = ?
`, trader.AIModelID, userID).Scan(
    &aiModel.ID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
    &aiModel.CreatedAt, &aiModel.UpdatedAt,
)

if err != nil {
    return nil, nil, nil, fmt.Errorf("AI模型配置不存在 (provider: %s, user_id: %s): %v", trader.AIModelID, userID, err)
}
```

**修改后：**
```go
// ai_model_id 可能是用户特定的ID（如 "admin_deepseek"）或 provider（如 "deepseek"）
// 首先尝试通过 ID 查找（新版逻辑）
err = d.db.QueryRow(`
    SELECT id, user_id, name, provider, enabled, api_key,
           COALESCE(custom_api_url, '') as custom_api_url,
           COALESCE(custom_model_name, '') as custom_model_name,
           created_at, updated_at
    FROM ai_models
    WHERE id = ? AND user_id = ?
`, trader.AIModelID, userID).Scan(
    &aiModel.ID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
    &aiModel.CustomAPIURL, &aiModel.CustomModelName,
    &aiModel.CreatedAt, &aiModel.UpdatedAt,
)

// 如果通过 ID 找不到，尝试通过 provider 查找（兼容旧数据）
if err != nil {
    err = d.db.QueryRow(`
        SELECT id, user_id, name, provider, enabled, api_key,
               COALESCE(custom_api_url, '') as custom_api_url,
               COALESCE(custom_model_name, '') as custom_model_name,
               created_at, updated_at
        FROM ai_models
        WHERE provider = ? AND user_id = ?
    `, trader.AIModelID, userID).Scan(
        &aiModel.ID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
        &aiModel.CustomAPIURL, &aiModel.CustomModelName,
        &aiModel.CreatedAt, &aiModel.UpdatedAt,
    )
}

// 如果还是找不到，尝试提取后缀作为 provider（例如 "admin_deepseek" -> "deepseek"）
if err != nil {
    if strings.Contains(trader.AIModelID, "_") {
        parts := strings.Split(trader.AIModelID, "_")
        lastPart := parts[len(parts)-1]
        err = d.db.QueryRow(`
            SELECT id, user_id, name, provider, enabled, api_key,
                   COALESCE(custom_api_url, '') as custom_api_url,
                   COALESCE(custom_model_name, '') as custom_model_name,
                   created_at, updated_at
            FROM ai_models
            WHERE (provider = ? OR id = ?) AND user_id = ?
        `, lastPart, lastPart, userID).Scan(
            &aiModel.ID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
            &aiModel.CustomAPIURL, &aiModel.CustomModelName,
            &aiModel.CreatedAt, &aiModel.UpdatedAt,
        )
    }
}

if err != nil {
    return nil, nil, nil, fmt.Errorf("AI模型配置不存在 (ai_model_id: %s, user_id: %s): %v", trader.AIModelID, userID, err)
}
```

### 修改说明
1. **多级匹配逻辑**：
   - 首先尝试通过 ID 直接匹配（适用于新数据，`ai_model_id` 存储用户特定ID的情况）
   - 如果找不到，尝试通过 provider 匹配（兼容旧数据，`ai_model_id` 存储 provider 的情况）
   - 如果还是找不到，提取 ID 后缀作为 provider 匹配（例如 `"admin_deepseek"` 的后缀 `"deepseek"` 匹配 provider）

2. **兼容新旧数据格式**：
   - 支持 `ai_model_id` 存储用户特定ID（如 `"admin_deepseek"`）
   - 支持 `ai_model_id` 存储 provider（如 `"deepseek"`）
   - 自动处理ID包含下划线的情况

3. **增强查询字段**：
   - 同时查询 `custom_api_url` 和 `custom_model_name` 字段，确保完整获取模型配置信息

### 影响范围
- ✅ 修复了编辑交易员时提示"AI模型配置不存在"的问题
- ✅ 支持用户特定ID和provider两种格式的匹配
- ✅ 兼容旧数据和新数据格式
- ✅ 不影响其他功能的正常运行

### 测试建议
1. 编辑一个使用用户特定ID（如 `"admin_deepseek"`）的交易员，应该能够正常加载配置
2. 编辑一个使用provider（如 `"deepseek"`）的交易员，应该能够正常加载配置
3. 验证不同ID格式都能正确匹配到对应的AI模型配置

---

## 2025-11-02 - 修复更新交易员时提示"AI模型配置不存在或未启用"的问题

### 问题描述
在修改交易员配置并保存时，系统提示"AI模型配置不存在或未启用"，导致无法保存编辑。即使模型和交易所都已启用，仍然提示此错误。

### 根本原因
问题有两个层面：

1. **数据格式不匹配**：
   - 后端 `handleGetTraderConfig` 返回的 `ai_model` 字段是处理后的 provider（如 `"deepseek"`），而不是用户特定的ID（如 `"admin_deepseek"`）
   - 前端的 `allModels` 中的模型 ID 是用户特定格式（如 `"admin_deepseek"`）
   - 当编辑时，`data.ai_model_id` 传入的是 `"deepseek"`（provider），但在 `allModels` 中查找时找不到匹配的模型，因为 `allModels` 中的 ID 是 `"admin_deepseek"`

2. **验证逻辑问题**：
   - 在 `handleSaveEditTrader` 函数中，代码从 `enabledModels`（已启用且有 API Key 的模型列表）中查找模型，如果模型被禁用或没有 API Key，就无法通过验证
   - 编辑模态框也使用 `enabledModels` 和 `enabledExchanges` 作为可用选项，导致无法选择被禁用的模型和交易所

### 修改文件
- `web/src/components/AITradersPage.tsx`
- `web/src/components/TraderConfigModal.tsx`

### 具体修改

#### 1. 修改 `TraderConfigModal` 组件中的模型ID匹配逻辑（第 67-99 行）

**修改前：**
```typescript
useEffect(() => {
  if (traderData) {
    setFormData(traderData);
    // 设置已选择的币种
    if (traderData.trading_symbols) {
      const coins = traderData.trading_symbols.split(',').map(s => s.trim()).filter(s => s);
      setSelectedCoins(coins);
    }
  }
```

**修改后：**
```typescript
useEffect(() => {
  if (traderData) {
    // 后端返回的 ai_model 可能是 provider（如 "deepseek"），需要匹配到 allModels 中的实际 ID
    let aiModelId = traderData.ai_model;
    
    // 尝试通过 ID 直接匹配
    let matchedModel = availableModels.find(m => m.id === aiModelId);
    
    // 如果找不到，尝试通过 provider 匹配
    if (!matchedModel) {
      matchedModel = availableModels.find(m => 
        m.provider === aiModelId || 
        m.id === aiModelId ||
        (m.id && m.id.endsWith('_' + aiModelId)) ||
        (m.id && m.id.split('_').pop() === aiModelId)
      );
    }
    
    // 如果找到了匹配的模型，使用它的 ID
    if (matchedModel) {
      aiModelId = matchedModel.id;
    }
    
    setFormData({
      ...traderData,
      ai_model: aiModelId  // 使用匹配到的模型 ID
    });
    
    // 设置已选择的币种
    if (traderData.trading_symbols) {
      const coins = traderData.trading_symbols.split(',').map(s => s.trim()).filter(s => s);
      setSelectedCoins(coins);
    }
  }
```

#### 2. 修改 `handleSaveEditTrader` 函数中的模型和交易所查找逻辑（第 164-209 行）

**修改前：**
```typescript
const handleSaveEditTrader = async (data: CreateTraderRequest) => {
  if (!editingTrader) return;

  try {
    const model = enabledModels?.find(m => m.id === data.ai_model_id);
    const exchange = enabledExchanges?.find(e => e.id === data.exchange_id);

    if (!model) {
      alert(t('modelConfigNotExist', language));
      return;
    }

    if (!exchange) {
      alert(t('exchangeConfigNotExist', language));
      return;
    }
```

**修改后：**
```typescript
const handleSaveEditTrader = async (data: CreateTraderRequest) => {
  if (!editingTrader) return;

  try {
    // 编辑模式下，从 allModels 和 allExchanges 中查找，允许编辑被禁用的配置
    let model = allModels?.find(m => m.id === data.ai_model_id);
    
    // 如果通过 ID 找不到，尝试通过 provider 匹配
    if (!model && data.ai_model_id) {
      model = allModels?.find(m => 
        m.provider === data.ai_model_id ||
        m.id === data.ai_model_id ||
        (m.id && m.id.endsWith('_' + data.ai_model_id)) ||
        (m.id && m.id.split('_').pop() === data.ai_model_id)
      );
    }
    
    const exchange = allExchanges?.find(e => e.id === data.exchange_id);

    if (!model) {
      console.error('模型未找到:', {
        ai_model_id: data.ai_model_id,
        allModelsIds: allModels?.map(m => ({ id: m.id, provider: m.provider, enabled: m.enabled })),
        allModelsCount: allModels?.length
      });
      alert(t('modelConfigNotExist', language));
      return;
    }

    if (!exchange) {
      console.error('交易所未找到:', {
        exchange_id: data.exchange_id,
        allExchangesIds: allExchanges?.map(e => ({ id: e.id, enabled: e.enabled })),
        allExchangesCount: allExchanges?.length
      });
      alert(t('exchangeConfigNotExist', language));
      return;
    }
    
    // 如果找到了匹配的模型，使用它的 ID（确保使用正确的ID格式）
    const finalAIModelId = model.id;

    const request = {
      name: data.name,
      ai_model_id: finalAIModelId,  // 使用匹配到的模型 ID，而不是可能不匹配的 data.ai_model_id
      exchange_id: data.exchange_id,
      // ...
    };
```

#### 2. 修改编辑模态框的模型和交易所列表（第 785-800 行）

**修改前：**
```typescript
{/* Edit Trader Modal */}
{showEditModal && editingTrader && (
  <TraderConfigModal
    isOpen={showEditModal}
    isEditMode={true}
    traderData={editingTrader}
    availableModels={enabledModels}
    availableExchanges={enabledExchanges}
    onSave={handleSaveEditTrader}
    onClose={() => {
      setShowEditModal(false);
      setEditingTrader(null);
    }}
  />
)}
```

**修改后：**
```typescript
{/* Edit Trader Modal */}
{showEditModal && editingTrader && (
  <TraderConfigModal
    isOpen={showEditModal}
    isEditMode={true}
    traderData={editingTrader}
    // 编辑模式下使用 allModels 和 allExchanges，以便编辑被禁用的配置
    availableModels={allModels}
    availableExchanges={allExchanges}
    onSave={handleSaveEditTrader}
    onClose={() => {
      setShowEditModal(false);
      setEditingTrader(null);
    }}
  />
)}
```

### 修改说明
1. **模型ID匹配逻辑**：
   - 在 `TraderConfigModal` 中，当加载交易员配置时，尝试将后端返回的 `ai_model`（可能是 provider，如 `"deepseek"`）匹配到 `availableModels` 中的实际模型ID（如 `"admin_deepseek"`）
   - 支持通过 ID 直接匹配、通过 provider 匹配、通过ID后缀匹配等多种方式

2. **保存时验证逻辑**：
   - 在 `handleSaveEditTrader` 中，首先尝试通过 ID 直接匹配
   - 如果找不到，尝试通过 provider 或后缀匹配
   - 如果找到了匹配的模型，使用其实际的 ID 发送到后端，而不是可能不匹配的 `data.ai_model_id`

3. **编辑验证逻辑**：在保存编辑时，从 `allModels` 和 `allExchanges` 中查找模型和交易所，而不是从过滤后的 `enabledModels` 和 `enabledExchanges` 中查找。这样允许用户编辑使用被禁用配置的交易员。

4. **编辑模态框选项**：编辑模式下，模态框使用 `allModels` 和 `allExchanges` 作为可用选项，确保所有已配置的模型和交易所都可以选择，即使它们当前被禁用。

5. **保持创建验证不变**：创建新交易员时仍然使用 `enabledModels` 和 `enabledExchanges`，确保只有启用且配置完整的模型和交易所才能用于新交易员。

6. **增强错误调试**：添加了详细的日志输出，当找不到模型或交易所时，输出所有可用的ID列表，便于调试问题。

### 影响范围
- ✅ 修复了更新交易员时提示"AI模型配置不存在或未启用"的问题
- ✅ 允许用户在编辑时选择任何已配置的模型和交易所（即使被禁用）
- ✅ 不影响创建新交易员的验证逻辑
- ✅ 允许编辑使用被禁用配置的交易员

### 测试建议
1. 创建一个使用某个模型的交易员
2. 禁用该模型
3. 尝试编辑该交易员，应该能够正常保存
4. 验证创建新交易员时仍然只显示已启用的模型和交易所

---

## 2025-11-02 - 修复编辑模型配置时提示"模型不存在"的问题

### 问题描述
在编辑模型配置界面保存时，系统提示"模型不存在"，导致无法保存编辑。

### 根本原因
在 `handleSaveModelConfig` 函数中，当编辑已配置的模型时，传入的 `modelId` 是来自 `allModels`（用户已配置的模型列表）的 ID，格式可能是 `"admin_deepseek"` 或 `"user_deepseek"` 这样的用户特定 ID。但是代码只在 `supportedModels`（系统支持的模型列表）中通过 ID 直接匹配查找，而 `supportedModels` 中的 ID 格式可能是 `"deepseek"` 这样的系统标准 ID，导致匹配失败。

**数据来源区别：**
- `allModels`：从 `/api/models` 获取，是当前用户已配置的模型列表，包含 API Key 等用户配置信息，ID 可能包含用户标识前缀（如 `"admin_deepseek"`）
- `supportedModels`：从 `/api/supported-models` 获取，是系统支持的所有模型列表，只有模型基本信息，ID 通常是标准的 provider 名称（如 `"deepseek"`）

### 修改文件
- `web/src/components/AITradersPage.tsx`

### 具体修改

#### 修改 `handleSaveModelConfig` 函数（第 278-331 行）

**修改前：**
```typescript
const handleSaveModelConfig = async (modelId: string, apiKey: string, customApiUrl?: string, customModelName?: string) => {
  try {
    // 找到要配置的模型（从supportedModels中）
    const modelToUpdate = supportedModels?.find(m => m.id === modelId);
    if (!modelToUpdate) {
      alert(t('modelNotExist', language));
      return;
    }
```

**修改后：**
```typescript
const handleSaveModelConfig = async (modelId: string, apiKey: string, customApiUrl?: string, customModelName?: string) => {
  try {
    // 首先从已配置的模型中查找（编辑模式时使用）
    let configuredModel = allModels?.find(m => m.id === modelId);
    
    // 从supportedModels中查找模型
    let modelToUpdate = supportedModels?.find(m => m.id === modelId);
    
    // 如果通过ID找不到，尝试通过provider匹配
    if (!modelToUpdate && configuredModel?.provider) {
      modelToUpdate = supportedModels?.find(m => 
        m.provider === configuredModel.provider || 
        m.id === configuredModel.provider ||
        (configuredModel.id && m.id === configuredModel.id)
      );
    }
    
    // 如果还是找不到，尝试在supportedModels中查找任何匹配的模型
    if (!modelToUpdate) {
      // 尝试通过ID的后缀匹配（例如 "admin_deepseek" 匹配 "deepseek"）
      const modelIdParts = modelId.split('_');
      const lastPart = modelIdParts[modelIdParts.length - 1];
      modelToUpdate = supportedModels?.find(m => 
        m.id === lastPart || 
        m.id === modelId || 
        m.provider === lastPart ||
        m.provider === modelId
      );
    }
    
    if (!modelToUpdate) {
      console.error('模型不存在:', { 
        modelId, 
        supportedModelsIds: supportedModels?.map(m => m.id),
        allModelsIds: allModels?.map(m => m.id)
      });
      alert(t('modelNotExist', language));
      return;
    }
```

#### 改进 existingModel 查找逻辑

**修改后：**
```typescript
// 创建或更新用户的模型配置
let existingModel = configuredModel;
if (!existingModel && modelToUpdate?.provider) {
  existingModel = allModels?.find(m => 
    m.provider === modelToUpdate.provider || 
    m.id === modelToUpdate.provider ||
    m.id === modelToUpdate.id
  );
}
```

### 修改说明
1. **多级匹配逻辑**：
   - 首先尝试通过 ID 直接匹配（适用于新建模式和ID格式一致的情况）
   - 如果找不到，通过 `provider` 匹配（适用于编辑模式，用户配置的ID包含前缀的情况）
   - 如果还是找不到，通过ID后缀匹配（例如 `"admin_deepseek"` 的后缀 `"deepseek"` 匹配系统标准ID）

2. **兼容编辑和新建模式**：
   - 编辑模式：`modelId` 来自 `allModels`（用户已配置），可能是 `"admin_deepseek"` 格式
   - 新建模式：`modelId` 来自 `supportedModels`（系统支持），是 `"deepseek"` 格式
   - 两种模式都能正确匹配到对应的模型

3. **保持数据完整性**：
   - 更新时保持使用原有的模型ID（用户配置的ID），确保数据库一致性
   - 新建时使用系统标准的模型ID

### 影响范围
- ✅ 修复了编辑模型配置时提示"模型不存在"的问题
- ✅ 支持编辑模式时用户特定ID与系统标准ID的匹配
- ✅ 不影响新建模型配置的功能
- ✅ 增强了模型匹配的健壮性

### 测试建议
1. 编辑一个已配置的模型，应该能够正常保存
2. 新建一个模型配置，应该能够正常保存
3. 验证不同ID格式（包含前缀和不包含前缀）都能正确匹配
4. 检查控制台是否有匹配失败的错误日志

---

## 2025-11-01 - 优化前端部署路径配置

### 优化描述
将前端应用的部署路径设置为 `/nofx/`，同时配置 API 代理路径为 `/nofx-api`，实现前端和后端在同一域名下的子路径部署。

### 优化目的
1. **支持子路径部署**：允许前端部署在 `/nofx/` 路径下，而不是根路径 `/`
2. **统一路由管理**：通过 Vite 的 `base` 配置统一管理前端资源路径
3. **API 代理优化**：通过 Vite 开发服务器的代理功能，将 `/nofx-api` 请求代理到后端的 `/api` 接口
4. **部署灵活性**：支持将前后端部署在同一域名下，提高部署灵活性

### 修改文件
- `web/vite.config.ts` - Vite 配置（base 路径和代理配置）
- `web/src/lib/api.ts` - API 基础路径常量
- `web/src/lib/config.ts` - 系统配置获取（使用 `/nofx-api/config`）
- `web/src/contexts/AuthContext.tsx` - 认证相关 API 调用（登录、注册、OTP验证等）
- `web/src/components/TraderConfigModal.tsx` - 交易员配置相关 API 调用
- `web/src/components/ModelIcons.tsx` - 静态资源路径工具函数（使用 BASE_URL）

### 具体修改

#### 1. 修改 `web/vite.config.ts` - 添加 base 路径和 API 代理配置

**修改内容（第6行和第10-15行）：**

```typescript
export default defineConfig({
  plugins: [react()],
  base: '/nofx/',  // 设置前端部署基础路径
  server: {
    host: '0.0.0.0',
    port: 3000,
    proxy: {
      '/nofx-api': {  // 配置 API 代理路径
        target: 'http://localhost:18080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/nofx-api/, '/api'),  // 将 /nofx-api 重写为 /api
      },
    },
  },
})
```

#### 2. 修改 `web/src/lib/api.ts` - 统一 API 基础路径

**修改内容（第16行）：**

```typescript
const API_BASE = '/nofx-api';
```

**说明**：
- 所有通过 `api` 对象调用的接口都使用 `API_BASE` 常量
- 覆盖了交易员管理、模型配置、交易所配置、状态查询等所有主要 API 接口（共30+个接口）

#### 3. 修改 `web/src/lib/config.ts` - 系统配置 API 路径

**修改内容（第15行）：**

```typescript
configPromise = fetch('/nofx-api/config')
```

**说明**：
- 系统配置获取接口统一使用 `/nofx-api/config`

#### 4. 修改 `web/src/contexts/AuthContext.tsx` - 认证相关 API 路径

**修改内容（第63、94、122、155行）：**

```typescript
// 登录
const response = await fetch('/nofx-api/login', { ... });

// 注册
const response = await fetch('/nofx-api/register', { ... });

// OTP 验证
const response = await fetch('/nofx-api/verify-otp', { ... });

// 完成注册
const response = await fetch('/nofx-api/complete-registration', { ... });
```

**说明**：
- 所有认证相关的 API 调用都统一使用 `/nofx-api` 前缀

#### 5. 修改 `web/src/components/TraderConfigModal.tsx` - 交易员配置相关 API 路径

**修改内容（第132、150行）：**

```typescript
// 获取系统配置
const response = await fetch('/nofx-api/config');

// 获取提示词模板列表
const response = await fetch('/nofx-api/prompt-templates');
```

**说明**：
- 交易员配置模态框中的直接 API 调用统一使用 `/nofx-api` 前缀

#### 6. 修改 `web/src/components/ModelIcons.tsx` - 静态资源路径适配 base 路径

**修改内容（第4-15行）：**

```typescript
// 获取图标路径的工具函数
// BASE_URL 是 Vite 内置环境变量，自动从 vite.config.ts 的 base 配置中获取
// 例如 base: '/nofx/' 时，BASE_URL 的值就是 '/nofx/'
export const getIconPath = (iconName: string): string => {
  const baseUrl = import.meta.env.BASE_URL || '/';
  // BASE_URL 通常是 '/nofx/' 这样的格式（以 / 开头和结尾）
  // 直接拼接即可，不需要移除末尾斜杠
  return `${baseUrl}icons/${iconName}`;
};

// 获取图片路径的工具函数
export const getImagePath = (imageName: string): string => {
  const baseUrl = import.meta.env.BASE_URL || '/';
  return `${baseUrl}images/${imageName}`;
};
```

**说明**：
- 使用 `import.meta.env.BASE_URL` 自动适配 base 路径配置
- 所有静态资源（图片、图标）通过工具函数获取路径，自动添加 `/nofx/` 前缀
- 影响范围：所有使用 `getImagePath` 和 `getIconPath` 的组件（共10+个文件）

**使用这些工具函数的文件**：
- `web/src/App.tsx` - Logo 图片和图标
- `web/src/components/Header.tsx` - Logo 图片
- `web/src/components/LoginPage.tsx` - Logo 图片
- `web/src/components/RegisterPage.tsx` - Logo 图片
- `web/src/components/landing/HeaderBar.tsx` - Logo 图片
- `web/src/components/landing/FooterSection.tsx` - Logo 图片
- `web/src/components/landing/HeroSection.tsx` - 主图
- 以及所有使用模型图标和交易所图标的地方

### 修改说明
1. **base 路径配置**：
   - 设置 `base: '/nofx/'`，使前端资源路径基于 `/nofx/` 路径
   - 所有静态资源（JS、CSS、图片等）都会自动添加 `/nofx/` 前缀
   - Vite 会自动设置 `import.meta.env.BASE_URL` 为 `/nofx/`

2. **API 代理配置**：
   - 配置开发服务器代理，将 `/nofx-api` 路径的请求代理到后端
   - `target: 'http://localhost:18080'` 指定后端服务地址
   - `rewrite` 规则将 `/nofx-api` 重写为 `/api`，保持后端 API 路径不变

3. **API 路径统一**：
   - 统一使用 `API_BASE = '/nofx-api'` 常量作为所有 API 调用的基础路径
   - 所有直接 `fetch` 调用都使用 `/nofx-api` 前缀
   - 开发环境下通过 Vite 代理转发请求
   - 生产环境下需要配置反向代理（如 Nginx）将 `/nofx-api` 转发到后端

4. **影响的接口范围**：
   - 交易员管理：创建、更新、删除、启动、停止、获取配置等
   - 模型配置：获取、更新模型配置等
   - 交易所配置：获取、更新交易所配置等
   - 认证相关：登录、注册、OTP验证等
   - 系统配置：获取系统配置、提示词模板列表等
   - 状态查询：账户、持仓、决策、统计等（共30+个接口）

### 相关影响
- ✅ 前端路由自动适配 `/nofx/` 基础路径
- ✅ 静态资源路径自动添加 `/nofx/` 前缀
  - 所有图片（logo.png、main.png）通过 `getImagePath` 自动适配
  - 所有图标（模型图标、交易所图标）通过 `getIconPath` 自动适配
  - 共影响10+个组件文件
- ✅ API 请求路径统一使用 `/nofx-api`，通过代理转发
- ✅ 支持前后端同域部署，提高部署灵活性

### 生产环境部署建议
在生产环境中，需要在反向代理（如 Nginx）中配置：
```nginx
# 前端静态资源
location /nofx/ {
    alias /path/to/web/dist/;
    try_files $uri $uri/ /nofx/index.html;
}

# API 代理
location /nofx-api {
    proxy_pass http://localhost:18080/api;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

---

## 如何使用本日志

在合并新版本时：
1. 检查本日志中记录的修改是否在新版本中已存在
2. 如果已存在，标记为已完成
3. 如果不存在，需要重新应用这些修改
4. 注意新版本中相关代码的结构变化，可能需要调整修改方式

