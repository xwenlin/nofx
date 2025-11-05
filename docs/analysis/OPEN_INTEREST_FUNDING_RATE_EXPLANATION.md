# "Open Interest and Funding Rate for Perps" 详解

## 📝 这句话的含义

**原文**：
```
"In addition, here is the latest %s open interest and funding rate for perps:"
```

**中文翻译**：
```
"另外，这是 %s 永续合约的最新持仓量和资金费率："
```

**详细解释**：
- **"In addition"** = 另外/此外
- **"here is the latest"** = 这是最新的
- **"%s"** = 币种符号（如 BTCUSDT、ETHUSDT 等）
- **"open interest"** = 持仓量（未平仓合约数量）
- **"funding rate"** = 资金费率
- **"for perps"** = 对于永续合约（perpetual futures）

**完整意思**：这句话是在告诉AI，接下来要提供的是某个币种在永续合约市场上的最新持仓量和资金费率数据。

---

## 🔍 数据来源和计算位置

### 1. Open Interest (持仓量) 数据

**调用位置**：
```68:73:market/data.go
	// 获取OI数据
	oiData, err := getOpenInterestData(symbol)
	if err != nil {
		// OI失败不影响整体,使用默认值
		oiData = &OIData{Latest: 0, Average: 0, DeltaPercent: 0}
	}
```

**数据获取函数**：
```333:393:market/data.go
// getOpenInterestData 获取OI数据（包括变化百分比）
func getOpenInterestData(symbol string) (*OIData, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	oiLatest, _ := strconv.ParseFloat(result.OpenInterest, 64)

	// 计算平均OI（使用当前值作为近似，如果有历史数据可以改进）
	oiAverage := oiLatest * 0.999 // 近似平均值

	// 尝试获取历史OI数据来计算变化百分比
	// Binance的openInterestHist接口需要指定时间范围，这里使用简化的方法
	// 获取24小时前的OI数据（如果有的话）
	oiDeltaPercent := 0.0
	histURL := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterestHist?symbol=%s&period=5m&limit=1", symbol)
	histResp, histErr := http.Get(histURL)
	if histErr == nil {
		defer histResp.Body.Close()
		histBody, _ := io.ReadAll(histResp.Body)
		var histResult []struct {
			SumOpenInterest string `json:"sumOpenInterest"`
			SumOpenInterestValue string `json:"sumOpenInterestValue"`
		}
		if json.Unmarshal(histBody, &histResult) == nil && len(histResult) > 0 {
			if histOI, parseErr := strconv.ParseFloat(histResult[0].SumOpenInterest, 64); parseErr == nil && histOI > 0 {
				oiDeltaPercent = ((oiLatest - histOI) / histOI) * 100
			}
		}
	}

	// 如果无法获取历史数据，使用当前值和平均值的差异作为近似
	if oiDeltaPercent == 0 && oiAverage > 0 {
		oiDeltaPercent = ((oiLatest - oiAverage) / oiAverage) * 100
	}

	return &OIData{
		Latest:      oiLatest,
		Average:     oiAverage,
		DeltaPercent: oiDeltaPercent,
	}, nil
}
```

**数据来源**：
- **API端点**：`https://fapi.binance.com/fapi/v1/openInterest?symbol={symbol}`
- **数据字段**：
  - `Latest`：最新持仓量（从API直接获取）
  - `Average`：平均持仓量（当前使用 `Latest * 0.999` 作为近似值）
  - `DeltaPercent`：持仓量变化百分比（尝试从历史API获取，失败则使用当前值和平均值的差异）

### 2. Funding Rate (资金费率) 数据

**调用位置**：
```75:76:market/data.go
	// 获取Funding Rate
	fundingRate, _ := getFundingRate(symbol)
```

**数据获取函数**：
```395:426:market/data.go
// getFundingRate 获取资金费率
func getFundingRate(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)
	return rate, nil
}
```

**数据来源**：
- **API端点**：`https://fapi.binance.com/fapi/v1/premiumIndex?symbol={symbol}`
- **数据字段**：从API响应的 `lastFundingRate` 字段获取

### 3. 数据格式化输出

**输出位置**：
```435:443:market/data.go
	sb.WriteString(fmt.Sprintf("In addition, here is the latest %s open interest and funding rate for perps:\n\n",
		data.Symbol))

	if data.OpenInterest != nil {
		sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f Average: %.2f Delta: %.2f%%\n\n",
			data.OpenInterest.Latest, data.OpenInterest.Average, data.OpenInterest.DeltaPercent))
	}

	sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))
```

---

## 📊 数据含义解释

### Open Interest (持仓量)

**定义**：
- 持仓量是指当前市场上所有未平仓的永续合约总数量
- 它反映了市场参与者的整体持仓规模

**交易意义**：
1. **持仓量增加 + 价格上涨** → 强势上涨趋势（多方力量强）
2. **持仓量增加 + 价格下跌** → 强势下跌趋势（空方力量强）
3. **持仓量减少** → 趋势可能减弱（资金流出）
4. **持仓量变化 >+5%** → 真实突破确认（提示词要求）

**代码中的使用**：
- 提示词在多空确认清单中使用OI变化来判断市场方向
- OI变化>+5%被视为真实突破的信号

### Funding Rate (资金费率)

**定义**：
- 资金费率是永续合约市场每8小时结算一次的费率
- 用于平衡永续合约价格与现货价格的差异

**费率含义**：
- **正费率**（>0）：多头支付空头 → 市场看涨情绪强
- **负费率**（<0）：空头支付多头 → 市场看跌情绪强
- **极端费率**（>0.01%或<-0.01%）：可能反转信号

**交易意义**：
1. **极端正费率** → 市场极度看涨，可能反转做空
2. **极端负费率** → 市场极度看跌，可能反转做多
3. **正常费率**（-0.01%~0.01%）→ 市场情绪中性

**代码中的使用**：
- 提示词在多空确认清单中使用资金费率判断市场情绪
- 极端费率（>0.01%或<-0.01%）被视为反转信号

---

## 🔄 数据流程

```
1. Get() 函数被调用
   ↓
2. 调用 getOpenInterestData(symbol)
   → HTTP GET https://fapi.binance.com/fapi/v1/openInterest?symbol={symbol}
   → 解析JSON响应，提取OpenInterest字段
   → 尝试获取历史OI数据计算变化百分比
   → 返回 OIData{Latest, Average, DeltaPercent}
   ↓
3. 调用 getFundingRate(symbol)
   → HTTP GET https://fapi.binance.com/fapi/v1/premiumIndex?symbol={symbol}
   → 解析JSON响应，提取LastFundingRate字段
   → 返回 float64 资金费率
   ↓
4. 将数据存储到 Data 结构
   → Data.OpenInterest = oiData
   → Data.FundingRate = fundingRate
   ↓
5. Format() 函数格式化输出
   → 输出 "In addition, here is the latest {symbol} open interest and funding rate for perps:"
   → 输出 "Open Interest: Latest: {Latest} Average: {Average} Delta: {DeltaPercent}%"
   → 输出 "Funding Rate: {FundingRate}"
   ↓
6. 传递给AI进行决策分析
```

---

## 📝 总结

1. **这句话的作用**：告诉AI接下来要提供的是持仓量和资金费率数据
2. **数据来源**：都是从Binance期货API获取的实时数据
3. **数据位置**：
   - Open Interest：`getOpenInterestData()` 函数（第333-393行）
   - Funding Rate：`getFundingRate()` 函数（第395-426行）
4. **数据用途**：用于AI的多空确认清单和市场情绪判断

这两个数据对于永续合约交易非常重要，因为它们能反映市场的真实情绪和资金流向。

