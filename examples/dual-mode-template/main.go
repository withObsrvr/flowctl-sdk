// Package main implements a dual-mode Stellar processor that works in both
// nebu (Unix pipes) and flowctl (gRPC) modes.
//
// Mode detection is automatic based on environment variables:
// - If FLOWCTL_ENDPOINT or ENABLE_FLOWCTL=true: flowctl mode
// - Otherwise: nebu mode
//
// Usage:
//
//	# Nebu mode (default)
//	nebu fetch --start-ledger 60000000 | ./dual-mode-processor
//
//	# Flowctl mode
//	ENABLE_FLOWCTL=true ./dual-mode-processor
package main

import (
	"log"
	"os"
)

const version = "1.0.0"

func main() {
	log.Printf("Dual-Mode Processor v%s starting...", version)

	// Detect runtime environment
	if isFlowctlMode() {
		log.Println("Detected flowctl environment, starting in flowctl mode")
		runFlowctlMode()
	} else {
		log.Println("Starting in nebu mode (Unix pipes)")
		runNebuMode()
	}
}

// isFlowctlMode detects if we should run in flowctl mode
func isFlowctlMode() bool {
	// Check for flowctl-specific environment variables
	if os.Getenv("FLOWCTL_ENDPOINT") != "" {
		return true
	}
	if os.Getenv("ENABLE_FLOWCTL") == "true" {
		return true
	}
	// Check if we're being called by flowctl (via control plane registration)
	if os.Getenv("FLOWCTL_COMPONENT_ID") != "" {
		return true
	}
	return false
}
