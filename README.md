# GateShift

![License](https://img.shields.io/github/license/ourines/GateShift)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/ourines/GateShift)
[![codecov](https://codecov.io/gh/ourines/GateShift/branch/main/graph/badge.svg)](https://codecov.io/gh/ourines/GateShift)

GateShift是一个专为OpenWrt旁路由设计的网关切换工具，让用户能够在默认网关和代理网关之间无缝切换网络流量路径，特别适合需要在普通上网和科学上网之间灵活切换的场景。

[English](./README_EN.md) | 简体中文

## 项目背景

在使用OpenWrt作为旁路由的网络环境中，用户常常需要在默认网关和OpenWrt旁路由网关之间切换，以满足不同的上网需求。这一过程通常需要手动修改网络设置，操作繁琐且容易出错。GateShift应运而生，通过简单的命令行工具，实现了网关的一键切换，大大简化了网络管理流程。

## 核心功能

- **旁路由网关切换**：一键在主路由网关和OpenWrt旁路由网关间切换
- **DNS防泄漏保护**：内置DNS代理功能，确保所有DNS请求都通过代理网关
- **守护进程模式**：支持以后台守护进程方式运行，提供持续稳定的服务
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

# DNS功能
gateshift dns enable                          # 启用DNS代理
gateshift dns disable                         # 禁用DNS代理
gateshift dns set-port 5353                   # 设置DNS监听端口
gateshift dns set-address 127.0.0.1           # 设置DNS监听地址
gateshift dns set-upstream 1.1.1.1 8.8.8.8 9.9.9.9  # 设置上游DNS服务器
gateshift dns show                            # 显示DNS配置
gateshift dns start                           # 启动DNS服务
gateshift dns start -f                        # 在前台启动DNS服务
gateshift dns restart                         # 重启DNS服务
gateshift dns stop                            # 停止正在运行的DNS服务
gateshift dns logs                            # 查看DNS日志
gateshift dns logs -f                         # 实时查看DNS日志
gateshift dns logs -n 100                     # 查看最近100行DNS日志
gateshift dns logs -F "baidu.com"             # 过滤包含baidu.com的日志

# 守护进程模式
gateshift daemon -d                           # 以守护进程方式运行
```

## 配置文件

应用程序将配置存储在`~/.gateshift/config.yaml`中。您可以手动编辑此文件或使用`config`命令。

默认配置：

```yaml
proxy_gateway: 192.168.31.100  # OpenWrt旁路由IP
default_gateway: 192.168.31.1  # 主路由IP
dns:
  enabled: false               # 是否启用DNS代理
  listen_addr: 127.0.0.1       # DNS监听地址
  listen_port: 53              # DNS监听端口
  upstream_dns:                # 上游DNS服务器列表
    - 1.1.1.1:53
    - 8.8.8.8:53
    - 9.9.9.9:53
```

## DNS功能详解

GateShift内置了强大的DNS代理功能，主要用于防止DNS泄漏和提供更可靠的DNS解析服务。

### DNS服务的三种运行模式

GateShift提供了三种DNS服务运行模式，以适应不同场景的需求：

1. **标准模式**：`gateshift dns start` - 启动DNS服务后退出
2. **前台模式**：`gateshift dns start -f` - 启动DNS服务并在前台持续运行
3. **守护进程模式**：`gateshift daemon -d` - 以守护进程方式在后台运行

### DNS配置管理

```bash
# 启用/禁用DNS代理（只修改配置，不启动/停止服务）
gateshift dns enable
gateshift dns disable

# 设置DNS监听地址和端口
gateshift dns set-address 127.0.0.1  # 默认监听本地
gateshift dns set-port 53            # 默认使用53端口

# 如果遇到53端口权限问题，可以设置为更高端口号
gateshift dns set-port 10053         # 使用非特权端口

# 配置上游DNS服务器（可设置多个）
gateshift dns set-upstream 1.1.1.1 8.8.8.8 9.9.9.9
# 系统会自动添加":53"端口号

# 查看当前DNS配置和运行状态
gateshift dns show
```

### DNS服务管理

```bash
# 启动DNS服务
sudo gateshift dns start          # 启动后台服务
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
gateshift dns logs -F "baidu" -n 10 -f  # 实时查看最新10行包含"baidu"的日志
```

## 典型应用场景

- **日常/科学上网切换**：在普通上网和科学上网之间快速切换
- **DNS防泄漏**：确保所有DNS请求通过代理网关，防止泄露真实IP
- **多网络环境**：在不同网络代理策略之间灵活转换
- **特定应用网络**：为需要特定网络环境的应用提供便捷的网关管理
- **网络测试**：测试不同网络路径的连接质量和速度
- **家庭/办公网络管理**：管理多种网络方案

## 运行模式

GateShift支持多种运行模式：

1. **标准模式**：`gateshift proxy` - 切换到代理网关后退出
2. **前台模式**：`gateshift proxy -k` - 切换到代理网关并在前台持续运行
3. **守护进程模式**：`gateshift dns daemon -d` - 以守护进程方式在后台运行DNS服务

## DNS日志查看

GateShift提供了便捷的DNS日志查看和过滤功能：

```bash
# 查看最近50行DNS日志（默认）
gateshift dns logs

# 实时查看DNS日志
gateshift dns logs -f

# 查看最近100行DNS日志
gateshift dns logs -n 100

# 过滤包含特定关键词的日志（不区分大小写）
gateshift dns logs -F "google"

# 过滤并实时查看
gateshift dns logs -F "query" -f

# 组合使用
gateshift dns logs -F "error" -n 20
```

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