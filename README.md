# GateShift

![License](https://img.shields.io/github/license/ourines/GateShift)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/ourines/GateShift)

GateShift是一个专为OpenWrt旁路由设计的网关切换工具，让用户能够在默认网关和代理网关之间无缝切换网络流量路径，特别适合需要在普通上网和科学上网之间灵活切换的场景。

[English](./README_EN.md) | 简体中文

## 项目背景

在使用OpenWrt作为旁路由的网络环境中，用户常常需要在默认网关和OpenWrt旁路由网关之间切换，以满足不同的上网需求。这一过程通常需要手动修改网络设置，操作繁琐且容易出错。GateShift应运而生，通过简单的命令行工具，实现了网关的一键切换，大大简化了网络管理流程。

## 核心功能

- **旁路由网关切换**：一键在主路由网关和OpenWrt旁路由网关间切换
- **跨平台支持**：兼容macOS、Linux和Windows系统
- **配置持久化**：自动记住您的网关配置信息
- **权限管理**：内置sudo会话管理，避免重复输入密码
- **实时状态检测**：提供当前网络状态和互联网连接检查
- **系统级安装**：支持全局安装，随时随地调用

## 安装方法

### 方法一：使用 Go 安装（推荐）

如果您已安装 Go 1.18 或更高版本：

```bash
go install github.com/ourines/GateShift@latest
```

### 方法二：使用快速安装脚本

```bash
curl -sSL https://raw.githubusercontent.com/ourines/GateShift/main/install.sh | bash
```

### 方法三：手动下载安装

从 [Releases 页面](https://github.com/ourines/GateShift/releases) 下载适合您平台的最新预构建二进制文件。

### 方法四：从源码构建

1. 克隆此仓库
2. 构建应用程序：

```bash
make build
```

3. 安装到本地bin目录：

```bash
make install
```

卸载：

```bash
gateshift uninstall
```

## 使用方法

```bash
# 切换到旁路由网关（例如OpenWrt）
gateshift proxy

# 切换回默认网关（例如主路由）
gateshift default

# 显示当前网络状态
gateshift status

# 配置网关
gateshift config set-proxy 192.168.31.100  # 设置OpenWrt旁路由IP
gateshift config set-default 192.168.31.1  # 设置主路由IP
gateshift config show

# 系统级安装
gateshift install

# 从系统卸载
gateshift uninstall
```

## 配置文件

应用程序将配置存储在`~/.gateshift/config.yaml`中。您可以手动编辑此文件或使用`config`命令。

默认配置：

```yaml
proxy_gateway: 192.168.31.100  # OpenWrt旁路由IP
default_gateway: 192.168.31.1  # 主路由IP
```

## 典型应用场景

- **日常/科学上网切换**：在普通上网和科学上网之间快速切换
- **多网络环境**：在不同网络代理策略之间灵活转换
- **特定应用网络**：为需要特定网络环境的应用提供便捷的网关管理
- **网络测试**：测试不同网络路径的连接质量和速度
- **家庭/办公网络管理**：管理多种网络方案

## 为不同平台构建

构建所有支持的平台：

```bash
make build-all
```

这将创建以下平台的二进制文件：
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

二进制文件将放置在`bin/`目录中。

## 协议

此项目使用MIT协议 - 详见[LICENSE](LICENSE)文件。 