package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port       int
	BMHost     string
	BMPort     int
	SoundsDir  string
	DataDir    string
	PollMS     int
	DevMode    bool
}

func Load() *Config {
	return &Config{
		Port:      envInt("DC_PORT", 8080),
		BMHost:    envStr("DC_BM_HOST", "localhost"),
		BMPort:    envInt("DC_BM_PORT", 3180),
		SoundsDir: envStr("DC_SOUNDS_DIR", "./sounds"),
		DataDir:   envStr("DC_DATA_DIR", "./data"),
		PollMS:    envInt("DC_POLL_INTERVAL_MS", 200),
		DevMode:   envStr("DC_DEV", "") != "",
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
