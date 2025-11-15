package proxyserver

import (
	"net/http"

	"context"
	"time"

	"github.com/Roman-Samoilenko/privacy-hub/internal/config"
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
	"github.com/elazarl/goproxy"
)

// TODO: implement HTTPS, SOCKS proxy

func Start(ctx context.Context, proxyConf config.ProxyConfig) error {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true

	port := proxyConf.Listen
	userAgent := proxyConf.UserAgent
	filterHeads := proxyConf.FilterHeads
	if filterHeads {
		filterList := proxyConf.FilterListHeaders
		proxy.OnRequest().DoFunc(newHeaderFilter(filterList))
	}

	proxy.OnRequest().DoFunc(newUserAgentSetter(userAgent))

	server := &http.Server{
		Addr:    port,
		Handler: proxy,
	}

	logger.Info("Starting Proxy server on %s", proxyConf.Listen)

	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Proxy failed: %v", err)
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		logger.Info("Shutting down proxy server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func newUserAgentSetter(userAgent string) func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if userAgent == "" {
			return req, nil
		}
		req.Header.Set("User-Agent", userAgent)
		return req, nil
	}
}

func newHeaderFilter(filterList []string) func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		for _, header := range filterList {
			req.Header.Del(header)
		}
		return req, nil
	}
}
