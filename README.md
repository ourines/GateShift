# Proxy Gateway Switcher

A cross-platform command-line utility for switching between network gateways. This tool allows you to easily switch between your default gateway and a proxy gateway.

## Features

- Cross-platform support (macOS, Linux, Windows)
- Remembers your gateway configurations
- Sudo session management (no need to re-enter password for subsequent operations)
- Status reporting with internet connectivity checking
- Clean command-line interface
- System-wide installation support

## Installation

### Method 1: Using the Install Command

If you've already built or downloaded the binary, you can install it system-wide with:

```
proxy install
```

This will install the proxy tool to your system so it can be called from anywhere in your terminal. This command requires administrator privileges.

To uninstall:

```
proxy uninstall
```

### Method 2: From Source

1. Clone this repository
2. Build the application:

```
make build
```

3. Install it to your local bin directory:

```
make install
```

### Method 3: Prebuilt Binaries

Download the latest prebuilt binary for your platform from the Releases page.

## Usage

```
# Switch to proxy gateway
proxy proxy

# Switch to default gateway
proxy default

# Show current network status
proxy status

# Configure gateways
proxy config set-proxy 192.168.31.100
proxy config set-default 192.168.31.1
proxy config show

# Install system-wide
proxy install

# Uninstall from system
proxy uninstall
```

## Configuration

The application stores its configuration in `~/.proxy/config.yaml`. You can edit this file manually or use the `config` commands.

Default configuration:

```yaml
proxy_gateway: 192.168.31.100
default_gateway: 192.168.31.1
```

## Building for Different Platforms

To build for all supported platforms:

```
make build-all
```

This will create binaries for:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

The binaries will be placed in the `bin/` directory.

## License

This project is licensed under the MIT License - see the LICENSE file for details. 