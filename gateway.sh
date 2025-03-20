#!/bin/bash

# 定义目标网关
PROXY_GATEWAY="192.168.31.100"
DEFAULT_GATEWAY="192.168.31.1"

# 获取当前活跃的网络接口
get_active_interface() {
    # 获取当前活跃的网络服务
    local active_service=$(networksetup -listnetworkserviceorder | grep -B 1 "$(route -n get default | grep interface | awk '{print $2}')" | head -n 1 | sed 's/^(.*) //')
    
    if [ -z "$active_service" ]; then
        echo "Error: No active network service found."
        exit 1
    fi
    
    # 获取该服务对应的接口名称
    local interface=$(networksetup -listallhardwareports | grep -A 1 "$active_service" | grep "Device:" | awk '{print $2}')
    
    if [ -z "$interface" ]; then
        echo "Error: Could not find interface for service $active_service"
        exit 1
    fi
    
    echo "$interface"
}

# 将十六进制子网掩码转换为点分十进制
convert_subnet_mask() {
    local hex=$1
    # 移除0x前缀
    hex=${hex#0x}
    # 转换为点分十进制
    printf "%d.%d.%d.%d\n" \
        $((16#${hex:0:2})) \
        $((16#${hex:2:2})) \
        $((16#${hex:4:2})) \
        $((16#${hex:6:2}))
}

# 获取活跃的网络接口和服务名称
ACTIVE_INTERFACE=$(get_active_interface)
ACTIVE_SERVICE=$(networksetup -listallhardwareports | grep -B 1 "$ACTIVE_INTERFACE" | head -n 1 | sed 's/Hardware Port: //')
echo "Active network interface: $ACTIVE_INTERFACE"
echo "Active network service: $ACTIVE_SERVICE"

# 检查网络接口是否已连接
if ! ifconfig "$ACTIVE_INTERFACE" >/dev/null 2>&1; then
    echo "Error: Network interface $ACTIVE_INTERFACE is not available."
    exit 1
fi

# 上网验证 (使用 ping 命令)
if ! ping -c 1 8.8.8.8 > /dev/null 2>&1; then
    echo "Error: Internet connection is not available."
    exit 1
fi

# 获取当前IP和子网掩码
CURRENT_IP=$(ifconfig "$ACTIVE_INTERFACE" | grep "inet " | awk '{print $2}')
HEX_SUBNET=$(ifconfig "$ACTIVE_INTERFACE" | grep "inet " | awk '{print $4}')
CURRENT_SUBNET=$(convert_subnet_mask "$HEX_SUBNET")
echo "Current IP: $CURRENT_IP"
echo "Current subnet: $CURRENT_SUBNET"

# 获取当前网关
CURRENT_GATEWAY=$(netstat -nr | grep "default" | grep "$ACTIVE_INTERFACE" | awk '{print $2}')
echo "Current gateway: $CURRENT_GATEWAY"

# 定义切换网关的函数
function switch_gateway() {
    GATEWAY=$1
    echo "Switching gateway to: $GATEWAY"

    # 使用 networksetup 命令设置网关
    # 保持当前的 IP 和子网掩码不变，只改变网关
    sudo networksetup -setmanual "$ACTIVE_SERVICE" "$CURRENT_IP" "$CURRENT_SUBNET" "$GATEWAY"

    echo "Gateway switched to: $GATEWAY"
}

# 判断是否需要切换到代理网关
if [ "$1" == "proxy" ]; then
    if [ "$CURRENT_GATEWAY" != "$PROXY_GATEWAY" ]; then
        switch_gateway "$PROXY_GATEWAY"
    else
        echo "Already using proxy gateway: $PROXY_GATEWAY"
    fi
# 判断是否需要切换到默认网关
elif [ "$1" == "default" ]; then
    if [ "$CURRENT_GATEWAY" != "$DEFAULT_GATEWAY" ]; then
        switch_gateway "$DEFAULT_GATEWAY"
    else
        echo "Already using default gateway: $DEFAULT_GATEWAY"
    fi
else
    echo "Usage: $0 [proxy|default]"
    exit 1
fi

exit 0

