package market

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// Get 获取指定代币的市场数据
func Get(symbol string) (*Data, error) {
	var klines3m, klines15m, klines1h, klines4h []Kline
	var err error
	// 标准化symbol
	symbol = Normalize(symbol)
	// 获取3分钟K线数据（获取100个用于计算指标，最后20个传递给AI）
	klines3m, err = WSMonitorCli.GetCurrentKlines(symbol, "3m") // 多获取一些用于计算
	if err != nil {
		return nil, fmt.Errorf("获取3分钟K线失败: %v", err)
	}

	// 获取15分钟K线数据（获取100个用于计算指标，最后20个传递给AI）
	klines15m, err = WSMonitorCli.GetCurrentKlines(symbol, "15m")
	if err != nil {
		return nil, fmt.Errorf("获取15分钟K线失败: %v", err)
	}

	// 获取1小时K线数据（获取100个用于计算指标，最后20个传递给AI）
	klines1h, err = WSMonitorCli.GetCurrentKlines(symbol, "1h")
	if err != nil {
		return nil, fmt.Errorf("获取1小时K线失败: %v", err)
	}

	// 获取4小时K线数据（获取100个用于计算指标，最后20个传递给AI）
	klines4h, err = WSMonitorCli.GetCurrentKlines(symbol, "4h") // 多获取用于计算指标
	if err != nil {
		return nil, fmt.Errorf("获取4小时K线失败: %v", err)
	}

	// 计算当前指标 (基于3分钟最新数据)
	currentPrice := klines3m[len(klines3m)-1].Close
	currentEMA20 := calculateEMA(klines3m, 20)
	currentMACD := calculateMACD(klines3m)
	currentRSI7 := calculateRSI(klines3m, 7)

	// 计算价格变化百分比
	// 1小时价格变化 = 20个3分钟K线前的价格
	priceChange1h := 0.0
	if len(klines3m) >= 21 { // 至少需要21根K线 (当前 + 20根前)
		price1hAgo := klines3m[len(klines3m)-21].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	}

	// 4小时价格变化 = 1个4小时K线前的价格
	priceChange4h := 0.0
	if len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}

	// 获取OI数据
	oiData, err := getOpenInterestData(symbol)
	if err != nil {
		// OI失败不影响整体,使用默认值
		oiData = &OIData{Latest: 0, Average: 0, DeltaPercent: 0}
	}

	// 获取Funding Rate
	fundingRate, _ := getFundingRate(symbol)

	// 计算各时间框架的系列数据
	intradayData := calculateIntradaySeries(klines3m)   // 3分钟序列
	series15m := calculateIntradaySeries(klines15m)     // 15分钟序列
	series1h := calculateIntradaySeries(klines1h)       // 1小时序列
	longerTermData := calculateLongerTermData(klines4h) // 4小时序列

	return &Data{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		PriceChange1h:     priceChange1h,
		PriceChange4h:     priceChange4h,
		CurrentEMA20:      currentEMA20,
		CurrentMACD:       currentMACD,
		CurrentRSI7:       currentRSI7,
		OpenInterest:      oiData,
		FundingRate:       fundingRate,
		IntradaySeries:    intradayData,
		Series15m:         series15m,
		Series1h:          series1h,
		LongerTermContext: longerTermData,
	}, nil
}

// calculateEMA 计算EMA
func calculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// 计算SMA作为初始EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// 计算EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

// calculateMACD 计算MACD
func calculateMACD(klines []Kline) float64 {
	if len(klines) < 26 {
		return 0
	}

	// 计算12期和26期EMA
	ema12 := calculateEMA(klines, 12)
	ema26 := calculateEMA(klines, 26)

	// MACD = EMA12 - EMA26
	return ema12 - ema26
}

// calculateRSI 计算RSI
func calculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	gains := 0.0
	losses := 0.0

	// 计算初始平均涨跌幅
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// 使用Wilder平滑方法计算后续RSI
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateATR 计算ATR
func calculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	trs := make([]float64, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// 计算初始ATR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Wilder平滑
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

// calculateIntradaySeries 计算日内系列数据
func calculateIntradaySeries(klines []Kline) *IntradayData {
	data := &IntradayData{
		MidPrices:     make([]float64, 0, 20),
		EMA20Values:   make([]float64, 0, 20),
		MACDValues:    make([]float64, 0, 20),
		RSI7Values:    make([]float64, 0, 20),
		RSI14Values:   make([]float64, 0, 20),
		Volumes:       make([]float64, 0, 20),
		BuySellRatios: make([]float64, 0, 20),
	}

	// 获取最近20个数据点（用于AI深度分析）
	start := len(klines) - 20
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		data.MidPrices = append(data.MidPrices, klines[i].Close)

		// 计算成交量序列
		data.Volumes = append(data.Volumes, klines[i].Volume)

		// 计算买卖压力比（BuySellRatio = TakerBuyBaseVolume / Volume）
		if klines[i].Volume > 0 {
			buySellRatio := klines[i].TakerBuyBaseVolume / klines[i].Volume
			data.BuySellRatios = append(data.BuySellRatios, buySellRatio)
		} else {
			data.BuySellRatios = append(data.BuySellRatios, 0.5) // 默认中性值
		}

		// 计算每个点的EMA20（需要至少20个数据点）
		// i+1 表示从数组开头到当前位置总共有多少个数据点
		if i+1 >= 20 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		} else {
			// 没有足够数据时，添加0以保持数组长度一致
			data.EMA20Values = append(data.EMA20Values, 0)
		}

		// 计算每个点的MACD（需要至少26个数据点）
		if i+1 >= 26 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		} else {
			data.MACDValues = append(data.MACDValues, 0)
		}

		// 计算每个点的RSI7（需要至少8个数据点）
		if i+1 >= 8 {
			rsi7 := calculateRSI(klines[:i+1], 7)
			data.RSI7Values = append(data.RSI7Values, rsi7)
		} else {
			data.RSI7Values = append(data.RSI7Values, 0)
		}

		// 计算每个点的RSI14（需要至少15个数据点）
		if i+1 >= 15 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		} else {
			data.RSI14Values = append(data.RSI14Values, 0)
		}
	}

	return data
}

// calculateLongerTermData 计算长期数据
func calculateLongerTermData(klines []Kline) *LongerTermData {
	data := &LongerTermData{
		MidPrices:     make([]float64, 0, 20),
		EMA20Values:   make([]float64, 0, 20),
		MACDValues:    make([]float64, 0, 20),
		RSI7Values:    make([]float64, 0, 20),
		RSI14Values:   make([]float64, 0, 20),
		Volumes:       make([]float64, 0, 20),
		BuySellRatios: make([]float64, 0, 20),
	}

	// 计算EMA（单值，保留用于兼容）
	data.EMA20 = calculateEMA(klines, 20)
	data.EMA50 = calculateEMA(klines, 50)

	// 计算ATR
	data.ATR3 = calculateATR(klines, 3)
	data.ATR14 = calculateATR(klines, 14)

	// 计算成交量（单值，保留用于兼容）
	if len(klines) > 0 {
		data.CurrentVolume = klines[len(klines)-1].Volume
		// 计算平均成交量
		sum := 0.0
		for _, k := range klines {
			sum += k.Volume
		}
		data.AverageVolume = sum / float64(len(klines))
	}

	// 计算序列数据（获取最近20个数据点用于AI深度分析）
	start := len(klines) - 20
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		// 添加价格序列
		data.MidPrices = append(data.MidPrices, klines[i].Close)

		// 计算成交量序列
		data.Volumes = append(data.Volumes, klines[i].Volume)

		// 计算买卖压力比（BuySellRatio = TakerBuyBaseVolume / Volume）
		if klines[i].Volume > 0 {
			buySellRatio := klines[i].TakerBuyBaseVolume / klines[i].Volume
			data.BuySellRatios = append(data.BuySellRatios, buySellRatio)
		} else {
			data.BuySellRatios = append(data.BuySellRatios, 0.5) // 默认中性值
		}

		// 计算每个点的EMA20序列（需要至少20个数据点）
		if i+1 >= 20 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		} else {
			data.EMA20Values = append(data.EMA20Values, 0)
		}

		// 计算MACD序列（需要至少26个数据点）
		if i+1 >= 26 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		} else {
			data.MACDValues = append(data.MACDValues, 0)
		}

		// 计算RSI7序列（需要至少8个数据点）
		if i+1 >= 8 {
			rsi7 := calculateRSI(klines[:i+1], 7)
			data.RSI7Values = append(data.RSI7Values, rsi7)
		} else {
			data.RSI7Values = append(data.RSI7Values, 0)
		}

		// 计算RSI14序列（需要至少15个数据点）
		if i+1 >= 15 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		} else {
			data.RSI14Values = append(data.RSI14Values, 0)
		}
	}

	return data
}

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

	// 尝试获取历史OI数据来计算变化百分比和平均值
	oiDeltaPercent := 0.0
	oiAverage := oiLatest // 默认使用当前值作为平均值（如果无法获取历史数据）

	// 获取24小时的历史OI数据（每1小时一个数据点，共24个）
	histURL := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterestHist?symbol=%s&period=1h&limit=24", symbol)
	histResp, histErr := http.Get(histURL)
	if histErr == nil {
		defer histResp.Body.Close()
		histBody, _ := io.ReadAll(histResp.Body)
		var histResult []struct {
			SumOpenInterest      string `json:"sumOpenInterest"`
			SumOpenInterestValue string `json:"sumOpenInterestValue"`
		}
		if json.Unmarshal(histBody, &histResult) == nil && len(histResult) > 0 {
			// 计算平均值（使用所有历史数据点）
			sum := 0.0
			validCount := 0
			for _, item := range histResult {
				if histOI, parseErr := strconv.ParseFloat(item.SumOpenInterest, 64); parseErr == nil && histOI > 0 {
					sum += histOI
					validCount++
				}
			}

			if validCount > 0 {
				oiAverage = sum / float64(validCount)

				// 计算变化百分比（使用最早的历史数据，即24小时前）
				// 历史数据按时间倒序排列，第一个是最早的
				earliestOIStr := histResult[len(histResult)-1].SumOpenInterest
				if earliestOI, parseErr := strconv.ParseFloat(earliestOIStr, 64); parseErr == nil && earliestOI > 0 {
					oiDeltaPercent = ((oiLatest - earliestOI) / earliestOI) * 100
				} else if len(histResult) > 0 {
					// 如果最后一个解析失败，尝试第一个
					if firstOI, parseErr := strconv.ParseFloat(histResult[0].SumOpenInterest, 64); parseErr == nil && firstOI > 0 {
						oiDeltaPercent = ((oiLatest - firstOI) / firstOI) * 100
					}
				}
			}
		}
	}

	// 如果无法获取历史数据或历史数据无效，使用当前值作为平均值
	// 但不设置假的DeltaPercent（保持为0），让AI知道这是当前数据，无法判断变化
	if oiDeltaPercent == 0 && oiAverage == oiLatest {
		// 这种情况下，我们无法知道真实的变化，DeltaPercent保持为0
		// 这意味着AI应该忽略OI变化这个指标，或者使用其他指标
	}

	return &OIData{
		Latest:       oiLatest,
		Average:      oiAverage,
		DeltaPercent: oiDeltaPercent,
	}, nil
}

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

// Format 格式化输出市场数据
func Format(data *Data) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("current_price = %.2f, current_ema20 = %.3f, current_macd = %.3f, current_rsi (7 period) = %.3f\n\n",
		data.CurrentPrice, data.CurrentEMA20, data.CurrentMACD, data.CurrentRSI7))

	sb.WriteString(fmt.Sprintf("In addition, here is the latest %s open interest and funding rate for perps:\n\n",
		data.Symbol))

	if data.OpenInterest != nil {
		sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f Average: %.2f Delta: %.2f%%\n\n",
			data.OpenInterest.Latest, data.OpenInterest.Average, data.OpenInterest.DeltaPercent))
	}

	sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))

	// 3分钟序列
	if data.IntradaySeries != nil {
		sb.WriteString("3-minute series (oldest → latest):\n\n")

		if len(data.IntradaySeries.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
		}

		if len(data.IntradaySeries.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
		}

		if len(data.IntradaySeries.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
		}

		if len(data.IntradaySeries.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
		}

		if len(data.IntradaySeries.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
		}

		if len(data.IntradaySeries.Volumes) > 0 {
			sb.WriteString(fmt.Sprintf("Volumes: %s\n\n", formatFloatSlice(data.IntradaySeries.Volumes)))
		}

		if len(data.IntradaySeries.BuySellRatios) > 0 {
			sb.WriteString(fmt.Sprintf("BuySellRatios: %s\n\n", formatFloatSlice(data.IntradaySeries.BuySellRatios)))
		}
	}

	// 15分钟序列
	if data.Series15m != nil {
		sb.WriteString("15-minute series (oldest → latest):\n\n")

		if len(data.Series15m.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.Series15m.MidPrices)))
		}

		if len(data.Series15m.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(data.Series15m.EMA20Values)))
		}

		if len(data.Series15m.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.Series15m.MACDValues)))
		}

		if len(data.Series15m.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(data.Series15m.RSI7Values)))
		}

		if len(data.Series15m.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.Series15m.RSI14Values)))
		}

		if len(data.Series15m.Volumes) > 0 {
			sb.WriteString(fmt.Sprintf("Volumes: %s\n\n", formatFloatSlice(data.Series15m.Volumes)))
		}

		if len(data.Series15m.BuySellRatios) > 0 {
			sb.WriteString(fmt.Sprintf("BuySellRatios: %s\n\n", formatFloatSlice(data.Series15m.BuySellRatios)))
		}
	}

	// 1小时序列
	if data.Series1h != nil {
		sb.WriteString("1-hour series (oldest → latest):\n\n")

		if len(data.Series1h.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.Series1h.MidPrices)))
		}

		if len(data.Series1h.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(data.Series1h.EMA20Values)))
		}

		if len(data.Series1h.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.Series1h.MACDValues)))
		}

		if len(data.Series1h.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(data.Series1h.RSI7Values)))
		}

		if len(data.Series1h.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.Series1h.RSI14Values)))
		}

		if len(data.Series1h.Volumes) > 0 {
			sb.WriteString(fmt.Sprintf("Volumes: %s\n\n", formatFloatSlice(data.Series1h.Volumes)))
		}

		if len(data.Series1h.BuySellRatios) > 0 {
			sb.WriteString(fmt.Sprintf("BuySellRatios: %s\n\n", formatFloatSlice(data.Series1h.BuySellRatios)))
		}
	}

	// 4小时序列
	if data.LongerTermContext != nil {
		sb.WriteString("4-hour series (oldest → latest):\n\n")

		sb.WriteString(fmt.Sprintf("20‑Period EMA: %.3f vs. 50‑Period EMA: %.3f\n\n",
			data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))

		sb.WriteString(fmt.Sprintf("3‑Period ATR: %.3f vs. 14‑Period ATR: %.3f\n\n",
			data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))

		sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
			data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))

		if len(data.LongerTermContext.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.LongerTermContext.MidPrices)))
		}

		if len(data.LongerTermContext.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(data.LongerTermContext.EMA20Values)))
		}

		if len(data.LongerTermContext.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
		}

		if len(data.LongerTermContext.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI7Values)))
		}

		if len(data.LongerTermContext.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
		}

		if len(data.LongerTermContext.Volumes) > 0 {
			sb.WriteString(fmt.Sprintf("Volumes: %s\n\n", formatFloatSlice(data.LongerTermContext.Volumes)))
		}

		if len(data.LongerTermContext.BuySellRatios) > 0 {
			sb.WriteString(fmt.Sprintf("BuySellRatios: %s\n\n", formatFloatSlice(data.LongerTermContext.BuySellRatios)))
		}
	}

	return sb.String()
}

// formatFloatSlice 格式化float64切片为字符串
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%.3f", v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// Normalize 标准化symbol,确保是USDT交易对
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// parseFloat 解析float值
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}
