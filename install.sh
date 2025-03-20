#!/bin/bash

# 检测系统和架构
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# 将 x86_64 映射到 amd64
if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" = "aarch64" ]; then
    ARCH="arm64"
fi

# 最新版本
VERSION=$(curl -s https://api.github.com/repos/ourines/GateShift/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

# 下载地址
BINARY="gateshift-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
    BINARY="${BINARY}.exe"
fi
URL="https://github.com/ourines/GateShift/releases/download/${VERSION}/${BINARY}"

# 下载并安装
echo "Downloading GateShift ${VERSION} for ${OS}-${ARCH}..."
curl -L "${URL}" -o gateshift
chmod +x gateshift

# 移动到 PATH 目录
sudo mv gateshift /usr/local/bin/

echo "GateShift has been installed to /usr/local/bin/gateshift"
echo "You can now run 'gateshift --help' to get started" 