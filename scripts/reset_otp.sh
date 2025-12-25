#!/bin/bash

# Mars AI交易系统 - 重置OTP密钥脚本
# 用于重置用户的OTP密钥（例如：换手机后无法验证）

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
echo -e "${PURPLE}║                      🔄 重置OTP密钥工具                               ║${NC}"
echo -e "${PURPLE}╚════════════════════════════════════════════════════════════════════════╝${NC}"
echo

# 进入项目根目录
cd "$PROJECT_ROOT"

# 检查数据库文件是否存在
DB_PATH="config.db"
if [ ! -f "$DB_PATH" ]; then
    echo -e "${RED}❌ 错误: 数据库文件不存在: $DB_PATH${NC}"
    echo -e "   请确保数据库文件存在，或使用 -db 参数指定数据库路径"
    exit 1
fi

# 检查 Go 程序是否存在
RESET_OTP_GO="$SCRIPT_DIR/reset_otp/reset_otp.go"
if [ ! -f "$RESET_OTP_GO" ]; then
    echo -e "${RED}❌ 错误: 找不到 reset_otp.go 文件${NC}"
    echo -e "   预期路径: $RESET_OTP_GO"
    exit 1
fi

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ 错误: 系统中未安装 Go${NC}"
    echo -e "请安装 Go: https://golang.org/dl/"
    exit 1
fi

echo -e "${GREEN}✓ Go 版本: $(go version)${NC}"

# 收集用户输入
echo
echo -e "${CYAN}📝 请输入要重置OTP的用户信息:${NC}"

# 用户ID输入（可选）
read -p "用户ID (可选，留空则使用邮箱查找): " USER_ID

# 邮箱输入（如果未提供用户ID）
if [ -z "$USER_ID" ]; then
    read -p "用户邮箱: " EMAIL
    if [ -z "$EMAIL" ]; then
        echo -e "${RED}❌ 错误: 必须提供用户ID或邮箱${NC}"
        exit 1
    fi
else
    EMAIL=""
fi

# 数据库路径（可选）
read -p "数据库路径 [默认: config.db]: " DB_INPUT
DB_PATH=${DB_INPUT:-config.db}

# 显示配置摘要
echo
echo -e "${BLUE}📋 配置摘要:${NC}"
if [ -n "$USER_ID" ]; then
    echo -e "  用户ID: ${YELLOW}$USER_ID${NC}"
else
    echo -e "  邮箱: ${YELLOW}$EMAIL${NC}"
fi
echo -e "  数据库: ${YELLOW}$DB_PATH${NC}"
echo

read -p "确认重置该用户的OTP密钥? [Y/n]: " -n 1 -r
echo
if [[ $REPLY =~ ^[Nn]$ ]]; then
    echo -e "${BLUE}ℹ️  操作已取消${NC}"
    exit 0
fi

# 编译并运行 Go 程序
echo
echo -e "${CYAN}🚀 正在重置OTP密钥...${NC}"

# 构建命令参数
ARGS="-db \"$DB_PATH\""
if [ -n "$USER_ID" ]; then
    ARGS="$ARGS -user-id \"$USER_ID\""
else
    ARGS="$ARGS -email \"$EMAIL\""
fi

# 使用 go run 直接运行（不需要先编译）
if eval "go run \"$RESET_OTP_GO\" $ARGS"; then
    echo
    echo -e "${GREEN}🎉 OTP密钥重置成功！${NC}"
    echo
    echo -e "${BLUE}📋 下一步操作:${NC}"
    echo -e "  1. 使用邮箱和密码登录系统"
    echo -e "  2. 登录后系统会自动显示OTP设置页面"
    echo -e "  3. 使用Google Authenticator扫描新的二维码或手动输入新密钥"
    echo -e "  4. 完成OTP设置后即可正常使用"
    echo
    echo -e "${YELLOW}⚠️  注意: 旧手机的OTP验证码将不再有效${NC}"
    echo
else
    echo
    echo -e "${RED}❌ 重置OTP密钥失败${NC}"
    exit 1
fi

