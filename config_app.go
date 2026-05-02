package main

import "os"

// AppConfig holds all runtime configuration sourced from environment variables.
type AppConfig struct {
	DataDir string
	Symbols string
}

// LoadAppConfig reads environment variables into AppConfig.
// Call after loadEnvFile so .env values are present.
func LoadAppConfig() AppConfig {
	return AppConfig{
		DataDir: os.Getenv("DATA_DIR"),
		Symbols: os.Getenv("SYMBOLS"),
	}
}
