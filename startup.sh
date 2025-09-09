#!/bin/bash

# ComBot Server 启动脚本
# 该脚本会编译项目并在项目根目录运行服务器

set -e  # 遇到错误立即退出

# 获取脚本所在目录（项目根目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🚀 ComBot Server 启动脚本"
echo "📂 项目目录: $SCRIPT_DIR"

# 检查是否存在配置文件
if [ ! -f "config.yaml" ] && [ ! -f ".config.yaml" ]; then
    echo "❌ 错误: 未找到配置文件 (config.yaml 或 .config.yaml)"
    echo "💡 请确保在项目根目录下有配置文件"
    exit 1
fi

# 显示找到的配置文件
if [ -f ".config.yaml" ]; then
    echo "✅ 找到配置文件: .config.yaml"
elif [ -f "config.yaml" ]; then
    echo "✅ 找到配置文件: config.yaml"
fi

# 编译项目
echo "🔨 正在编译项目..."
make build

# 检查编译是否成功
if [ ! -f "./combot-server" ]; then
    echo "❌ 编译失败: 未找到可执行文件"
    exit 1
fi

echo "✅ 编译成功"

# 运行服务器
echo "🌟 启动 ComBot Server..."
echo "📍 工作目录: $(pwd)"
echo ""
echo "按 Ctrl+C 停止服务器"
echo "=========================="

# 运行服务器，传递所有命令行参数
./combot-server "$@"
