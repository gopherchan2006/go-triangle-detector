package config

import "os"

type AppConfig struct {
	DataDir string
	Symbols string
}

func LoadAppConfig() AppConfig {
	return AppConfig{
		DataDir: os.Getenv("DATA_DIR"),
		Symbols: os.Getenv("SYMBOLS"),
	}
}
