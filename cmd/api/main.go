package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"isp-onboarding/internal/billingclient"
	"isp-onboarding/internal/config"
	"isp-onboarding/internal/handler"
	"isp-onboarding/internal/repository"
	"isp-onboarding/internal/service"
	"isp-onboarding/internal/smartoltclient"
)

func main() {
	cfg := config.Load()

	log.Printf("==================================================")
	log.Printf("   ISP ONBOARDING & PARTNER REGISTRATION GATEWAY  ")
	log.Printf("==================================================")
	log.Printf("Port                : %s", cfg.Port)
	log.Printf("DB Storage          : %s (%s)", cfg.DatabaseDriver, cfg.DatabaseDSN)
	log.Printf("GOGIGABILL Core URL : %s", cfg.GigabillBaseURL)
	log.Printf("Max Coverage Limit  : %.0f meters", cfg.MaxCoverageMeters)

	// 1. Inisialisasi Storage (SQLite / PostgreSQL with PostGIS)
	var storage repository.Storage
	var err error
	if strings.ToLower(cfg.DatabaseDriver) == "postgres" || strings.ToLower(cfg.DatabaseDriver) == "postgresql" {
		storage, err = repository.NewPostgresStorage(cfg.DatabaseDSN)
		if err != nil {
			log.Fatalf("Gagal inisialisasi PostgreSQL (PostGIS) database: %v", err)
		}
		log.Printf("PostgreSQL + PostGIS Centralized Storage initialized successfully.")
	} else {
		storage, err = repository.NewSQLiteStorage(cfg.DatabaseDSN)
		if err != nil {
			log.Fatalf("Gagal inisialisasi SQLite database: %v", err)
		}
		log.Printf("SQLite database storage initialized with demo ODPs and Partners.")
	}
	defer storage.Close()

	// 2. Inisialisasi GOGIGABILL Client & SmartOLT Client
	billingClient := billingclient.New(cfg.GigabillBaseURL, cfg.GigabillAPIToken)
	var smartOLTClient *smartoltclient.Client
	if cfg.SmartOLTEnabled && cfg.SmartOLTAPIKey != "" {
		smartOLTClient = smartoltclient.New(cfg.SmartOLTBaseURL, cfg.SmartOLTAPIKey)
		log.Printf("SmartOLT Jartaplok Client: CONNECTED (%s, Zone: %s)", cfg.SmartOLTBaseURL, cfg.SmartOLTZoneName)
	}

	// Cek koneksi ke GOGIGABILL
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if ok, _ := billingClient.CheckHealth(ctx); ok {
		log.Printf("GOGIGABILL Core Billing Connection: CONNECTED (OK)")
	} else {
		log.Printf("GOGIGABILL Core Billing Connection: STANDALONE MODE (Core offline / not reachable, fallback active)")
	}

	// 3. Inisialisasi Business Service
	onboardingSvc := service.NewOnboardingService(storage, billingClient, cfg.MaxCoverageMeters)

	// 4. Inisialisasi HTTP Handler & Router
	server := handler.NewServer(onboardingSvc, storage, billingClient, cfg.AdminAPIKey, cfg.FTTXBaseURL, cfg.FTTXAdminKey).WithSmartOLT(smartOLTClient)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      server.Routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run server in goroutine
	go func() {
		log.Printf("API Server listening on http://localhost:%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Onboarding Gateway...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	fmt.Println("Server exited properly.")
}
