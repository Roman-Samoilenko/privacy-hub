package dnsresolver

import (
	"strings"
	"sync"
)

type Filter struct {
	blocklist map[string]bool
	allowlist map[string]bool
	enabled   bool
	mu        sync.RWMutex
}

func NewFilter(blocklist, allowlist []string, enabled bool) *Filter {
	f := &Filter{
		blocklist: make(map[string]bool),
		allowlist: make(map[string]bool),
		enabled:   enabled,
	}

	for _, domain := range blocklist {
		f.blocklist[normalizeDomain(domain)] = true
	}

	for _, domain := range allowlist {
		f.allowlist[normalizeDomain(domain)] = true
	}

	return f
}

func (f *Filter) IsBlocked(domain string) bool {
	if !f.enabled {
		return false
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	domain = normalizeDomain(domain)

	// Check allowlist first
	if f.allowlist[domain] {
		return false
	}

	// Check blocklist
	if f.blocklist[domain] {
		return true
	}

	// Check parent domains
	parts := strings.Split(domain, ".")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[i:], ".")
		if f.blocklist[parent] {
			return true
		}
	}

	return false
}

func (f *Filter) AddToBlocklist(domain string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blocklist[normalizeDomain(domain)] = true
}

func (f *Filter) RemoveFromBlocklist(domain string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.blocklist, normalizeDomain(domain))
}

func (f *Filter) AddToAllowlist(domain string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allowlist[normalizeDomain(domain)] = true
}

func normalizeDomain(domain string) string {
	domain = strings.ToLower(domain)
	return strings.TrimSuffix(domain, ".")
}
