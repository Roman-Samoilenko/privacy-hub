package main

import (
	"log"

	"privacy-hub/internal/supervisor"
)

func main() {
	log.Println("Starting privacy-hub...")
	supervisor.Start()
}
