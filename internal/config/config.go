package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port              string
	DatabaseDriver    string // "sqlite" or "postgres"
	DatabaseDSN       string
	GigabillBaseURL   string
	GigabillAPIToken  string
	MaxCoverageMeters float64
	AdminAPIKey       string
	FTTXBaseURL       string
	FTTXAdminKey      string
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:              getEnv("PORT", "8081"),
		DatabaseDriver:    getEnv("DB_DRIVER", "sqlite"),
		DatabaseDSN:       getEnv("DATABASE_URL", "onboarding.db"),
		GigabillBaseURL:   getEnv("GIGABILL_BASE_URL", "http://localhost:8080"),
		GigabillAPIToken:  getEnv("GIGABILL_API_TOKEN", "supersecret-admin-token"),
		MaxCoverageMeters: getEnvFloat("MAX_COVERAGE_METERS", 250.0), // Standard dropcore limit
		AdminAPIKey:       getEnv("ADMIN_API_KEY", "isp-onboarding-admin-key"),
		FTTXBaseURL:       getEnv("FTTX_BASE_URL", "http://localhost:8082"),
		FTTXAdminKey:      getEnv("FTTX_ADMIN_KEY", "gogiga-noc-admin-99a8f27c3d14"),
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return defaultVal
	}
	return val
}
