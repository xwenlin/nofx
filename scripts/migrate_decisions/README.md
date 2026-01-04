# 决策日志数据迁移工具

从 `content` 字段中提取 `decisions` 数据并迁移到新字段，以优化查询性能。

## 功能

- 自动检查并添加 `decisions` 字段（如果不存在）
- 从 `content` 字段中提取 `decisions` 数据
- 将提取的数据写入新的 `decisions` 字段
- 保留 `content` 字段作为备份（不会删除）
- 显示迁移进度和统计信息

## 使用方法

### 方法一：使用 Shell 脚本（推荐）

```bash
# 从项目根目录运行
./scripts/migrate_decisions.sh

# 指定数据库路径
./scripts/migrate_decisions.sh --db config.db

# 查看帮助
./scripts/migrate_decisions.sh --help
```

### 方法二：直接运行 Go 程序

```bash
# 从项目根目录运行
go run ./scripts/migrate_decisions/main.go config.db

# 使用相对路径
go run ./scripts/migrate_decisions/main.go ../config.db
```

## 迁移说明

### 为什么需要迁移？

在优化之前，决策日志的所有数据都存储在 `content` 字段中（完整的 JSON）。这导致：

1. **查询性能差**：列表查询需要加载所有字段，包括长文本字段（`system_prompt`, `input_prompt`, `cot_trace`）
2. **数据传输量大**：每次查询都要传输大量不必要的数据
3. **内存占用高**：前端需要加载大量无用数据

### 优化后的结构

- `decisions` 字段：单独存储决策动作列表（JSON）
- 列表查询：不加载长文本字段，只加载必要数据
- 按需加载：点击详情时，才加载完整的长文本字段

### 迁移过程

1. **检查字段**：如果 `decisions` 字段不存在，自动添加
2. **提取数据**：从 `content` 字段中解析 JSON，提取 `decisions` 数组
3. **写入新字段**：将提取的 `decisions` 序列化为 JSON 并写入新字段
4. **保留备份**：`content` 字段保留不变，作为数据备份

### 迁移后的影响

- ✅ **新数据**：新保存的决策日志将直接写入 `decisions` 字段
- ✅ **旧数据**：已迁移的数据可以从 `decisions` 字段快速读取
- ✅ **兼容性**：如果 `decisions` 字段为空，系统会自动回退到 `content` 字段
- ✅ **安全性**：`content` 字段保留，不会丢失数据

## 注意事项

1. **备份数据库**：迁移前建议备份数据库文件
2. **迁移时间**：迁移时间取决于数据量，每 100 条记录会显示进度
3. **失败处理**：如果某条记录迁移失败，会记录错误但继续处理其他记录
4. **content 字段**：迁移后不会删除 `content` 字段，如需删除请手动操作

## 故障排除

### 问题：迁移失败，提示字段已存在

**原因**：`decisions` 字段已经存在，可能是之前已经运行过迁移。

**解决**：这是正常情况，工具会自动跳过添加字段的步骤，直接进行数据迁移。

### 问题：某些记录迁移失败

**原因**：可能是 `content` 字段中的数据格式不正确或已损坏。

**解决**：
- 检查日志中的错误信息
- 失败的记录会保留在 `content` 字段中
- 系统会自动回退到使用 `content` 字段

### 问题：迁移后查询变慢

**原因**：可能是数据库索引问题。

**解决**：
- 检查数据库是否有适当的索引
- 考虑使用 `VACUUM` 命令优化数据库

## 技术细节

### 数据库变更

```sql
-- 添加新字段（如果不存在）
ALTER TABLE decisions ADD COLUMN decisions TEXT DEFAULT '';
```

### 数据迁移逻辑

1. 查询所有需要迁移的记录：
   ```sql
   SELECT id, content 
   FROM decisions 
   WHERE content != '' AND (decisions IS NULL OR decisions = '')
   ```

2. 解析 `content` 字段中的 JSON：
   ```go
   var fullRecord logger.DecisionRecord
   json.Unmarshal([]byte(content), &fullRecord)
   ```

3. 提取并序列化 `decisions`：
   ```go
   decisionsJSON, _ := json.Marshal(fullRecord.Decisions)
   ```

4. 更新数据库：
   ```sql
   UPDATE decisions SET decisions = ? WHERE id = ?
   ```

## 相关文档

- [数据库优化说明](../../docs/database_optimization.md)
- [API 接口文档](../../docs/api.md)

