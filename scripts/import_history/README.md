# 历史数据导入工具

从日志文件（JSON）中导入历史决策日志和交易记录到数据库。

## 功能

- 扫描 `decision_logs/` 目录下的所有 JSON 日志文件
- 导入决策日志到 `decisions` 表
- 从决策记录中提取交易记录（开仓/平仓）并导入到 `trades` 表
- 自动去重（避免重复导入）
- 支持试运行模式（dry-run）

## 使用方法

### 方法一：使用 Shell 脚本（推荐）

```bash
# 从项目根目录运行
./scripts/import_history.sh

# 指定参数
./scripts/import_history.sh \
  --log-dir=decision_logs \
  --db=trading.db \
  --trader-id=trader_001 \
  --verbose

# 试运行模式（不实际写入数据库）
./scripts/import_history.sh --dry-run --verbose

# 查看帮助
./scripts/import_history.sh --help
```

### 方法二：直接运行 Go 程序

```bash
cd scripts/import_history
go run main.go
```

### 指定参数

```bash
go run main.go \
  -log-dir=decision_logs \
  -db=../config.db \
  -trader-id=trader_001 \
  -verbose
```

### 试运行模式（不实际写入数据库）

```bash
go run main.go -dry-run -verbose
```

## 参数说明

- `-log-dir`: 决策日志目录路径（默认: `decision_logs`）
- `-db`: 数据库文件路径（默认: `config.db`）
- `-trader-id`: 交易员ID（如果为空，将从目录结构推断：`decision_logs/{trader_id}/`）
- `-dry-run`: 试运行模式，仅显示将要导入的数据，不实际写入数据库
- `-verbose`: 显示详细信息

## 注意事项

1. **去重机制**：脚本会自动检查数据库中是否已存在相同的记录（根据 trader_id, cycle_number, timestamp），避免重复导入。

2. **交易记录匹配**：
   - 开仓记录：根据 symbol, side, open_time 匹配
   - 平仓记录：匹配最近的未平仓记录

3. **部分平仓**：部分平仓（partial_close）暂不支持自动导入，因为需要更新交易记录的数量字段。

4. **时间顺序**：建议按时间顺序处理日志文件，以确保开仓和平仓能正确匹配。

5. **备份**：导入前建议备份数据库文件。

## 示例输出

```
╔════════════════════════════════════════════════════════════════════════╗
║                   历史数据导入工具                                    ║
║              从日志文件导入决策日志和交易记录到数据库                ║
╚════════════════════════════════════════════════════════════════════════╝

✓ 数据库已打开: config.db

📂 扫描日志目录: /path/to/decision_logs

📄 处理文件: decision_logs/trader_001/decision_20240101_120000_cycle1.json
  ✓ 决策日志已导入 (ID: 1, Cycle #1)
  ✓ 开仓记录已导入 (ID: 1, BTCUSDT long)
  ✓ 平仓记录已更新 (BTCUSDT long, PnL: 12.50 USDT)

╔════════════════════════════════════════════════════════════════════════╗
║                           导入完成                                    ║
╚════════════════════════════════════════════════════════════════════════╝
📊 统计信息:
  - 处理的文件数: 10
  - 导入的决策日志: 10
  - 导入的交易记录: 15
  - 错误数: 0
```

