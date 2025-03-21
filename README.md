# GateShift

![License](https://img.shields.io/github/license/ourines/GateShift)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/ourines/GateShift)

GateShift是一个专为OpenWrt旁路由设计的网关切换工具，让用户能够在默认网关和代理网关之间无缝切换网络流量路径，特别适合需要在普通上网和科学上网之间灵活切换的场景。

[English](./README_EN.md) | 简体中文

## 项目背景

在使用OpenWrt作为旁路由的网络环境中，用户常常需要在默认网关和OpenWrt旁路由网关之间切换，以满足不同的上网需求。这一过程通常需要手动修改网络设置，操作繁琐且容易出错。GateShift应运而生，通过简单的命令行工具，实现了网关的一键切换，大大简化了网络管理流程。

## 核心功能

- **旁路由网关切换**：一键在主路由网关和OpenWrt旁路由网关间切换
- **DNS防泄漏保护**：内置DNS代理功能，确保所有DNS请求都通过代理网关
- **功能分离设计**：网关切换与DNS服务完全独立，按需使用
- **跨平台支持**：兼容macOS、Linux和Windows系统
- **配置持久化**：自动记住您的网关配置信息
- **权限管理**：内置sudo会话管理，避免重复输入密码
- **实时状态检测**：提供当前网络状态和互联网连接检查
- **系统级安装**：支持全局安装，随时随地调用

### DNS 泄露解决

使用代理网络时，DNS请求有时会绕过代理直接连接到您的ISP的DNS服务器，从而暴露您的实际浏览活动。GateShift的DNS代理功能通过将所有DNS查询路由通过代理网络来防止这种情况。

#### 之前

![](./images/dns-1.png)

#### 之后

![](./images/dns-2.png)

## 安装方法

### 方法一：使用 Go 安装（推荐）

如果您已安装 Go 1.18 或更高版本：

```bash
go install github.com/ourines/GateShift/cmd/gateshift@latest
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
# 切换到旁路由（如 OpenWrt）网关
gateshift proxy

# 切换回默认网关（如主路由）
gateshift default

# 显示当前网络状态
gateshift status

# 配置网关
gateshift config set-proxy 192.168.31.100  # 设置旁路由 IP
gateshift config set-default 192.168.31.1  # 设置主路由 IP
gateshift config show

# 全局安装
gateshift install

# 从系统中卸载
gateshift uninstall

# DNS 功能（独立于网关切换）
gateshift dns start                        # 启动 DNS 服务用于防止 DNS 泄露
gateshift dns add-server 1.1.1.1           # 添加上游DNS服务器
gateshift dns remove-server 8.8.8.8        # 移除指定的上游DNS服务器
gateshift dns list-servers                 # 列出所有配置的上游DNS服务器
gateshift dns show                         # 显示 DNS 配置
gateshift dns start -f                     # 在前台启动 DNS 服务
gateshift dns restart                      # 重启 DNS 服务
gateshift dns stop                         # 停止运行中的 DNS 服务
gateshift dns logs                         # 查看 DNS 日志
gateshift dns logs -f                      # 实时查看 DNS 日志
gateshift dns logs -n 100                  # 查看最近 100 行 DNS 日志
gateshift dns logs -F "google.com"         # 过滤包含 google.com 的日志
```

## 配置文件

应用程序将配置存储在`~/.gateshift/config.yaml`中。您可以手动编辑此文件或使用`config`命令。

默认配置：

```yaml
proxy_gateway: 192.168.31.100  # OpenWrt旁路由IP
default_gateway: 192.168.31.1  # 主路由IP
dns:
  listen_addr: 127.0.0.1       # DNS监听地址
  listen_port: 53              # DNS监听端口
  upstream_dns:                # 上游DNS服务器列表
    - 1.1.1.1:53
    - 8.8.8.8:53
```

## 网关切换与DNS服务

GateShift将网关切换和DNS服务设计为完全独立的功能，用户可以根据需求选择使用：

1. **仅切换网关**：使用 `gateshift proxy` 或 `gateshift default` 命令
2. **仅使用DNS服务**：使用 `gateshift dns start` 系列命令
3. **组合使用**：先切换网关，再手动启动DNS服务

这种设计提供了更大的灵活性，让用户可以根据自己的需求自由组合功能。

## DNS功能详解

GateShift内置了强大的DNS代理功能，主要用于防止DNS泄漏和提供更可靠的DNS解析服务。

### DNS服务运行模式

GateShift提供了两种DNS服务运行模式，以适应不同场景的需求：

1. **后台模式**：`gateshift dns start` - 启动DNS服务后退出终端
2. **前台模式**：`gateshift dns start -f` - 启动DNS服务并在前台持续运行（按Ctrl+C停止）

注意：使用端口53（默认DNS端口）通常需要管理员/root权限，因为：
- 绑定特权端口（小于1024的端口）需要特殊权限
- 修改系统DNS设置需要特殊权限

### DNS配置管理

```bash
# 配置上游DNS服务器
gateshift dns add-server 1.1.1.1           # 添加单个上游DNS服务器（自动添加":53"端口号）
gateshift dns remove-server 8.8.8.8        # 移除指定的上游DNS服务器
gateshift dns list-servers                 # 列出所有配置的上游DNS服务器

# 查看当前DNS配置和运行状态
gateshift dns show
```

### DNS服务管理

```bash
# 启动DNS服务
sudo gateshift dns start          # 启动后台服务（需要sudo权限）
sudo gateshift dns start -f       # 前台运行（按Ctrl+C停止）

# 停止DNS服务
sudo gateshift dns stop

# 重启DNS服务（应用新配置）
sudo gateshift dns restart
```

### DNS日志查看与分析

GateShift提供了强大的DNS日志查看功能，帮助您监控DNS活动：

```bash
# 基本日志查看（默认显示最后50行）
gateshift dns logs

# 实时查看日志（类似tail -f）
gateshift dns logs -f

# 自定义显示行数
gateshift dns logs -n 200         # 查看最后200行

# 使用关键词过滤日志（不区分大小写）
gateshift dns logs -F "google"    # 查看包含"google"的日志
gateshift dns logs -F "error"     # 只查看错误信息
gateshift dns logs -F "query"     # 只查看查询请求

# 组合使用
gateshift dns logs -F "google" -n 10 -f  # 实时查看最新10行包含"google"的日志
```

### 日志和配置文件位置

GateShift将所有数据存储在用户主目录下的 `.gateshift` 文件夹中：

```
~/.gateshift/               # 主配置目录
├── config.yaml             # 配置文件
└── logs/                   # 日志目录
    └── gateshift-dns.log   # DNS服务日志文件
```

## 典型应用场景

- **日常/科学上网切换**：在普通上网和科学上网之间快速切换
- **DNS防泄漏**：确保所有DNS请求通过代理网关，防止泄露真实IP
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