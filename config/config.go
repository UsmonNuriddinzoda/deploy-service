package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port          string
	SecretToken   string
	DatabaseURL   string
	UIUsername    string
	UIPassword    string
	ScriptTimeout time.Duration
}

func Load() *Config {
	token := os.Getenv("SECRET_TOKEN")
	if token == "" {
		log.Fatal("[FATAL] SECRET_TOKEN environment variable is required")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("[FATAL] DATABASE_URL environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	uiUser := os.Getenv("UI_USERNAME")
	if uiUser == "" {
		uiUser = "admin"
	}
	uiPass := os.Getenv("UI_PASSWORD")
	if uiPass == "" {
		log.Fatal("[FATAL] UI_PASSWORD environment variable is required")
	}

	timeoutSec := 600 // 10 minutes default
	if ts := os.Getenv("SCRIPT_TIMEOUT"); ts != "" {
		if v, err := strconv.Atoi(ts); err == nil && v > 0 {
			timeoutSec = v
		}
	}

	return &Config{
		Port:          port,
		SecretToken:   token,
		DatabaseURL:   dbURL,
		UIUsername:    uiUser,
		UIPassword:    uiPass,
		ScriptTimeout: time.Duration(timeoutSec) * time.Second,
	}
}
