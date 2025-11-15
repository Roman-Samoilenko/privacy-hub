package main

import (
	"github.com/Roman-Samoilenko/privacy-hub/internal/logger"
	"github.com/Roman-Samoilenko/privacy-hub/internal/supervisor"
)

func main() {
	logger.Info("Starting privacy-hub...")
	supervisor.Start()
}
