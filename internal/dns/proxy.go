package dns

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// DNSProxy represents a DNS proxy server
type DNSProxy struct {
	listenAddr  string
	upstreamDNS []string
	conn        *net.UDPConn
	running     bool
	mu          sync.Mutex
	stopChan    chan struct{}
}

// NewDNSProxy creates a new DNS proxy
func NewDNSProxy(listenAddr string, upstreamDNS []string) (*DNSProxy, error) {
	return &DNSProxy{
		listenAddr:  listenAddr,
		upstreamDNS: upstreamDNS,
		running:     false,
		stopChan:    make(chan struct{}),
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

	p.running = true
	log.Printf("DNS proxy started on %s", addr)
	log.Printf("Using upstream DNS servers: %v", p.upstreamDNS)
	return nil
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

			log.Printf("Received DNS query from %s (%d bytes)", addr.String(), n)
			// Process the query
			go p.processQuery(buffer[:n], addr)
		}
	}
}

// processQuery handles a single DNS query
func (p *DNSProxy) processQuery(query []byte, clientAddr *net.UDPAddr) {
	if len(p.upstreamDNS) == 0 {
		log.Printf("No upstream DNS servers configured")
		return
	}

	log.Printf("Processing DNS query from %s", clientAddr.String())

	// Use the first upstream DNS server for now
	// In a more advanced implementation, we could try multiple servers or implement
	// more sophisticated server selection
	upstreamServer := p.upstreamDNS[0]
	log.Printf("Forwarding query to upstream DNS server: %s", upstreamServer)

	// Connect to the upstream DNS server
	upstreamAddr, err := net.ResolveUDPAddr("udp", upstreamServer)
	if err != nil {
		log.Printf("Failed to resolve upstream DNS server %s: %v", upstreamServer, err)
		return
	}

	upstreamConn, err := net.DialUDP("udp", nil, upstreamAddr)
	if err != nil {
		log.Printf("Failed to connect to upstream DNS server %s: %v", upstreamServer, err)
		return
	}
	defer upstreamConn.Close()

	// Send the query to upstream DNS
	bytesWritten, err := upstreamConn.Write(query)
	if err != nil {
		log.Printf("Failed to send query to upstream DNS server: %v", err)
		return
	}
	log.Printf("Query sent to upstream DNS server %s (%d bytes)", upstreamServer, bytesWritten)

	// Receive the response
	response := make([]byte, 4096)
	upstreamConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := upstreamConn.Read(response)
	if err != nil {
		log.Printf("Failed to receive response from upstream DNS server: %v", err)
		return
	}
	log.Printf("Received response from upstream DNS server (%d bytes)", n)

	// Send the response back to the client
	bytesWritten, err = p.conn.WriteToUDP(response[:n], clientAddr)
	if err != nil {
		log.Printf("Failed to send response to client: %v", err)
		return
	}
	log.Printf("Response sent back to client %s (%d bytes)", clientAddr.String(), bytesWritten)
}
