package dnsresolver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
	"github.com/miekg/dns"
)

type Resolver struct {
	cache       *Cache
	filter      *Filter
	upstreams   []string
	dohUpstream []string
	client      *dns.Client
	dohClient   *dns.Client
	timeout     time.Duration
	mu          sync.RWMutex
}

func NewResolver(cfg config.DNSConfig) *Resolver {
	r := &Resolver{
		cache:       NewCache(cfg.CacheSize, time.Duration(cfg.CacheTTL)*time.Second),
		filter:      NewFilter(cfg.Blocklist, cfg.Allowlist, cfg.EnableFiltering),
		upstreams:   cfg.Upstreams,
		dohUpstream: cfg.DoHUpstreams,
		timeout:     cfg.Timeout,
		client: &dns.Client{
			Net:     "tcp-tls",
			Timeout: cfg.Timeout,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
		dohClient: &dns.Client{
			Net:     "tcp",
			Timeout: cfg.Timeout,
		},
	}
	return r
}

func (r *Resolver) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	if len(req.Question) == 0 {
		dns.HandleFailed(w, req)
		return
	}

	question := req.Question[0]
	domain := question.Name
	qtype := dns.TypeToString[question.Qtype]

	logger.Debugf("DNS query: %s %s from %s", domain, qtype, w.RemoteAddr())

	// Check filter
	if r.filter.IsBlocked(domain) {
		logger.Infof("Blocked domain: %s", domain)
		r.sendNXDomain(w, req)
		return
	}

	// Check cache
	if cached := r.cache.Get(domain, question.Qtype); cached != nil {
		logger.Debugf("Cache hit: %s %s", domain, qtype)
		cached.SetReply(req)
		w.WriteMsg(cached)
		return
	}

	// Forward to upstream
	resp, err := r.forward(req)
	if err != nil {
		logger.Errorf("Forward failed for %s: %v", domain, err)
		dns.HandleFailed(w, req)
		return
	}

	// Cache successful response
	if resp.Rcode == dns.RcodeSuccess {
		r.cache.Set(domain, question.Qtype, resp)
	}

	// Send response
	resp.SetReply(req)
	w.WriteMsg(resp)

	logger.Debugf("Resolved: %s %s -> %d answers", domain, qtype, len(resp.Answer))
}

func (r *Resolver) forward(req *dns.Msg) (*dns.Msg, error) {
	var lastErr error

	// Try DNS-over-TLS first
	for _, upstream := range r.upstreams {
		resp, _, err := r.client.Exchange(req, upstream)
		if err == nil && resp != nil {
			return resp, nil
		}
		lastErr = err
		logger.Debugf("DoT upstream %s failed: %v", upstream, err)
	}

	// Fallback to DNS-over-HTTPS
	for _, dohURL := range r.dohUpstream {
		resp, err := r.queryDoH(req, dohURL)
		if err == nil && resp != nil {
			return resp, nil
		}
		lastErr = err
		logger.Debugf("DoH upstream %s failed: %v", dohURL, err)
	}

	return nil, fmt.Errorf("all upstreams failed: %v", lastErr)
}

func (r *Resolver) queryDoH(req *dns.Msg, url string) (*dns.Msg, error) {
	// Simple DoH implementation via DNS wire format over HTTPS
	// In production, use a proper DoH library
	host, _, err := net.SplitHostPort(url)
	if err != nil {
		host = url
	}

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: r.timeout},
		"tcp",
		host+":443",
		&tls.Config{ServerName: host},
	)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	co := &dns.Conn{Conn: conn}
	if err := co.WriteMsg(req); err != nil {
		return nil, err
	}

	return co.ReadMsg()
}

func (r *Resolver) sendNXDomain(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetRcode(req, dns.RcodeNameError)
	w.WriteMsg(m)
}

func Start(ctx context.Context, cfg config.DNSConfig) error {
	resolver := NewResolver(cfg)

	// UDP server
	udpServer := &dns.Server{
		Addr:    cfg.Listen,
		Net:     "udp",
		Handler: resolver,
		UDPSize: 65535,
	}

	// TCP server (for large responses)
	tcpAddr := cfg.Listen
	if tcpAddr == ":9000" {
		tcpAddr = ":9001"
	}
	tcpServer := &dns.Server{
		Addr:    tcpAddr,
		Net:     "tcp",
		Handler: resolver,
	}

	errChan := make(chan error, 2)

	// Start UDP server
	go func() {
		logger.Infof("DNS UDP server listening on %s", udpServer.Addr)
		if err := udpServer.ListenAndServe(); err != nil {
			errChan <- fmt.Errorf("UDP server: %v", err)
		}
	}()

	// Start TCP server
	go func() {
		logger.Infof("DNS TCP server listening on %s", tcpServer.Addr)
		if err := tcpServer.ListenAndServe(); err != nil {
			errChan <- fmt.Errorf("TCP server: %v", err)
		}
	}()

	// Wait for shutdown or error
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		logger.Infof("Shutting down DNS servers...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := udpServer.ShutdownContext(shutdownCtx); err != nil {
			logger.Errorf("UDP shutdown error: %v", err)
		}
		if err := tcpServer.ShutdownContext(shutdownCtx); err != nil {
			logger.Errorf("TCP shutdown error: %v", err)
		}
		return nil
	}
}
