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
	SmartOLTEnabled   bool
	SmartOLTBaseURL   string
	SmartOLTAPIKey    string
	SmartOLTZoneID    string
	SmartOLTZoneName  string
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
		SmartOLTEnabled:   getEnv("SMARTOLT_ENABLED", "false") == "true" || getEnv("SMARTOLT_ENABLED", "false") == "1",
		SmartOLTBaseURL:   getEnv("SMARTOLT_BASE_URL", "https://gnet-biaro.smartolt.com"),
		SmartOLTAPIKey:    getEnv("SMARTOLT_API_KEY", "b71455fab579457d98d8f4b6d28cfdff"),
		SmartOLTZoneID:    getEnv("SMARTOLT_ZONE_ID", "122"),
		SmartOLTZoneName:  getEnv("SMARTOLT_ZONE_NAME", "GOGIGA"),
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
