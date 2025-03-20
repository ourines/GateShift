# GateShift

![License](https://img.shields.io/github/license/ourines/GateShift)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/ourines/GateShift)

A cross-platform gateway switching tool designed for OpenWrt bypass routers, allowing seamless traffic path switching between default and proxy gateways.

English | [简体中文](./README.md)

## Background

In network environments using OpenWrt as a bypass router, users often need to switch between the default gateway and the OpenWrt bypass router gateway to meet different internet access needs. This process typically requires manual network setting modifications, which is cumbersome and error-prone. GateShift was created to solve this problem, offering a simple command-line tool that enables one-click gateway switching, greatly simplifying network management.

## Core Features

- **Seamless Gateway Switching**: Switch between main router and OpenWrt bypass router gateways with a single command
- **Cross-Platform Support**: Compatible with macOS, Linux, and Windows systems
- **Configuration Persistence**: Automatically remembers your gateway configurations
- **Permission Management**: Built-in sudo session management to avoid repeated password entry
- **Real-time Status Detection**: Provides current network status and internet connectivity checks
- **System-wide Installation**: Supports global installation for command access from anywhere

## Installation

### Method 1: Using the Install Command

If you've already built or downloaded the binary, you can install it system-wide with:

```bash
gateshift install
```

This will install the GateShift tool to your system so it can be called from anywhere in your terminal. This command requires administrator privileges.

To uninstall:

```bash
gateshift uninstall
```

### Method 2: From Source

1. Clone this repository
2. Build the application:

```bash
make build
```

3. Install it to your local bin directory:

```bash
make install
```

### Method 3: Prebuilt Binaries

Download the latest prebuilt binary for your platform from the [Releases page](https://github.com/ourines/GateShift/releases).

## Usage

```bash
# Switch to bypass router gateway (e.g., OpenWrt)
gateshift proxy

# Switch back to default gateway (e.g., main router)
gateshift default

# Show current network status
gateshift status

# Configure gateways
gateshift config set-proxy 192.168.31.100  # Set OpenWrt bypass router IP
gateshift config set-default 192.168.31.1  # Set main router IP
gateshift config show

# Install system-wide
gateshift install

# Uninstall from system
gateshift uninstall
```

## Configuration

The application stores its configuration in `~/.gateshift/config.yaml`. You can edit this file manually or use the `config` commands.

Default configuration:

```yaml
proxy_gateway: 192.168.31.100  # OpenWrt bypass router IP
default_gateway: 192.168.31.1  # Main router IP
```

## Typical Use Cases

- **Regular/Proxy Internet Switching**: Quickly switch between regular internet access and proxy-based access
- **Multiple Network Environments**: Flexibly transition between different network proxy strategies
- **Application-specific Networking**: Provide convenient gateway management for applications requiring specific network environments
- **Network Testing**: Test connection quality and speed across different network paths
- **Home/Office Network Management**: Manage multiple network schemes

## Building for Different Platforms

To build for all supported platforms:

```bash
make build-all
```

This will create binaries for:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

The binaries will be placed in the `bin/` directory.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. 