#!/bin/bash

# Mars AI交易系统 - 历史数据导入工具
# 从日志文件导入决策日志和交易记录到数据库

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
echo -e "${PURPLE}║                    📥 历史数据导入工具                                ║${NC}"
echo -e "${PURPLE}╚════════════════════════════════════════════════════════════════════════╝${NC}"
echo

# 进入项目根目录
cd "$PROJECT_ROOT"

# 默认参数
LOG_DIR="${LOG_DIR:-decision_logs}"
DB_PATH="${DB_PATH:-config.db}"
TRADER_ID="${TRADER_ID:-}"
DRY_RUN="${DRY_RUN:-false}"
VERBOSE="${VERBOSE:-false}"

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --log-dir)
            LOG_DIR="$2"
            shift 2
            ;;
        --db)
            DB_PATH="$2"
            shift 2
            ;;
        --trader-id)
            TRADER_ID="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN="true"
            shift
            ;;
        --verbose|-v)
            VERBOSE="true"
            shift
            ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo
            echo "选项:"
            echo "  --log-dir DIR     决策日志目录路径 (默认: decision_logs)"
            echo "  --db PATH         数据库文件路径 (默认: config.db)"
            echo "  --trader-id ID    交易员ID (如果为空，将从目录结构推断)"
            echo "  --dry-run         试运行模式，仅显示将要导入的数据，不实际写入数据库"
            echo "  --verbose, -v     显示详细信息"
            echo "  --help, -h        显示此帮助信息"
            echo
            echo "示例:"
            echo "  $0 --log-dir decision_logs --db config.db --trader-id trader_001 --verbose"
            echo "  $0 --dry-run --verbose  # 试运行模式"
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
    echo -e "${YELLOW}⚠️  数据库文件不存在: $DB_PATH${NC}"
    echo -e "${YELLOW}   将创建新的数据库文件${NC}"
    echo
fi

# 检查日志目录是否存在
if [ ! -d "$LOG_DIR" ]; then
    echo -e "${RED}❌ 日志目录不存在: $LOG_DIR${NC}"
    exit 1
fi

# 构建 Go 命令参数
GO_ARGS=(
    "-log-dir=$LOG_DIR"
    "-db=$DB_PATH"
)

if [ -n "$TRADER_ID" ]; then
    GO_ARGS+=("-trader-id=$TRADER_ID")
fi

if [ "$DRY_RUN" = "true" ]; then
    GO_ARGS+=("-dry-run")
fi

if [ "$VERBOSE" = "true" ]; then
    GO_ARGS+=("-verbose")
fi

# 运行导入工具
echo -e "${CYAN}📂 日志目录: $LOG_DIR${NC}"
echo -e "${CYAN}💾 数据库: $DB_PATH${NC}"
if [ -n "$TRADER_ID" ]; then
    echo -e "${CYAN}👤 交易员ID: $TRADER_ID${NC}"
fi
if [ "$DRY_RUN" = "true" ]; then
    echo -e "${YELLOW}⚠️  试运行模式（不会实际写入数据库）${NC}"
fi
echo

cd "$PROJECT_ROOT"
go run ./scripts/import_history "${GO_ARGS[@]}"

echo
echo -e "${GREEN}✅ 导入完成！${NC}"

