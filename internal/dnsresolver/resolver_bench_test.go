package dnsresolver

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/miekg/dns"
)

func BenchmarkCacheHit(b *testing.B) {
	cfg := config.DNSConfig{
		CacheSize: 10000,
		CacheTTL:  3600,
		Timeout:   5 * time.Second,
	}

	resolver := NewResolver(cfg)

	// Предварительное заполнение кеша
	msg := &dns.Msg{}
	msg.SetQuestion("example.com.", dns.TypeA)
	resolver.cache.Set("example.com.", dns.TypeA, msg)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		resolver.cache.Get("example.com.", dns.TypeA)
	}
}

func BenchmarkCacheMiss(b *testing.B) {
	cfg := config.DNSConfig{
		CacheSize: 10000,
		CacheTTL:  3600,
		Timeout:   5 * time.Second,
		Upstreams: []string{"1.1.1.1:853"},
	}

	resolver := NewResolver(cfg)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msg := &dns.Msg{}
		msg.SetQuestion("test.example.com.", dns.TypeA)
		resolver.forward(msg)
	}
}

func BenchmarkMemoryUsage(b *testing.B) {
	cfg := config.DNSConfig{
		CacheSize: 10000,
		CacheTTL:  3600,
	}

	resolver := NewResolver(cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		domain := fmt.Sprintf("test%d.example.com.", i%10000)
		msg := &dns.Msg{}
		msg.SetQuestion(domain, dns.TypeA)

		// Добавляем фейковый ответ
		answer := &dns.A{
			Hdr: dns.RR_Header{
				Name:   domain,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: net.ParseIP("1.2.3.4"),
		}
		msg.Answer = append(msg.Answer, answer)

		resolver.cache.Set(domain, dns.TypeA, msg)
	}
}
