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

	"ispsync/internal/auth"
	"ispsync/internal/handler"
	"ispsync/internal/middleware"
	"ispsync/internal/repository"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082" // Port 8082 agar tidak bentrok dengan 8080 (GOGIGABILL) atau 8081 (ISP Onboarding)
	}

	dbURL := os.Getenv("DATABASE_URL")
	dbPath := os.Getenv("DATABASE_PATH")

	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "ispsync.id"
	}

	log.Printf("[ISPSYNC CORE] Starting SaaS Multi-Tenant Engine on port %s...", port)
	log.Printf("[ISPSYNC CORE] Base SaaS Domain: %s", baseDomain)

	// 1. Inisialisasi Database Storage (PostgreSQL Primary / SQLite Fallback)
	var store repository.Storage
	if dbURL != "" && (strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://")) {
		log.Printf("[ISPSYNC CORE] Initializing Central PostgreSQL Storage...")
		pgStore, err := repository.NewPostgresStorage(dbURL)
		if err != nil {
			log.Fatalf("Fatal: Gagal inisialisasi PostgreSQL: %v", err)
		}
		store = pgStore
	} else if os.Getenv("POSTGRES_HOST") != "" {
		host := os.Getenv("POSTGRES_HOST")
		pgPort := os.Getenv("POSTGRES_PORT")
		if pgPort == "" {
			pgPort = "5432"
		}
		user := os.Getenv("POSTGRES_USER")
		pass := os.Getenv("POSTGRES_PASSWORD")
		dbname := os.Getenv("POSTGRES_DB")
		if dbname == "" {
			dbname = "ispsync"
		}
		connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, pgPort, dbname)
		log.Printf("[ISPSYNC CORE] Initializing PostgreSQL Storage from ENV (%s:%s/%s)...", host, pgPort, dbname)
		pgStore, err := repository.NewPostgresStorage(connStr)
		if err != nil {
			log.Fatalf("Fatal: Gagal inisialisasi PostgreSQL: %v", err)
		}
		store = pgStore
	} else if dbPath != "" {
		log.Printf("[ISPSYNC CORE] Initializing SQLite Storage: %s", dbPath)
		sStore, err := repository.NewSQLiteStorage(dbPath)
		if err != nil {
			log.Fatalf("Fatal: Gagal inisialisasi SQLite: %v", err)
		}
		store = sStore
	} else {
		defaultPG := "postgres://isp_admin:IspsyncPgPass2026!Sec@127.0.0.1:5432/ispsync?sslmode=disable"
		log.Printf("[ISPSYNC CORE] Attempting default PostgreSQL connection (%s)...", "127.0.0.1:5432/ispsync")
		pgStore, err := repository.NewPostgresStorage(defaultPG)
		if err == nil {
			store = pgStore
		} else {
			log.Printf("[ISPSYNC CORE] Default PostgreSQL unavailable (%v), falling back to SQLite...", err)
			dbPath = "ispsync.db"
			sStore, sErr := repository.NewSQLiteStorage(dbPath)
			if sErr != nil {
				log.Fatalf("Fatal: Gagal inisialisasi database: %v", sErr)
			}
			store = sStore
		}
	}
	defer store.Close()

	// 2. Inisialisasi Handlers
	sessionSecret := auth.LoadSecret()
	apiH := handler.NewAPIHandler(store, sessionSecret)
	pageH := handler.NewPageHandler(store)

	// 3. Router Setup
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Tenant-Slug"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Multi-Tenant Context Middleware
	r.Use(middleware.TenantResolver(store, baseDomain))

	// API Endpoints
	// Middleware otorisasi (sesi staf bertanda tangan; wajib milik tenant yang diakses)
	adminOnly := middleware.RequireRoles(sessionSecret, false, "OWNER", "NOC")
	financeStaff := middleware.RequireRoles(sessionSecret, false, "OWNER", "NOC", "FINANCE")
	anyStaff := middleware.RequireRoles(sessionSecret, false, "OWNER", "NOC", "FINANCE", "SALES", "TECHNICIAN")
	fieldStaff := middleware.RequireRoles(sessionSecret, false, "OWNER", "NOC", "TECHNICIAN")
	ownerOnly := middleware.RequireRoles(sessionSecret, false, "OWNER")
	platformOwner := middleware.RequireRoles(sessionSecret, true, "OWNER")

	r.Route("/api/v1", func(api chi.Router) {
		// â”€â”€ PUBLIK (tanpa login) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
		// Caddy On-Demand TLS Permission Verification Hook
		api.Get("/caddy/ask", apiH.CaddyAsk)

		api.Get("/context", apiH.GetContext)
		api.Get("/plans", apiH.ListPlans)
		api.Get("/odps", apiH.ListODPs)
		api.Post("/coverage-check", apiH.CheckCoverage)
		api.Post("/register", apiH.Register)
		api.Get("/registrations/{regNo}", apiH.PublicTrack)

		// â”€â”€ PENGATURAN TENANT (OWNER/NOC) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
		api.With(adminOnly).Post("/tenant/custom-domain", apiH.UpdateCustomDomain)
		api.With(adminOnly).Post("/tenant/profile", apiH.UpdateTenantProfile)
		api.With(adminOnly).Post("/tenant/mikrotik/generate", apiH.GenerateMikrotikVPN)
		api.With(adminOnly).Post("/tenant/hotspot/generate", apiH.GenerateHotspotConfig)
		api.With(adminOnly).Post("/tenant/isolir/generate", apiH.GenerateIsolirScript)
		api.With(adminOnly).Post("/tenant/mikrotik/kick", apiH.KickSubscriber)
		api.With(adminOnly).Get("/olts", apiH.ListOLTs)

		// Invoices & Billing
		api.With(financeStaff).Get("/invoices", apiH.ListInvoices)
		api.With(financeStaff).Post("/invoices/{id}/pay", apiH.PayInvoice)

		// Vouchers & Hotspot POS Blanko
		api.With(financeStaff).Get("/vouchers", apiH.ListVouchers)
		api.With(financeStaff).Post("/vouchers/generate", apiH.GenerateVouchers)

		// Subscriber Management (memuat kredensial PPPoE -> hanya OWNER/NOC)
		api.With(adminOnly).Get("/subscribers", apiH.ListSubscribers)
		api.With(adminOnly).Get("/subscribers/{id}", apiH.GetSubscriber)
		api.With(adminOnly).Post("/subscribers/{id}/status", apiH.UpdateSubscriberStatus)
		api.With(adminOnly).Post("/subscribers/{id}/provision", apiH.ProvisionSubscriber)
		api.With(adminOnly).Get("/subscribers/{id}/ont", apiH.SubscriberONT)
		api.With(adminOnly).Post("/subscribers/{id}/ont/reboot", apiH.SubscriberONTReboot)
		api.With(adminOnly).Get("/subscribers/{id}/ont/wifi", apiH.SubscriberONTWifi)
		api.With(adminOnly).Put("/subscribers/{id}/ont/wifi", apiH.SubscriberONTWifiUpdate)

		// Work Orders / SPK & BAST
		api.With(anyStaff).Get("/work-orders", apiH.ListWorkOrders)
		api.With(fieldStaff).Post("/work-orders/{id}/bast", apiH.CompleteBAST)

		// Jartaplok Sharing Agreements
		api.With(adminOnly).Get("/jartaplok/agreements", apiH.ListJartaplokAgreements)
	api.With(ownerOnly).Post("/jartaplok/agreements", apiH.CreateJartaplokAgreement)
	api.With(ownerOnly).Post("/jartaplok/agreements/{id}/odps", apiH.LeaseJartaplokODP)
	api.With(ownerOnly).Post("/jartaplok/agreements/{id}/status", apiH.SetJartaplokAgreementStatus)
	api.With(adminOnly).Get("/tenant/capabilities", apiH.GetCapabilities)

		// â”€â”€ GOGIGANET PORTAL COMPATIBLE SUB-ROUTES â”€â”€â”€â”€â”€â”€â”€â”€â”€
		api.Route("/public", func(pub chi.Router) {
			pub.Post("/coverage-check", apiH.PublicCoverageCheck)
			pub.Get("/plans", apiH.PublicPlans)
			pub.Post("/register", apiH.PublicRegister)
			pub.Get("/odps", apiH.PublicODPs)
			pub.Get("/clusters", apiH.PublicClusters)
			pub.Get("/track/{regNo}", apiH.PublicTrack)
			pub.Post("/track/{regNo}/ktp", apiH.PublicTrackKTP)
			pub.Post("/track/{regNo}/sign-contract", apiH.PublicTrackSignContract)
			pub.Get("/referral/check", apiH.PublicReferralCheck)
		})

		api.Route("/auth", func(a chi.Router) {
			a.Post("/login", apiH.AuthLogin)
			a.Get("/me", apiH.AuthMe)
			a.Post("/logout", apiH.AuthLogout)
		})

		api.Route("/admin", func(adm chi.Router) {
			adm.Use(adminOnly)
			adm.Get("/registrations", apiH.AdminListRegistrations)
			adm.Get("/odps", apiH.AdminListODPs)
			adm.Post("/odps", apiH.AdminCreateODP)
			adm.Delete("/odps/{id}", apiH.AdminDeleteODP)
			adm.Post("/odps/{id}/delete", apiH.AdminDeleteODP)
			adm.Get("/work-orders", apiH.AdminListWorkOrders)
			adm.Get("/clusters", apiH.AdminListClusters)
			adm.Get("/staff-kpi", apiH.AdminStaffKPI)
			adm.Get("/radius/live-sessions", apiH.AdminRadiusLiveSessions)
		})

		api.Route("/technician", func(tech chi.Router) {
			tech.Use(fieldStaff)
			tech.Get("/work-orders", apiH.TechnicianListWorkOrders)
			tech.Get("/work-orders/{id}", apiH.TechnicianGetWorkOrder)
			tech.Post("/work-orders/{id}/bast", apiH.TechnicianCompleteBAST)
		})

		api.Route("/partner", func(part chi.Router) {
			part.Use(adminOnly)
			part.Route("/jartaplok", func(j chi.Router) {
				j.Get("/billing", apiH.JartaplokBilling)
				j.Get("/odps", apiH.JartaplokODPs)
				j.Post("/odps", apiH.SyncODPFromFiberGrid)
				j.Post("/upload-kml", apiH.UploadJartaplokKML)
				j.Get("/ports", apiH.JartaplokPorts)
			})
			part.Get("/registrations", apiH.AdminListRegistrations)
			part.Get("/registrations/{regNo}", apiH.PublicTrack)
		})

		api.Route("/superuser", func(sup chi.Router) {
			sup.Use(ownerOnly)
			sup.Get("/overview", apiH.SuperuserOverview)
			sup.Get("/jartaplok-partners", apiH.SuperuserJartaplokPartners)
			sup.Get("/staff", apiH.SuperuserListStaff)
			sup.Post("/staff", apiH.CreateStaffUser)
			sup.Put("/staff/{id}", apiH.SuperuserUpdateStaff)
			sup.Post("/staff/reset-password", apiH.SuperuserResetStaffPassword)
			sup.With(platformOwner).Put("/tenants/{slug}/capabilities", apiH.SuperuserSetCapabilities)
		})

		// Staff Quota & Add-on Management
		api.Route("/staff", func(st chi.Router) {
			st.Use(ownerOnly)
			st.Get("/quota", apiH.GetStaffQuota)
			st.Get("/users", apiH.ListStaffUsers)
			st.Post("/users", apiH.CreateStaffUser)
		})
		api.Route("/addons", func(ad chi.Router) {
			ad.Use(ownerOnly)
			ad.Get("/", apiH.ListAddons)
			ad.Post("/purchase", apiH.PurchaseAddon)
		})
	})


	// Static Assets (Logo, Documents, Web files)
	r.Get("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/logo.png")
	})
	r.Get("/FORM_SCORECARD_EVALUASI_KPI_BULANAN_GOGIGANET.pdf", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/FORM_SCORECARD_EVALUASI_KPI_BULANAN_GOGIGANET.pdf")
	})
	r.Get("/MASTER_SOP_EKOSISTEM_3_ENGINE_GOGIGANET.pdf", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/MASTER_SOP_EKOSISTEM_3_ENGINE_GOGIGANET.pdf")
	})
	r.Get("/MATRIKS_TUGAS_DAN_TANGGUNG_JAWAB_SDM_GOGIGANET.pdf", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/MATRIKS_TUGAS_DAN_TANGGUNG_JAWAB_SDM_GOGIGANET.pdf")
	})
	r.Get("/PROSEDUR_PENDAFTARAN_PELANGGAN_BARU.pdf", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/PROSEDUR_PENDAFTARAN_PELANGGAN_BARU.pdf")
	})
	r.Get("/SOP_OPERASIONAL_LENGKAP_GOGIGANET.pdf", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/SOP_OPERASIONAL_LENGKAP_GOGIGANET.pdf")
	})
	
	// NOC Telco Preset Captive Portal & Isolir
	r.Get("/hotspot", pageH.ServeHotspot)
	r.Get("/isolir", pageH.ServeIsolir)
	
	r.Handle("/web/*", http.StripPrefix("/web/", http.FileServer(http.Dir("web"))))

	// Web UI Multi-Tenant (CMS, Portal, NOC, Sales, Teknisi)
	r.Get("/*", pageH.ServeApp)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("[ISPSYNC CORE] Server listening on http://localhost:%s", port)
		log.Printf("[ISPSYNC CORE] Testing link: http://localhost:%s/?tenant=ispku&app=cms", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[ISPSYNC CORE] Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("[ISPSYNC CORE] Stopped.")
}
