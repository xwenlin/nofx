#!/bin/bash

# Mars AI交易系统 - 决策日志数据迁移工具
# 从 content 字段中提取 decisions 并迁移到新字段

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "${PURPLE}╔════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${PURPLE}║                          Mars AI交易系统                              ║${NC}"
echo -e "${PURPLE}║                  🔄 决策日志数据迁移工具                              ║${NC}"
echo -e "${PURPLE}╚════════════════════════════════════════════════════════════════════════╝${NC}"
echo

# 进入项目根目录
cd "$PROJECT_ROOT"

# 默认参数
DB_PATH="${DB_PATH:-config.db}"

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --db|-d)
            DB_PATH="$2"
            shift 2
            ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo
            echo "选项:"
            echo "  --db, -d PATH    数据库文件路径 (默认: config.db)"
            echo "  --help, -h       显示此帮助信息"
            echo
            echo "说明:"
            echo "  此工具将从 decisions 表的 content 字段中提取 decisions 数据，"
            echo "  并迁移到新的 decisions 字段中，以优化查询性能。"
            echo
            echo "示例:"
            echo "  $0                    # 使用默认数据库 config.db"
            echo "  $0 --db config.db     # 指定数据库路径"
            echo "  $0 -d /path/to/db.db  # 使用绝对路径"
            exit 0
            ;;
        *)
            echo -e "${RED}❌ 未知参数: $1${NC}"
            echo "使用 --help 查看帮助信息"
            exit 1
            ;;
    esac
done

# 检查数据库文件是否存在
if [ ! -f "$DB_PATH" ]; then
    echo -e "${RED}❌ 错误: 数据库文件不存在: $DB_PATH${NC}"
    echo -e "   请确保数据库文件存在，或使用 --db 参数指定数据库路径"
    exit 1
fi

# 检查 Go 程序是否存在
MIGRATE_GO="$SCRIPT_DIR/migrate_decisions/main.go"
if [ ! -f "$MIGRATE_GO" ]; then
    echo -e "${RED}❌ 错误: 找不到 migrate_decisions/main.go 文件${NC}"
    echo -e "   预期路径: $MIGRATE_GO"
    exit 1
fi

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ 错误: 系统中未安装 Go${NC}"
    echo -e "请安装 Go: https://golang.org/dl/"
    exit 1
fi

echo -e "${GREEN}✓ Go 版本: $(go version)${NC}"
echo -e "${CYAN}💾 数据库: $DB_PATH${NC}"
echo

# 显示迁移说明
echo -e "${BLUE}📋 迁移说明:${NC}"
echo -e "  1. 检查并添加 decisions 字段（如果不存在）"
echo -e "  2. 从 content 字段中提取 decisions 数据"
echo -e "  3. 将提取的数据写入新的 decisions 字段"
echo -e "  4. 保留 content 字段作为备份（不会删除）"
echo

# 确认执行
read -p "确认开始迁移? [Y/n]: " -n 1 -r
echo
if [[ $REPLY =~ ^[Nn]$ ]]; then
    echo -e "${BLUE}ℹ️  操作已取消${NC}"
    exit 0
fi

# 运行迁移工具
echo
echo -e "${CYAN}🚀 正在执行数据迁移...${NC}"
echo

cd "$PROJECT_ROOT"
if go run "$MIGRATE_GO" "$DB_PATH"; then
    echo
    echo -e "${GREEN}✅ 数据迁移完成！${NC}"
    echo
    echo -e "${BLUE}📋 迁移结果:${NC}"
    echo -e "  • decisions 字段已添加（如果之前不存在）"
    echo -e "  • 数据已从 content 字段迁移到 decisions 字段"
    echo -e "  • content 字段已保留作为备份"
    echo
    echo -e "${YELLOW}💡 提示:${NC}"
    echo -e "  • 迁移后，新的决策日志将直接写入 decisions 字段"
    echo -e "  • 列表查询将不再加载长文本字段，性能显著提升"
    echo -e "  • 如需删除 content 字段，请使用数据库管理工具手动操作"
    echo
else
    echo
    echo -e "${RED}❌ 数据迁移失败${NC}"
    exit 1
fi

