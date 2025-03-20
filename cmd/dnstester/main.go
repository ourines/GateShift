package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	// 要解析的域名
	domain := "example.com"
	if len(os.Args) > 1 {
		domain = os.Args[1]
	}

	// 要使用的端口号
	port := 53
	if len(os.Args) > 2 {
		p, err := strconv.Atoi(os.Args[2])
		if err == nil && p > 0 && p < 65536 {
			port = p
		}
	}

	// 检查 DNS 服务是否正在运行
	dnsAddr := fmt.Sprintf("127.0.0.1:%d", port)
	fmt.Printf("Testing DNS resolution for %s using %s...\n", domain, dnsAddr)

	// 首先检查 DNS 服务器可连接性
	conn, err := net.DialTimeout("udp", dnsAddr, 2*time.Second)
	if err != nil {
		fmt.Printf("Failed to connect to DNS server at %s: %v\n", dnsAddr, err)
		fmt.Println("Make sure the DNS service is running with 'sudo ./bin/gateshift dns show'")
		fmt.Println("You can restart it with 'sudo ./bin/gateshift dns restart'")
	} else {
		fmt.Printf("Successfully connected to DNS server at %s\n", dnsAddr)
		conn.Close()
	}

	// 使用系统的 DNS 设置进行解析
	fmt.Println("\n1. Using system DNS settings:")
	start := time.Now()

	// 使用系统默认 DNS 解析
	ips, err := net.LookupIP(domain)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("Failed to resolve %s: %v\n", domain, err)
	} else {
		fmt.Printf("Successfully resolved %s in %v\n", domain, elapsed.Round(time.Millisecond))
		fmt.Println("IP Addresses found:")
		for _, ip := range ips {
			fmt.Printf("  %s\n", ip.String())
		}
	}

	// 明确指定我们的 DNS 服务器
	fmt.Printf("\n2. Using explicit DNS server at %s:\n", dnsAddr)
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Second * 10,
			}
			return d.DialContext(ctx, network, dnsAddr)
		},
	}

	start = time.Now()
	ipsExplicit, err := r.LookupIPAddr(context.Background(), domain)
	elapsedExplicit := time.Since(start)

	if err != nil {
		fmt.Printf("Failed to resolve %s via explicit resolver: %v\n", domain, err)
		fmt.Println("Try running with sudo or checking your DNS proxy status")
	} else {
		fmt.Printf("Successfully resolved %s via explicit resolver in %v\n", domain, elapsedExplicit.Round(time.Millisecond))
		fmt.Println("IP Addresses found:")
		for _, ip := range ipsExplicit {
			fmt.Printf("  %s\n", ip.String())
		}
	}

	// 测试原始 UDP DNS 查询
	fmt.Printf("\n3. Testing raw UDP DNS query to %s:\n", dnsAddr)
	result := testRawDNSQuery(domain, dnsAddr)
	if result {
		fmt.Printf("Successfully received response for raw DNS query to %s\n", domain)
	} else {
		fmt.Printf("Failed to receive response for raw DNS query\n")
	}
}

// 测试原始 DNS 查询
func testRawDNSQuery(domain string, serverAddr string) bool {
	// 创建 UDP 连接
	conn, err := net.Dial("udp", serverAddr)
	if err != nil {
		fmt.Printf("Error creating UDP connection: %v\n", err)
		return false
	}
	defer conn.Close()

	// 设置超时
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// 构建一个简单的 DNS 查询包
	query := buildDNSQuery(domain)

	// 发送查询
	_, err = conn.Write(query)
	if err != nil {
		fmt.Printf("Error sending DNS query: %v\n", err)
		return false
	}

	// 接收响应
	resp := make([]byte, 512)
	n, err := conn.Read(resp)
	if err != nil {
		fmt.Printf("Error receiving DNS response: %v\n", err)
		return false
	}

	// 如果收到响应，则认为测试成功
	fmt.Printf("Received %d bytes in response\n", n)
	return true
}

// 构建一个简单的 DNS 查询包
func buildDNSQuery(domain string) []byte {
	// 简单的 DNS 查询包，查询 A 记录
	query := []byte{
		0x00, 0x01, // ID
		0x01, 0x00, // 标准查询
		0x00, 0x01, // 问题数: 1
		0x00, 0x00, // 应答数: 0
		0x00, 0x00, // 权威应答数: 0
		0x00, 0x00, // 附加数: 0
	}

	// 添加查询的域名
	parts := net.ParseIP(domain)
	if parts == nil {
		// 如果不是 IP 地址，按照域名处理
		labels := strings.Split(domain, ".")
		for _, label := range labels {
			query = append(query, byte(len(label)))
			query = append(query, []byte(label)...)
		}
	} else {
		// 如果是 IP 地址，构建反向查询
		query = append(query, 0x07) // 7 字节
		query = append(query, []byte("example")...)
		query = append(query, 0x03) // 3 字节
		query = append(query, []byte("com")...)
	}

	query = append(query, 0x00)       // 域名结束
	query = append(query, 0x00, 0x01) // 类型: A
	query = append(query, 0x00, 0x01) // 类: IN

	return query
}
