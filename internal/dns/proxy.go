package dns

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// DNSCache 表示DNS缓存
type DNSCache struct {
	cache map[string]CacheEntry
	mu    sync.RWMutex
}

// CacheEntry 表示缓存条目
type CacheEntry struct {
	response   []byte
	expiration time.Time
}

// NewDNSCache 创建新的DNS缓存
func NewDNSCache() *DNSCache {
	return &DNSCache{
		cache: make(map[string]CacheEntry),
	}
}

// Get 从缓存中获取响应
func (c *DNSCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists || time.Now().After(entry.expiration) {
		if exists {
			// 删除过期条目
			go func() {
				c.mu.Lock()
				delete(c.cache, key)
				c.mu.Unlock()
			}()
		}
		return nil, false
	}

	return entry.response, true
}

// Set 将响应存入缓存
func (c *DNSCache) Set(key string, response []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = CacheEntry{
		response:   response,
		expiration: time.Now().Add(ttl),
	}
}

// CleanupExpired 清理过期缓存条目
func (c *DNSCache) CleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.cache {
		if now.After(entry.expiration) {
			delete(c.cache, key)
		}
	}
}

// Size 返回缓存大小
func (c *DNSCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// DNSProxy represents a DNS proxy server
type DNSProxy struct {
	listenAddr  string
	upstreamDNS []string
	conn        *net.UDPConn
	running     bool
	mu          sync.Mutex
	stopChan    chan struct{}
	cache       *DNSCache
}

// NewDNSProxy creates a new DNS proxy
func NewDNSProxy(listenAddr string, upstreamDNS []string) (*DNSProxy, error) {
	return &DNSProxy{
		listenAddr:  listenAddr,
		upstreamDNS: upstreamDNS,
		running:     false,
		stopChan:    make(chan struct{}),
		cache:       NewDNSCache(),
	}, nil
}

// Start starts the DNS proxy server
func (p *DNSProxy) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("DNS proxy is already running")
	}

	// 固定使用53端口
	const port = 53

	// Bind UDP port
	addr := fmt.Sprintf("%s:%d", p.listenAddr, port)
	log.Printf("Attempting to bind to %s", addr)

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	p.conn = conn
	log.Printf("Successfully bound to %s", addr)

	// Handle DNS requests
	go p.handleRequests()

	// 启动缓存清理协程
	go p.cacheCleanupTask()

	p.running = true
	log.Printf("DNS proxy started on %s", addr)
	log.Printf("Using upstream DNS servers: %v", p.upstreamDNS)
	return nil
}

// cacheCleanupTask 定期清理过期缓存
func (p *DNSProxy) cacheCleanupTask() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			beforeSize := p.cache.Size()
			p.cache.CleanupExpired()
			afterSize := p.cache.Size()
			if beforeSize != afterSize {
				log.Printf("Cache cleanup: removed %d expired entries, current size: %d", beforeSize-afterSize, afterSize)
			}
		case <-p.stopChan:
			return
		}
	}
}

// Stop stops the DNS proxy server
func (p *DNSProxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	close(p.stopChan)
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}

	p.running = false
	log.Printf("DNS proxy stopped")
	return nil
}

// IsRunning returns true if the proxy is running
func (p *DNSProxy) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// GetPort returns the port that the DNS proxy is listening on
func (p *DNSProxy) GetPort() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return 53
}

// handleRequests handles incoming DNS requests
func (p *DNSProxy) handleRequests() {
	buffer := make([]byte, 4096)
	log.Printf("DNS request handler started")

	for {
		select {
		case <-p.stopChan:
			log.Printf("DNS request handler received stop signal")
			return
		default:
			p.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, addr, err := p.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout, just continue
					continue
				}
				log.Printf("Error reading from UDP: %v", err)
				continue
			}

			// 复制查询数据，因为buffer会被重用
			query := make([]byte, n)
			copy(query, buffer[:n])

			log.Printf("Received DNS query from %s (%d bytes)", addr.String(), n)
			// Process the query
			go p.processQuery(query, addr)
		}
	}
}

// extractQueryName 从DNS查询中提取域名，用于缓存键
func extractQueryName(query []byte) (string, error) {
	if len(query) < 12 {
		return "", fmt.Errorf("query too short")
	}

	// 跳过DNS头部（12字节）
	offset := 12
	var labels []string

	// 解析域名标签
	for offset < len(query) {
		labelLength := int(query[offset])
		if labelLength == 0 {
			break // 域名结束
		}

		// 检查是否超出查询边界
		if offset+1+labelLength > len(query) {
			return "", fmt.Errorf("malformed query")
		}

		// 提取标签
		label := string(query[offset+1 : offset+1+labelLength])
		labels = append(labels, label)
		offset += 1 + labelLength
	}

	if len(labels) == 0 {
		return "", fmt.Errorf("no domain in query")
	}

	// 提取查询类型（如A, AAAA, MX等）
	qtype := binary.BigEndian.Uint16(query[offset+1 : offset+3])

	// 构建缓存键：域名+查询类型
	domainName := fmt.Sprintf("%s|%d", labels, qtype)
	return domainName, nil
}

// getTTL 从DNS响应中提取TTL
func getTTL(response []byte) time.Duration {
	// 这里简化处理，实际上应该解析响应中的TTL字段
	// 默认缓存10分钟
	return 10 * time.Minute
}

// queryUpstreamServer 向单个上游DNS服务器发送查询
func (p *DNSProxy) queryUpstreamServer(server string, query []byte) ([]byte, error) {
	// 连接到上游DNS服务器
	upstreamAddr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve upstream DNS server %s: %v", server, err)
	}

	upstreamConn, err := net.DialUDP("udp", nil, upstreamAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to upstream DNS server %s: %v", server, err)
	}
	defer upstreamConn.Close()

	// 发送查询
	if _, err := upstreamConn.Write(query); err != nil {
		return nil, fmt.Errorf("failed to send query to upstream DNS server: %v", err)
	}

	// 接收响应
	response := make([]byte, 4096)
	upstreamConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := upstreamConn.Read(response)
	if err != nil {
		return nil, fmt.Errorf("failed to receive response from upstream DNS server: %v", err)
	}

	// 返回响应
	return response[:n], nil
}

// processQuery handles a single DNS query
func (p *DNSProxy) processQuery(query []byte, clientAddr *net.UDPAddr) {
	if len(p.upstreamDNS) == 0 {
		log.Printf("No upstream DNS servers configured")
		return
	}

	log.Printf("Processing DNS query from %s", clientAddr.String())

	// 尝试从缓存获取
	cacheKey, err := extractQueryName(query)
	if err == nil {
		if response, found := p.cache.Get(cacheKey); found {
			// 使用缓存的响应
			bytesWritten, err := p.conn.WriteToUDP(response, clientAddr)
			if err != nil {
				log.Printf("Failed to send cached response to client: %v", err)
				return
			}
			log.Printf("Cache hit: Response sent to client %s (%d bytes)", clientAddr.String(), bytesWritten)
			return
		}
	}

	// 缓存未命中，并行查询所有上游DNS服务器
	responseChan := make(chan []byte, len(p.upstreamDNS))
	timeoutChan := time.After(5 * time.Second)

	// 并行向所有上游DNS服务器发送查询
	for _, server := range p.upstreamDNS {
		go func(upstreamServer string) {
			log.Printf("Forwarding query to upstream DNS server: %s", upstreamServer)

			response, err := p.queryUpstreamServer(upstreamServer, query)
			if err != nil {
				log.Printf("Failed query to %s: %v", upstreamServer, err)
				return
			}

			log.Printf("Received response from upstream DNS server %s (%d bytes)", upstreamServer, len(response))

			// 发送到响应通道
			select {
			case responseChan <- response:
			default:
				// 已经收到更快的响应，忽略这个
			}
		}(server)
	}

	// 等待第一个响应或超时
	select {
	case response := <-responseChan:
		// 将响应发送给客户端
		bytesWritten, err := p.conn.WriteToUDP(response, clientAddr)
		if err != nil {
			log.Printf("Failed to send response to client: %v", err)
			return
		}
		log.Printf("Response sent back to client %s (%d bytes)", clientAddr.String(), bytesWritten)

		// 将响应添加到缓存
		if err == nil {
			ttl := getTTL(response)
			p.cache.Set(cacheKey, response, ttl)
		}

	case <-timeoutChan:
		log.Printf("All DNS queries timed out for client %s", clientAddr.String())
	}
}

// CacheStats 返回缓存统计信息
func (p *DNSProxy) CacheStats() map[string]interface{} {
	return map[string]interface{}{
		"size": p.cache.Size(),
	}
}

// ClearCache 清除缓存
func (p *DNSProxy) ClearCache() {
	p.cache.mu.Lock()
	defer p.cache.mu.Unlock()

	size := len(p.cache.cache)
	p.cache.cache = make(map[string]CacheEntry)
	log.Printf("DNS cache cleared, %d entries removed", size)
}
