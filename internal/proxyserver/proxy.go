package proxyserver

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
	"github.com/elazarl/goproxy"
)

func Start(ctx context.Context, cfg config.ProxyConfig) error {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false // Включите true для отладки

	// 1. Настройка MITM (расшифровка HTTPS)
	if cfg.MITMEnabled {
		logger.Infof("Enabling MITM for HTTPS inspection")

		// Загружаем пару ключей
		tlsc, err := tls.LoadX509KeyPair(cfg.MITMCACert, cfg.MITMCAKey)
		if err != nil {
			logger.Errorf("Failed to load MITM certs: %v", err)
			return err
		}

		// Включаем MITM для всех CONNECT запросов
		proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

		// ПРАВИЛЬНЫЙ СПОСОБ установки своего CA:
		goproxy.GoproxyCa = tlsc // Присваиваем глобальной переменной библиотеки
	} else {
		// Если MITM выключен, просто туннелируем HTTPS без расшифровки
		logger.Infof("MITM disabled - HTTPS headers will NOT be filtered")
	}

	// 2. Настройка фильтров (работает для HTTP и для расшифрованного HTTPS)
	if cfg.FilterHeads {
		proxy.OnRequest().DoFunc(newHeaderFilter(cfg.FilterListHeaders))
	}

	if cfg.UserAgent != "" {
		proxy.OnRequest().DoFunc(newUserAgentSetter(cfg.UserAgent))
	}

	server := &http.Server{
		Addr:    cfg.Listen,
		Handler: proxy,
		// Увеличиваем таймауты для тяжелых страниц
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	logger.Infof("Starting HTTP proxy on %s", cfg.Listen)

	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		logger.Infof("Shutting down proxy server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func newUserAgentSetter(ua string) func(*http.Request, *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		req.Header.Set("User-Agent", ua)
		return req, nil
	}
}

func newHeaderFilter(filterList []string) func(*http.Request, *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		for _, header := range filterList {
			req.Header.Del(header)
		}
		return req, nil
	}
}
