# 修复日志

本文档记录了对代码的修复和补丁，用于在合并新版本时参考。

---

## 2025-11-02 - 修复前端编辑模型时提示"模型不存在"的问题

### 问题描述
在前端编辑交易员时，如果选择的模型被禁用或没有配置 API Key，系统会提示"模型不存在"，导致无法保存编辑。

### 根本原因
在 `handleSaveEditTrader` 函数中，代码只从 `enabledModels`（已启用且有 API Key 的模型列表）中查找模型，导致被禁用的模型无法被找到，从而触发错误提示。

### 修改文件
- `web/src/components/AITradersPage.tsx`

### 具体修改

#### 1. 修改 `handleSaveEditTrader` 函数（第 164-180 行）

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
    const model = allModels?.find(m => m.id === data.ai_model_id);
    const exchange = allExchanges?.find(e => e.id === data.exchange_id);

    if (!model) {
      alert(t('modelConfigNotExist', language));
      return;
    }

    if (!exchange) {
      alert(t('exchangeConfigNotExist', language));
      return;
    }
```

#### 2. 修改编辑模态框的模型和交易所列表（第 762-777 行）

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
1. **编辑验证逻辑**：在保存编辑时，从 `allModels` 和 `allExchanges` 中查找模型和交易所，而不是从过滤后的 `enabledModels` 和 `enabledExchanges` 中查找。这样允许用户编辑使用被禁用配置的交易员。

2. **编辑模态框选项**：编辑模式下，模态框使用 `allModels` 和 `allExchanges` 作为可用选项，确保所有已配置的模型和交易所都可以选择，即使它们当前被禁用。

3. **保持创建验证不变**：创建新交易员时仍然使用 `enabledModels` 和 `enabledExchanges`，确保只有启用且配置完整的模型和交易所才能用于新交易员。

### 影响范围
- ✅ 修复了编辑交易员时无法保存使用被禁用模型的配置的问题
- ✅ 允许用户在编辑时选择任何已配置的模型和交易所
- ✅ 不影响创建新交易员的验证逻辑

### 测试建议
1. 创建一个使用某个模型的交易员
2. 禁用该模型
3. 尝试编辑该交易员，应该能够正常保存
4. 验证创建新交易员时仍然只显示已启用的模型和交易所

---

## 如何使用本日志

在合并新版本时：
1. 检查本日志中记录的修改是否在新版本中已存在
2. 如果已存在，标记为已完成
3. 如果不存在，需要重新应用这些修改
4. 注意新版本中相关代码的结构变化，可能需要调整修改方式

