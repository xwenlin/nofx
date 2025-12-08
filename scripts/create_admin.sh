#!/bin/bash

# Mars AI交易系统 - 创建管理员账号脚本
# 用于在注册功能关闭时创建管理员账号

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
echo -e "${PURPLE}║                        👤 创建管理员账号工具                          ║${NC}"
echo -e "${PURPLE}╚════════════════════════════════════════════════════════════════════════╝${NC}"
echo

# 进入项目根目录
cd "$PROJECT_ROOT"

# 检查数据库文件是否存在
DB_PATH="config.db"
if [ ! -f "$DB_PATH" ]; then
    echo -e "${YELLOW}⚠️  数据库文件不存在: $DB_PATH${NC}"
    read -p "是否继续创建数据库? [y/N]: " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}ℹ️  操作已取消${NC}"
        exit 0
    fi
fi

# 检查 Go 程序是否存在
CREATE_ADMIN_GO="$SCRIPT_DIR/create_admin/create_admin.go"
if [ ! -f "$CREATE_ADMIN_GO" ]; then
    echo -e "${RED}❌ 错误: 找不到 create_admin.go 文件${NC}"
    echo -e "   预期路径: $CREATE_ADMIN_GO"
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
echo -e "${CYAN}📝 请输入管理员信息:${NC}"

# 邮箱输入
read -p "管理员邮箱 [默认: admin@example.com]: " EMAIL
EMAIL=${EMAIL:-admin@example.com}

# 验证邮箱格式（简单验证）
if [[ ! "$EMAIL" =~ ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$ ]]; then
    echo -e "${YELLOW}⚠️  警告: 邮箱格式可能不正确，但将继续创建${NC}"
fi

# 密码输入（隐藏输入）
echo -n "管理员密码: "
read -s PASSWORD
echo

if [ -z "$PASSWORD" ]; then
    echo -e "${RED}❌ 错误: 密码不能为空${NC}"
    exit 1
fi

# 确认密码
echo -n "确认密码: "
read -s PASSWORD_CONFIRM
echo

if [ "$PASSWORD" != "$PASSWORD_CONFIRM" ]; then
    echo -e "${RED}❌ 错误: 两次输入的密码不一致${NC}"
    exit 1
fi

# 检查密码长度
if [ ${#PASSWORD} -lt 8 ]; then
    echo -e "${YELLOW}⚠️  警告: 密码长度少于8位，建议使用更强的密码${NC}"
    read -p "是否继续? [y/N]: " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}ℹ️  操作已取消${NC}"
        exit 0
    fi
fi

# 数据库路径（可选）
read -p "数据库路径 [默认: config.db]: " DB_INPUT
DB_PATH=${DB_INPUT:-config.db}

# 显示配置摘要
echo
echo -e "${BLUE}📋 配置摘要:${NC}"
echo -e "  邮箱: ${YELLOW}$EMAIL${NC}"
echo -e "  密码: ${YELLOW}${PASSWORD:0:1}****${NC} (已隐藏)"
echo -e "  数据库: ${YELLOW}$DB_PATH${NC}"
echo

read -p "确认创建管理员账号? [Y/n]: " -n 1 -r
echo
if [[ $REPLY =~ ^[Nn]$ ]]; then
    echo -e "${BLUE}ℹ️  操作已取消${NC}"
    exit 0
fi

# 编译并运行 Go 程序
echo
echo -e "${CYAN}🚀 正在创建管理员账号...${NC}"

# 使用 go run 直接运行（不需要先编译）
if go run "$CREATE_ADMIN_GO" -email "$EMAIL" -password "$PASSWORD" -db "$DB_PATH"; then
    echo
    echo -e "${GREEN}🎉 管理员账号创建成功！${NC}"
    echo
    echo -e "${BLUE}📋 使用指南:${NC}"
    echo -e "  1. 使用邮箱和密码登录系统"
    echo -e "  2. 登录后设置 Google Authenticator 进行二次验证"
    echo -e "  3. 建议立即修改默认密码"
    echo
else
    echo
    echo -e "${RED}❌ 创建管理员账号失败${NC}"
    exit 1
fi

