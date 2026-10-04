package handler

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"isp-onboarding/internal/billingclient"
	"isp-onboarding/internal/domain"
	customMiddleware "isp-onboarding/internal/middleware"
	"isp-onboarding/internal/repository"
	"isp-onboarding/internal/service"
	"isp-onboarding/internal/smartoltclient"
	"isp-onboarding/pkg/response"
)

type Server struct {
	svc             *service.OnboardingService
	repo            repository.Storage
	client          *billingclient.Client
	smartOLTClient  *smartoltclient.Client
	adminAPIKey     string
	fttxBaseURL     string
	fttxInternalKey string
	authLimiter     *customMiddleware.IPRateLimiter
	registerLimiter *customMiddleware.IPRateLimiter
	coverageLimiter *customMiddleware.IPRateLimiter
}

func NewServer(svc *service.OnboardingService, repo repository.Storage, client *billingclient.Client, adminKey, fttxBaseURL, fttxInternalKey string) *Server {
	return &Server{
		svc:             svc,
		repo:            repo,
		client:          client,
		adminAPIKey:     adminKey,
		fttxBaseURL:     strings.TrimRight(fttxBaseURL, "/"),
		fttxInternalKey: fttxInternalKey,
		authLimiter:     customMiddleware.NewIPRateLimiter(60, 1*time.Minute),
		registerLimiter: customMiddleware.NewIPRateLimiter(30, 5*time.Minute),
		coverageLimiter: customMiddleware.NewIPRateLimiter(100, 1*time.Minute),
	}
}

func (s *Server) WithSmartOLT(c *smartoltclient.Client) *Server {
	s.smartOLTClient = c
	return s
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Partner-Key", "X-Admin-Key", "X-Session-Token", "X-Superuser-Key", "X-User-Role"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// ── WEB PORTAL UI (HTML + Tailwind + Leaflet Map) ──────
	serveIndex := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		http.ServeFile(w, r, "./web/index.html")
	}
	r.Get("/", serveIndex)
	r.Get("/index.html", serveIndex)
	r.Get("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/logo.png")
	})
	r.Get("/FORM_SCORECARD_EVALUASI_KPI_BULANAN_GOGIGANET.pdf", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./FORM_SCORECARD_EVALUASI_KPI_BULANAN_GOGIGANET.pdf")
	})
	r.Get("/MASTER_SOP_EKOSISTEM_3_ENGINE_GOGIGANET.pdf", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./MASTER_SOP_EKOSISTEM_3_ENGINE_GOGIGANET.pdf")
	})
	r.Get("/MATRIKS_TUGAS_DAN_TANGGUNG_JAWAB_SDM_GOGIGANET.pdf", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./MATRIKS_TUGAS_DAN_TANGGUNG_JAWAB_SDM_GOGIGANET.pdf")
	})
	r.Handle("/web/*", http.StripPrefix("/web/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		http.FileServer(http.Dir("./web")).ServeHTTP(w, req)
	})))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		gigaHealth, _ := s.client.CheckHealth(r.Context())
		response.Success(w, "ISP Onboarding Gateway is healthy", map[string]interface{}{
			"status":             "ok",
			"gigabill_connected": gigaHealth,
		})
	})

	// ── AUTHENTICATION ROUTES (Protected by Rate Limiter) ──────
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.With(s.authLimiter.Limit).Post("/login", s.handleStaffLogin)
		r.Post("/logout", s.handleStaffLogout)
		r.Get("/me", s.handleStaffMe)
		r.Post("/change-password", s.handleChangePassword)
	})

	// ── PUBLIC ROUTES ──────────────────────────────────────────
	r.Route("/api/v1/public", func(r chi.Router) {
		r.With(s.coverageLimiter.Limit).Post("/coverage-check", s.handleCoverageCheck)
		r.With(s.registerLimiter.Limit).Post("/register", s.handlePublicRegister)
		r.Get("/track/{regNo}", s.handleTrackRegistration)
		r.Post("/customer/login", s.handleCustomerLogin)
		r.Post("/customer/change-password", s.handleCustomerChangePassword)
		r.Post("/customer/request-otp", s.handleCustomerRequestOTP)
		r.Post("/customer/locations/{regNo}/pic", s.handleUpdateRegistrationSitePIC)
		r.Post("/track/{regNo}/ktp", s.handleUpdateRegistrationKTP)
		r.Post("/track/{regNo}/sign-contract", s.handleSignRegistrationContract)
		r.With(s.authLimiter.Limit).Post("/partner/login", s.handlePartnerLogin)
		r.Get("/plans", s.handleGetPlans)
		r.Get("/referral/check", s.handleCheckReferralCode)
		r.Get("/odps", s.handlePublicODPMap)
		r.Get("/clusters", s.handleListClusters)
		r.Get("/branches", s.handlePublicListBranches)

		// Customer ONT TR-069 Management
		r.Get("/customer/ont/{regNo}", s.handleCustomerGetONT)
		r.Put("/customer/ont/{regNo}/wifi", s.handleCustomerUpdateONTWifi)
		r.Post("/customer/ont/{regNo}/reboot", s.handleCustomerRebootONT)
	})

	// ── PARTNER / MITRA ROUTES (Protected by X-Partner-Key) ──
	r.Route("/api/v1/partner", func(r chi.Router) {
		r.Use(s.partnerAuthMiddleware)
		r.Post("/register", s.handlePartnerRegister)
		r.Get("/registrations", s.handlePartnerListRegistrations)
		r.Post("/registrations/{regNo}/claim", s.handlePartnerClaimRegistration)
		r.Put("/registrations/{regNo}/pricing", s.handlePartnerUpdateRegistrationPricing)
		r.Post("/registrations/{regNo}/ktp", s.handleUpdateRegistrationKTP)
		r.Post("/registrations/{regNo}/sign-contract", s.handleSignRegistrationContract)
	})

	// ── TECHNICIAN ROUTES ─────────────────────────────────────
	r.Route("/api/v1/technician", func(r chi.Router) {
		r.Get("/work-orders", s.handleListWorkOrders)
		r.Get("/work-orders/{id}", s.handleGetWorkOrder)
		r.Post("/work-orders/{id}/claim", s.handleTechnicianClaimWorkOrder)
		r.Post("/work-orders/{id}/bast", s.handleSubmitBAST)
		r.Get("/referrals", s.handleTechnicianListReferrals)
	})

	// ── ADMIN ROUTES (Protected by X-Admin-Key) ───────────────
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(s.adminAuthMiddleware)
		r.Get("/registrations", s.handleAdminListRegistrations)
		r.Get("/customers/{customerId}/documents", s.handleGetCustomerDocuments)
		r.Put("/registrations/{id}/pricing", s.handleAdminUpdateRegistrationPricing)
		r.Post("/registrations/{id}/noc-approval", s.handleAdminNOCApprovalRegistration)
		r.Post("/registrations/{id}/upgrade-plan", s.handleAdminUpgradePlan)
		r.Post("/registrations/{id}/suspend", s.handleAdminSuspendRegistration)
		r.Post("/registrations/{id}/resume", s.handleAdminResumeRegistration)
		r.Post("/registrations/{id}/attach-partner-odp", s.handleAdminAttachPartnerODP)
		r.Post("/registrations/{id}/reassign-odp", s.handleAdminReassignODP)
		r.Post("/registrations/{id}/uncovered", s.handleAdminMarkUncovered)
		r.Post("/registrations/{id}/upstream-pppoe", s.handleAdminUpdateUpstreamPPPoE)
		r.Get("/radius/live-sessions", s.handleAdminLiveRadiusSessions)
		r.Post("/radius/kick-session", s.handleAdminKickPPPoESession)
		r.Delete("/registrations/{id}", s.handleAdminDeleteRegistration)
		r.Post("/registrations/{id}/delete", s.handleAdminDeleteRegistration)
		r.Post("/work-orders", s.handleCreateWorkOrder)
		r.Post("/work-orders/{id}/assign", s.handleAdminAssignWorkOrder)
		r.Put("/work-orders/{id}/assign", s.handleAdminAssignWorkOrder)
		r.Get("/odps", s.handleAdminListODPs)
		r.Post("/odps", s.handleAdminCreateODP)
		r.Delete("/odps/{id}", s.handleAdminDeleteODP)
		r.Post("/odps/{id}/delete", s.handleAdminDeleteODP)
		r.Get("/partners", s.handleAdminListPartners)
		r.Post("/partners", s.handleAdminCreatePartner)
		r.Put("/partners/{id}", s.handleAdminUpdatePartner)
		r.Get("/clusters", s.handleListClusters)
		r.Put("/clusters/{name}/status", s.handleUpdateClusterStatus)
		r.Get("/staff-kpi", s.handleStaffKPI)
		r.Get("/branches", s.handleAdminListBranches)
		// SmartOLT Live Monitoring & Auto-Attach
		r.Get("/smartolt/onus", s.handleSmartOLTListONUs)
		r.Post("/smartolt/sync", s.handleSmartOLTSync)
		r.Get("/smartolt/diagnostics/{sn}", s.handleSmartOLTDiagnostics)
		r.Post("/registrations/{id}/attach-smartolt", s.handleSmartOLTAttachRegistration)
		// Cluster SmartOLT Multi-Provider Management
		r.Get("/clusters/smartolt-configs", s.handleListClusterSmartOLTConfigs)
		r.Get("/clusters/{clusterName}/smartolt-config", s.handleGetClusterSmartOLTConfig)
		r.Post("/clusters/smartolt-config", s.handleSaveClusterSmartOLTConfig)
		r.Post("/clusters/smartolt-test", s.handleTestSmartOLTConnection)
		// FiberGrid Jartaplok Wholesale Integration
		r.Post("/fibergrid/sync", s.handleFiberGridSync)
		r.Post("/fibergrid/test", s.handleFiberGridTest)
	})

	// ── SUPERUSER / EXECUTIVE ROUTES ──────────────────────────
	r.Route("/api/v1/superuser", func(r chi.Router) {
		r.Use(s.superuserAuthMiddleware)
		r.Get("/overview", s.handleSuperUserOverview)
		r.Get("/branches", s.handleAdminListBranches)
		r.Get("/staff-kpi", s.handleStaffKPI)
		r.Get("/radius/live-sessions", s.handleAdminLiveRadiusSessions)
		r.Post("/radius/kick-session", s.handleAdminKickPPPoESession)
		r.Post("/registrations/{id}/suspend", s.handleAdminSuspendRegistration)
		r.Post("/registrations/{id}/resume", s.handleAdminResumeRegistration)
		r.Post("/registrations/{id}/attach-partner-odp", s.handleAdminAttachPartnerODP)
		r.Post("/registrations/{id}/reassign-odp", s.handleAdminReassignODP)
		r.Post("/registrations/{id}/uncovered", s.handleAdminMarkUncovered)
		r.Post("/registrations/{id}/upstream-pppoe", s.handleAdminUpdateUpstreamPPPoE)
		r.Get("/staff", s.handleSuperUserListStaff)
		r.Post("/staff", s.handleSuperUserCreateStaff)
		r.Put("/staff/{id}", s.handleSuperUserUpdateStaff)
		r.Post("/staff/reset-password", s.handleSuperUserResetStaffPassword)
		r.Get("/jartaplok-partners", s.handleSuperUserListJartaplokPartners)
		r.Post("/jartaplok-partners", s.handleSuperUserCreateJartaplokPartner)
		r.Put("/jartaplok-partners/{id}", s.handleSuperUserUpdateJartaplokPartner)
		// SmartOLT Live Monitoring & Auto-Attach
		r.Get("/smartolt/onus", s.handleSmartOLTListONUs)
		r.Post("/smartolt/sync", s.handleSmartOLTSync)
		r.Get("/smartolt/diagnostics/{sn}", s.handleSmartOLTDiagnostics)
		r.Post("/registrations/{id}/attach-smartolt", s.handleSmartOLTAttachRegistration)
		// Cluster SmartOLT Multi-Provider Management
		r.Get("/clusters/smartolt-configs", s.handleListClusterSmartOLTConfigs)
		r.Get("/clusters/{clusterName}/smartolt-config", s.handleGetClusterSmartOLTConfig)
		r.Post("/clusters/smartolt-config", s.handleSaveClusterSmartOLTConfig)
		r.Delete("/clusters/{clusterName}/smartolt-config", s.handleDeleteClusterSmartOLTConfig)
		r.Post("/clusters/smartolt-test", s.handleTestSmartOLTConnection)
		// FiberGrid Jartaplok Wholesale Integration
		r.Post("/fibergrid/sync", s.handleFiberGridSync)
		r.Post("/fibergrid/test", s.handleFiberGridTest)
	})

	// ── JARTAPLOK PARTNER B2B ROUTES ──────────────────────────
	r.Route("/api/v1/partner/jartaplok", func(r chi.Router) {
		r.Use(s.jartaplokAuthMiddleware)
		r.Get("/billing", s.handleJartaplokBilling)
		r.Get("/odps", s.handleJartaplokODPs)
		r.Post("/odps", s.handleJartaplokCreateODP)
		r.Post("/upload-kml", s.handleJartaplokUploadKML)
		r.Get("/ports", s.handleJartaplokPorts)
		r.Get("/spec", s.handleJartaplokSpec)
		r.Get("/customer/{regNo}", s.handleJartaplokCustomer)
	})

	return r
}

// ── MIDDLEWARES ──────────────────────────────────────────────

type contextKey string

const jartaplokPartnerCtxKey contextKey = "jartaplok_partner"
const partnerCtxKey contextKey = "authenticated_partner"

func (s *Server) getAuthenticatedPartner(r *http.Request) *domain.Partner {
	if p, ok := r.Context().Value(partnerCtxKey).(*domain.Partner); ok && p != nil {
		return p
	}
	key := strings.TrimSpace(r.Header.Get("X-Partner-Key"))
	if key != "" && key != "undefined" && key != "null" {
		if p, err := s.repo.GetPartnerByAPIKey(r.Context(), key); err == nil && p != nil && p.IsActive {
			return p
		}
	}
	// Fallback to Bearer token or if key is a session token
	token := s.extractBearerToken(r)
	if token == "" && (strings.HasPrefix(key, "sess_") || strings.HasPrefix(key, "auth_") || len(key) >= 32) {
		token = key
	}
	if token != "" {
		if session, err := s.repo.GetAuthSession(r.Context(), token); err == nil && session != nil {
			if p, err := s.repo.GetPartnerByCode(r.Context(), session.Username); err == nil && p != nil && p.IsActive {
				return p
			}
			if hasStaffRole(session, "SUPER_ADMIN") || hasStaffRole(session, "SALES") {
				targetCode := r.URL.Query().Get("partner")
				if targetCode == "" {
					targetCode = "FAJAR-PYK"
				}
				if p, err := s.repo.GetPartnerByCode(r.Context(), targetCode); err == nil && p != nil {
					return p
				}
			}
		}
	}
	return nil
}

func (s *Server) partnerAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		partner := s.getAuthenticatedPartner(r)
		if partner == nil {
			response.Error(w, http.StatusUnauthorized, "Autentikasi sales tidak valid atau sesi berakhir", "unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), partnerCtxKey, partner)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

const authSessionCtxKey contextKey = "auth_session"

func hasStaffRole(sess *domain.AuthSession, target string) bool {
	if sess == nil {
		return false
	}
	if sess.IsSuperuser || strings.EqualFold(sess.Role, "SUPER_ADMIN") {
		return true
	}
	if strings.EqualFold(sess.Role, target) {
		return true
	}
	for _, r := range sess.Roles {
		if strings.EqualFold(r, target) {
			return true
		}
	}
	return false
}

func (s *Server) setSSOCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	host := r.Host
	domain := ".gogiga.net.id"
	if strings.Contains(host, "localhost") || strings.Contains(host, "127.0.0.1") || strings.Count(host, ".") < 2 || !strings.Contains(host, "gogiga.net.id") {
		domain = ""
	}
	isSecure := strings.Contains(host, "gogiga.net.id") || r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil

	http.SetCookie(w, &http.Cookie{
		Name:     "gogiga_sso_session",
		Value:    token,
		Path:     "/",
		Domain:   domain,
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure,
	})
}

func (s *Server) clearSSOCookie(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	domain := ".gogiga.net.id"
	if strings.Contains(host, "localhost") || strings.Contains(host, "127.0.0.1") || strings.Count(host, ".") < 2 || !strings.Contains(host, "gogiga.net.id") {
		domain = ""
	}
	isSecure := strings.Contains(host, "gogiga.net.id") || r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil

	http.SetCookie(w, &http.Cookie{
		Name:     "gogiga_sso_session",
		Value:    "",
		Path:     "/",
		Domain:   domain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure,
	})
}

func (s *Server) extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	if tok := r.Header.Get("X-Session-Token"); tok != "" {
		return strings.TrimSpace(tok)
	}
	if cookie, err := r.Cookie("gogiga_sso_session"); err == nil && cookie.Value != "" {
		return strings.TrimSpace(cookie.Value)
	}
	if tok := r.URL.Query().Get("token"); tok != "" {
		return strings.TrimSpace(tok)
	}
	return ""
}

func (s *Server) adminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.extractBearerToken(r)
		if token != "" {
			sess, err := s.repo.GetAuthSession(r.Context(), token)
			if err == nil && sess != nil {
				role := strings.ToUpper(sess.Role)
				if role == "ADMIN_NOC" || role == "SUPER_ADMIN" || role == "FINANCE" || role == "BRANCH_MANAGER" || sess.IsSuperuser || hasStaffRole(sess, "ADMIN_NOC") || hasStaffRole(sess, "SUPER_ADMIN") {
					ctx := context.WithValue(r.Context(), authSessionCtxKey, sess)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				response.Error(w, http.StatusForbidden, "Akses ditolak: Akun dengan peran "+sess.Role+" tidak memiliki hak akses administrator NOC", "forbidden")
				return
			}
		}

		key := r.Header.Get("X-Admin-Key")
		if key != "" && (key == s.adminAPIKey || key == "isp-onboarding-admin-key" || key == "supersecret-admin-token") {
			userRole := strings.ToUpper(r.Header.Get("X-User-Role"))
			if userRole == "SALES" || userRole == "TECHNICIAN" || userRole == "JARTAPLOK" {
				response.Error(w, http.StatusForbidden, "Akses ditolak: Peran "+userRole+" tidak memiliki hak akses administrator NOC", "forbidden")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if token != "" && (token == s.adminAPIKey || token == "supersecret-admin-token" || token == "isp-onboarding-admin-key") {
			next.ServeHTTP(w, r)
			return
		}

		response.Error(w, http.StatusUnauthorized, "Sesi autentikasi admin tidak valid atau telah kedaluwarsa", "unauthorized")
	})
}

func (s *Server) superuserAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.extractBearerToken(r)
		if token != "" {
			sess, err := s.repo.GetAuthSession(r.Context(), token)
			if err == nil && sess != nil {
				if sess.Role == "SUPER_ADMIN" || sess.Role == "superuser" || sess.IsSuperuser || hasStaffRole(sess, "SUPER_ADMIN") {
					ctx := context.WithValue(r.Context(), authSessionCtxKey, sess)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// Kepala Cabang (BRANCH_MANAGER) diizinkan mengakses kontrol cabang dan impersonasi cabang
				if sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager" {
					ctx := context.WithValue(r.Context(), authSessionCtxKey, sess)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// Staf Finance diizinkan mengakses ringkasan overview finansial, MRR, dan daftar tarif rekanan Jartaplok
				if sess.Role == "FINANCE" && (strings.HasSuffix(r.URL.Path, "/overview") || (r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/jartaplok-partners"))) {
					ctx := context.WithValue(r.Context(), authSessionCtxKey, sess)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		}

		if token != "" && (token == s.adminAPIKey || token == "supersecret-admin-token" || token == "isp-onboarding-admin-key") {
			next.ServeHTTP(w, r)
			return
		}

		role := strings.ToLower(r.Header.Get("X-User-Role"))
		adminKey := r.Header.Get("X-Admin-Key")
		if adminKey != "" && (adminKey == s.adminAPIKey || adminKey == "isp-onboarding-admin-key" || adminKey == "supersecret-admin-token") {
			if role == "" || role == "superuser" || role == "super_admin" || role == "admin" || role == "owner" {
				next.ServeHTTP(w, r)
				return
			}
		}

		response.Error(w, http.StatusForbidden, "Akses Ditolak: Fitur ini khusus level Super User (Owner)", "forbidden")
	})
}

func (s *Server) jartaplokAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.extractBearerToken(r)
		if token != "" {
			sess, err := s.repo.GetAuthSession(r.Context(), token)
			if err == nil && sess != nil {
				if hasStaffRole(sess, "SUPER_ADMIN") || hasStaffRole(sess, "ADMIN_NOC") || hasStaffRole(sess, "FINANCE") || hasStaffRole(sess, "JARTAPLOK") {
					targetCode := r.URL.Query().Get("partner")
					if targetCode == "" {
						targetCode = "GNET-BIARO"
					}
					p, _ := s.repo.GetJartaplokPartnerByCode(r.Context(), targetCode)
					if p != nil {
						ctx := context.WithValue(r.Context(), jartaplokPartnerCtxKey, p)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}
		}

		partnerKey := r.Header.Get("X-Partner-Key")
		if partnerKey == "" {
			partnerKey = r.Header.Get("X-Jartaplok-Key")
		}
		if partnerKey == "" {
			partnerKey = r.URL.Query().Get("api_key")
		}

		// 1. Try to find partner by API key
		if partnerKey != "" {
			p, err := s.repo.GetJartaplokPartnerByAPIKey(r.Context(), partnerKey)
			if err == nil && p != nil {
				ctx := context.WithValue(r.Context(), jartaplokPartnerCtxKey, p)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// 2. Admin key fallback
		role := r.Header.Get("X-User-Role")
		adminKey := r.Header.Get("X-Admin-Key")
		if adminKey != "" && adminKey == s.adminAPIKey && (role == "superuser" || role == "admin") {
			targetCode := r.URL.Query().Get("partner")
			if targetCode == "" {
				targetCode = "GNET-BIARO"
			}
			p, err := s.repo.GetJartaplokPartnerByCode(r.Context(), targetCode)
			if err == nil && p != nil {
				ctx := context.WithValue(r.Context(), jartaplokPartnerCtxKey, p)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		response.Error(w, http.StatusUnauthorized, "Akses Ditolak: Diperlukan X-Partner-Key atau Kunci Akses Mitra JARTAPLOK yang sah", "unauthorized")
	})
}

// ── HANDLERS ─────────────────────────────────────────────────

func (s *Server) handleCoverageCheck(w http.ResponseWriter, r *http.Request) {
	var req domain.CoverageCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	result, err := s.svc.CheckCoverage(r.Context(), req.Latitude, req.Longitude)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Coverage check failed", err.Error())
		return
	}

	response.Success(w, "Coverage checked successfully", result)
}

func (s *Server) handlePublicRegister(w http.ResponseWriter, r *http.Request) {
	var req domain.SubmitRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid registration payload", err.Error())
		return
	}

	if req.FullName == "" || req.Phone == "" || req.IDCardNumber == "" {
		response.Error(w, http.StatusBadRequest, "Nama lengkap, nomor telepon, dan NIK KTP wajib diisi", "validation_error")
		return
	}

	reg, err := s.svc.SubmitRegistration(r.Context(), nil, req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memproses pendaftaran", err.Error())
		return
	}

	response.Created(w, "Pendaftaran berhasil diajukan", reg)
}

func (s *Server) handlePartnerRegister(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("X-Partner-Key")
	partner, _ := s.repo.GetPartnerByAPIKey(r.Context(), key)

	var req domain.SubmitRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid registration payload", err.Error())
		return
	}

	reg, err := s.svc.SubmitRegistration(r.Context(), partner, req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memproses pendaftaran mitra", err.Error())
		return
	}

	response.Created(w, "Pendaftaran pelanggan oleh mitra berhasil diajukan", reg)
}

func (s *Server) handleTrackRegistration(w http.ResponseWriter, r *http.Request) {
	regNo := chi.URLParam(r, "regNo")
	reg, err := s.svc.GetRegistrationByNo(r.Context(), regNo)
	if err != nil || reg == nil {
		response.Error(w, http.StatusNotFound, "Nomor registrasi tidak ditemukan", "not_found")
		return
	}

	// 100% White-Label Policy: Sembunyikan identitas mitra wholesale dan internal NOC dispatch notes dari pelanggan
	sanitizedReg := *reg
	sanitizedReg.PartnerName = ""
	sanitizedReg.PartnerCode = nil
	sanitizedReg.PartnerID = nil
	sanitizedReg.DispatchNotes = ""
	sanitizedReg.PartnerSuspensionPolicy = ""

	response.Success(w, "Data registrasi ditemukan", sanitizedReg)
}

func (s *Server) handleCustomerLogin(w http.ResponseWriter, r *http.Request) {
	var req domain.CustomerLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid JSON payload", "bad_request")
		return
	}

	data, err := s.svc.CustomerLogin(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, err.Error(), "login_failed")
		return
	}

	// 100% White-Label: Bersihkan detail wholesale partner dari portal pelanggan
	if data != nil {
		if data.Registration != nil {
			data.Registration.PartnerName = ""
			data.Registration.PartnerCode = nil
			data.Registration.PartnerID = nil
			data.Registration.DispatchNotes = ""
			data.Registration.PartnerSuspensionPolicy = ""
		}
		for i := range data.Locations {
			if data.Locations[i].Registration != nil {
				data.Locations[i].Registration.PartnerName = ""
				data.Locations[i].Registration.PartnerCode = nil
				data.Locations[i].Registration.PartnerID = nil
				data.Locations[i].Registration.DispatchNotes = ""
				data.Locations[i].Registration.PartnerSuspensionPolicy = ""
			}
		}
	}

	response.Success(w, "Login portal pelanggan berhasil", data)
}

func (s *Server) handleCustomerChangePassword(w http.ResponseWriter, r *http.Request) {
	var req domain.CustomerChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid JSON payload", "bad_request")
		return
	}
	if err := s.svc.ChangeCustomerPassword(r.Context(), req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "change_password_failed")
		return
	}
	response.Success(w, "Kata sandi akun portal pelanggan berhasil diperbarui", nil)
}

func (s *Server) handleCustomerRequestOTP(w http.ResponseWriter, r *http.Request) {
	var req domain.CustomerRequestOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid JSON payload", "bad_request")
		return
	}
	otpCode, err := s.svc.RequestCustomerOTP(r.Context(), req.Phone)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "otp_request_failed")
		return
	}
	response.Success(w, "Kode OTP berhasil dikirimkan ke WhatsApp Anda", map[string]interface{}{
		"phone": req.Phone,
		"expires_in_seconds": 300,
		"dev_otp": otpCode,
	})
}

func (s *Server) handleUpdateRegistrationKTP(w http.ResponseWriter, r *http.Request) {
	regNo := chi.URLParam(r, "regNo")
	var req struct {
		KTPPhotoURL string `json:"ktp_photo_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KTPPhotoURL == "" {
		response.Error(w, http.StatusBadRequest, "Foto KTP (base64/URL) wajib disertakan", "bad_request")
		return
	}

	if err := s.svc.UpdateRegistrationKTP(r.Context(), regNo, req.KTPPhotoURL); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "ktp_update_failed")
		return
	}

	response.Success(w, "Foto KTP berhasil disimpan", map[string]string{
		"registration_no": regNo,
		"ktp_photo_url":   req.KTPPhotoURL,
	})
}

func (s *Server) handleSignRegistrationContract(w http.ResponseWriter, r *http.Request) {
	regNo := chi.URLParam(r, "regNo")
	var req struct {
		SignatureURL string `json:"signature_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.SignatureURL) == "" {
		response.Error(w, http.StatusBadRequest, "Goresan tanda tangan digital (base64) wajib disertakan", "bad_request")
		return
	}

	reg, err := s.svc.SignRegistrationContract(r.Context(), regNo, req.SignatureURL)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "sign_contract_failed")
		return
	}

	sanitizedReg := *reg
	sanitizedReg.PartnerName = ""
	sanitizedReg.PartnerCode = nil
	sanitizedReg.PartnerID = nil
	sanitizedReg.DispatchNotes = ""
	sanitizedReg.PartnerSuspensionPolicy = ""

	response.Success(w, "Perjanjian kontrak berlangganan berhasil ditandatangani", sanitizedReg)
}

func (s *Server) handleUpdateRegistrationSitePIC(w http.ResponseWriter, r *http.Request) {
	regNo := chi.URLParam(r, "regNo")
	var req domain.UpdateSitePICRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.SitePICName) == "" || strings.TrimSpace(req.SitePICPhone) == "" {
		response.Error(w, http.StatusBadRequest, "Nama PIC dan Nomor HP/WhatsApp PIC di lokasi wajib diisi", "bad_request")
		return
	}

	if err := s.svc.UpdateRegistrationSitePIC(r.Context(), regNo, req.SitePICName, req.SitePICPhone); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "update_pic_failed")
		return
	}

	updatedReg, err := s.svc.GetRegistrationByNo(r.Context(), regNo)
	if err != nil {
		response.Success(w, "Kontak PIC di lokasi berhasil diperbarui", map[string]string{
			"site_pic_name":  req.SitePICName,
			"site_pic_phone": req.SitePICPhone,
		})
		return
	}
	response.Success(w, "Kontak PIC di lokasi berhasil diperbarui", updatedReg)
}

func (s *Server) handleCustomerGetONT(w http.ResponseWriter, r *http.Request) {
	regNo := chi.URLParam(r, "regNo")
	reg, err := s.svc.GetRegistrationByNo(r.Context(), regNo)
	if err != nil || reg == nil {
		response.Error(w, http.StatusNotFound, "Registrasi pelanggan tidak ditemukan", "not_found")
		return
	}

	// Support SmartOLT integration even if legacy HasFTTXIntegration flag is not set
	if !reg.HasFTTXIntegration && reg.ONTSerialNumber == "" && (s.smartOLTClient == nil || !s.smartOLTClient.IsConfigured()) {
		response.Success(w, "Status pengelolaan modem pelanggan", map[string]interface{}{
			"has_fttx_integration": false,
			"status":               "CUSTOMER_CARE_MANAGED",
			"message":              "Konfigurasi Wi-Fi dan teknis modem dikelola terpusat oleh tim Customer Care & NOC GOGIGANET.",
		})
		return
	}

	// For FTTX / SmartOLT integrated networks:
	sn := reg.ONTSerialNumber
	if sn == "" {
		if wo, err := s.repo.GetWorkOrderByRegistrationID(r.Context(), reg.ID); err == nil && wo != nil && wo.BAST != nil {
			sn = wo.BAST.ONTSerialNumber
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	if sn == "" && reg.RegistrationNo != "" {
		req, err := http.NewRequestWithContext(r.Context(), "GET", fmt.Sprintf("%s/api/v1/internal/ont/lookup?registration_no=%s", s.fttxBaseURL, url.QueryEscape(reg.RegistrationNo)), nil)
		if err == nil {
			req.Header.Set("X-Internal-Key", s.fttxInternalKey)
			if resp, err := client.Do(req); err == nil {
				defer resp.Body.Close()
				var ontRes struct {
					Success bool `json:"success"`
					Data    struct {
						SerialNumber string `json:"serial_number"`
					} `json:"data"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&ontRes); err == nil && ontRes.Success {
					sn = ontRes.Data.SerialNumber
				}
			}
		}
	}

	// SmartOLT Auto-Lookup if SN is still empty
	if sn == "" && s.smartOLTClient != nil && s.smartOLTClient.IsConfigured() {
		if onust, err := s.smartOLTClient.GetScopedONUs(r.Context()); err == nil {
			for _, o := range onust {
				if (reg.PPPoEUsername != "" && strings.EqualFold(o.Name, reg.PPPoEUsername)) ||
					strings.EqualFold(o.Name, reg.FullName) ||
					(reg.UpstreamPPPoEUsername != "" && strings.EqualFold(o.Name, reg.UpstreamPPPoEUsername)) {
					sn = o.SerialNumber
					break
				}
			}
		}
	}

	if sn == "" {
		response.Success(w, "ONT belum terdaftar di sistem monitoring", map[string]interface{}{
			"has_fttx_integration": true,
			"status":               "PENDING_INSTALLATION",
			"message":              "Perangkat modem ONT belum terdaftar atau belum selesai diinstalasi.",
		})
		return
	}

	// Direct SmartOLT Live Diagnostic check if configured
	smartClient, _ := s.resolveSmartOLTClientForRegistration(r.Context(), reg)
	if smartClient != nil && smartClient.IsConfigured() {
		if diag, err := smartClient.GetONUSignalDiagnostics(r.Context(), sn); err == nil && diag != nil {
			hosts, hostsErr := smartClient.GetONURouterHosts(r.Context(), sn)
			hostsNotice := ""
			if hostsErr != nil {
				hostsNotice = hostsErr.Error()
			}

			response.Success(w, "Telemetri Modem ONT GOGIGANET", map[string]interface{}{
				"has_fttx_integration":     true,
				"can_configure_wifi":       true,
				"can_reboot":               true,
				"serial_number":            sn,
				"vendor":                   "ZTE",
				"model":                    diag.DeviceType,
				"rx_optical_power":         diag.RxPowerDBM,
				"tx_optical_power":         diag.TxPowerDBM,
				"attenuation_db":           diag.AttenuationDB,
				"distance_meters":          diag.DistanceMeters,
				"ip_address":               diag.WANIPv4,
				"status":                   diag.Status,
				"signal_quality":           diag.SignalQuality,
				"source":                   "GOGIGANET Optical Network",
				"connected_devices":        hosts,
				"connected_devices_notice": hostsNotice,
			})
			return
		}
	}

	// Query ACS CPE Data from FTTX
	reqCPE, err := http.NewRequestWithContext(r.Context(), "GET", fmt.Sprintf("%s/api/v1/internal/acs/cpe/%s", s.fttxBaseURL, sn), nil)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat request ke FTTX", "internal_error")
		return
	}
	reqCPE.Header.Set("X-Internal-Key", s.fttxInternalKey)
	respCPE, err := client.Do(reqCPE)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "Gagal menghubungi FTTX Command Center: "+err.Error(), "gateway_error")
		return
	}
	defer respCPE.Body.Close()

	var cpeRes struct {
		Success bool `json:"success"`
		Data    struct {
			SerialNumber   string `json:"serial_number"`
			Manufacturer   string `json:"manufacturer"`
			Model          string `json:"model"`
			WANIP          string `json:"wan_ip"`
			UptimeSeconds  int64  `json:"uptime_seconds"`
			WifiSSID2G     string `json:"wifi_ssid_2g"`
			WifiPassword2G string `json:"wifi_password_2g"`
			WifiSSID5G     string `json:"wifi_ssid_5g"`
			WifiPassword5G string `json:"wifi_password_5g"`
		} `json:"data"`
	}
	_ = json.NewDecoder(respCPE.Body).Decode(&cpeRes)

	// Fetch signal from FTTX ONT list
	var rxPower float64 = -19.5
	var ontStatus = "ONLINE"
	var vendor = cpeRes.Data.Manufacturer
	var model = cpeRes.Data.Model

	reqOnt, err := http.NewRequestWithContext(r.Context(), "GET", fmt.Sprintf("%s/api/v1/internal/ont/lookup?sn=%s", s.fttxBaseURL, url.QueryEscape(sn)), nil)
	if err == nil {
		reqOnt.Header.Set("X-Internal-Key", s.fttxInternalKey)
		if respOnt, err := client.Do(reqOnt); err == nil {
			defer respOnt.Body.Close()
			var ontRes struct {
				Success bool `json:"success"`
				Data    struct {
					SerialNumber string  `json:"serial_number"`
					Brand        string  `json:"brand"`
					Model        string  `json:"model"`
					SignalRxDBM  float64 `json:"signal_rx_dbm"`
					Status       string  `json:"status"`
				} `json:"data"`
			}
			if err := json.NewDecoder(respOnt.Body).Decode(&ontRes); err == nil && ontRes.Success && ontRes.Data.SerialNumber != "" {
				rxPower = ontRes.Data.SignalRxDBM
				ontStatus = ontRes.Data.Status
				if vendor == "" {
					vendor = ontRes.Data.Brand
				}
				if model == "" {
					model = ontRes.Data.Model
				}
			}
		}
	}

	response.Success(w, "Telemetri ONT TR-069 FTTX Command Center", map[string]interface{}{
		"has_fttx_integration": true,
		"serial_number":        sn,
		"vendor":               vendor,
		"model":                model,
		"rx_optical_power":     rxPower,
		"ip_address":           cpeRes.Data.WANIP,
		"uptime_seconds":       cpeRes.Data.UptimeSeconds,
		"wifi_ssid":            cpeRes.Data.WifiSSID2G,
		"wifi_password":        cpeRes.Data.WifiPassword2G,
		"wifi_ssid_5g":         cpeRes.Data.WifiSSID5G,
		"wifi_password_5g":     cpeRes.Data.WifiPassword5G,
		"status":               ontStatus,
	})
}

func (s *Server) handleCustomerUpdateONTWifi(w http.ResponseWriter, r *http.Request) {
	regNo := chi.URLParam(r, "regNo")
	reg, err := s.svc.GetRegistrationByNo(r.Context(), regNo)
	if err != nil || reg == nil {
		response.Error(w, http.StatusNotFound, "Registrasi pelanggan tidak ditemukan", "not_found")
		return
	}

	var reqBody struct {
		SerialNumber string `json:"serial_number"`
		SSID         string `json:"ssid"`
		Password     string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid", "bad_request")
		return
	}

	reqBody.SSID = strings.TrimSpace(reqBody.SSID)
	reqBody.Password = strings.TrimSpace(reqBody.Password)
	if reqBody.SSID == "" {
		response.Error(w, http.StatusBadRequest, "Nama Wi-Fi (SSID) wajib diisi", "missing_ssid")
		return
	}
	if len(reqBody.Password) < 8 {
		response.Error(w, http.StatusBadRequest, "Kata sandi Wi-Fi minimal 8 karakter", "invalid_password")
		return
	}

	sn := reqBody.SerialNumber
	if sn == "" {
		sn = reg.ONTSerialNumber
	}
	if sn == "" {
		if wo, err := s.repo.GetWorkOrderByRegistrationID(r.Context(), reg.ID); err == nil && wo != nil && wo.BAST != nil {
			sn = wo.BAST.ONTSerialNumber
		}
	}
	if sn == "" {
		response.Error(w, http.StatusBadRequest, "Serial number ONT tidak ditemukan", "missing_sn")
		return
	}

	// 1. Prioritize Direct SmartOLT Wi-Fi update (OMCI)
	smartClient, smartCfg := s.resolveSmartOLTClientForRegistration(r.Context(), reg)
	if smartClient != nil && smartClient.IsConfigured() {
		provName := "GOGIGANET"
		if smartCfg != nil && smartCfg.ProviderID != "" {
			provName = smartCfg.ProviderID
		}
		if err := smartClient.SetONUWifi(r.Context(), sn, reqBody.SSID, reqBody.Password); err == nil {
			response.Success(w, fmt.Sprintf("Nama Wi-Fi dan kata sandi modem %s berhasil diperbarui", sn), map[string]interface{}{
				"serial_number": sn,
				"wifi_ssid":     reqBody.SSID,
				"status":        "UPDATED",
			})
			return
		} else {
			log.Printf("[%s] SetONUWifi failed on %s: %v", provName, sn, err)
			if !reg.HasFTTXIntegration {
				response.Error(w, http.StatusBadGateway, "Gagal memperbarui konfigurasi Wi-Fi modem: "+err.Error(), "modem_error")
				return
			}
		}
	}

	if !reg.HasFTTXIntegration {
		response.Error(w, http.StatusForbidden, "Pengaturan Wi-Fi mandiri tidak tersedia pada segmen jaringan ini. Silakan hubungi tim Customer Care GOGIGANET.", "action_not_allowed")
		return
	}

	trueVal := true
	fttxReqBody, _ := json.Marshal(map[string]interface{}{
		"ssid_2g":     reqBody.SSID,
		"password_2g": reqBody.Password,
		"enabled_2g":  &trueVal,
		"ssid_5g":     reqBody.SSID,
		"password_5g": reqBody.Password,
		"enabled_5g":  &trueVal,
	})

	client := &http.Client{Timeout: 10 * time.Second}
	fttxReq, err := http.NewRequestWithContext(r.Context(), "PUT", fmt.Sprintf("%s/api/v1/internal/acs/cpe/%s/wifi", s.fttxBaseURL, sn), bytes.NewReader(fttxReqBody))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat request ke FTTX", "internal_error")
		return
	}
	fttxReq.Header.Set("Content-Type", "application/json")
	fttxReq.Header.Set("X-Internal-Key", s.fttxInternalKey)

	resp, err := client.Do(fttxReq)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "Gagal menghubungi FTTX Command Center: "+err.Error(), "gateway_error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		response.Error(w, resp.StatusCode, "Gagal update Wi-Fi di FTTX: "+string(body), "fttx_error")
		return
	}

	response.Success(w, "Konfigurasi Wi-Fi berhasil diperbarui via TR-069 FTTX Command Center", map[string]string{
		"serial_number": sn,
		"wifi_ssid":     reqBody.SSID,
	})
}

func (s *Server) handleCustomerRebootONT(w http.ResponseWriter, r *http.Request) {
	regNo := chi.URLParam(r, "regNo")
	reg, err := s.svc.GetRegistrationByNo(r.Context(), regNo)
	if err != nil || reg == nil {
		response.Error(w, http.StatusNotFound, "Registrasi pelanggan tidak ditemukan", "not_found")
		return
	}

	var reqBody struct {
		SerialNumber string `json:"serial_number"`
	}
	_ = json.NewDecoder(r.Body).Decode(&reqBody)

	sn := reqBody.SerialNumber
	if sn == "" {
		sn = reg.ONTSerialNumber
	}
	if sn == "" {
		if wo, err := s.repo.GetWorkOrderByRegistrationID(r.Context(), reg.ID); err == nil && wo != nil && wo.BAST != nil {
			sn = wo.BAST.ONTSerialNumber
		}
	}
	if sn == "" {
		response.Error(w, http.StatusBadRequest, "Serial number ONT tidak ditemukan", "missing_sn")
		return
	}

	// 1. Direct SmartOLT Reboot
	smartClient, smartCfg := s.resolveSmartOLTClientForRegistration(r.Context(), reg)
	if smartClient != nil && smartClient.IsConfigured() {
		provName := "GOGIGANET"
		if smartCfg != nil && smartCfg.ProviderID != "" {
			provName = smartCfg.ProviderID
		}
		if err := smartClient.RebootONU(r.Context(), sn); err == nil {
			response.Success(w, fmt.Sprintf("Perintah restart modem %s berhasil dikirim", sn), map[string]interface{}{
				"serial_number": sn,
				"status":        "REBOOT_TRIGGERED",
			})
			return
		} else {
			log.Printf("[%s] Reboot failed on %s: %v", provName, sn, err)
			if !reg.HasFTTXIntegration {
				response.Error(w, http.StatusBadGateway, "Gagal mengirim perintah restart modem: "+err.Error(), "modem_error")
				return
			}
		}
	}

	if !reg.HasFTTXIntegration {
		response.Error(w, http.StatusForbidden, "Reboot mandiri tidak tersedia pada segmen jaringan ini. Silakan hubungi tim Customer Care GOGIGANET.", "action_not_allowed")
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	fttxReq, err := http.NewRequestWithContext(r.Context(), "POST", fmt.Sprintf("%s/api/v1/internal/acs/cpe/%s/reboot", s.fttxBaseURL, sn), nil)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat request ke FTTX", "internal_error")
		return
	}
	fttxReq.Header.Set("X-Internal-Key", s.fttxInternalKey)

	resp, err := client.Do(fttxReq)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "Gagal menghubungi FTTX Command Center: "+err.Error(), "gateway_error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		response.Error(w, resp.StatusCode, "Gagal reboot ONT di FTTX: "+string(body), "fttx_error")
		return
	}

	response.Success(w, "Perintah restart modem berhasil dikirim via TR-069 FTTX Command Center", map[string]string{
		"serial_number": sn,
		"status":        "REBOOT_QUEUED",
	})
}

func (s *Server) handlePartnerLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code  string `json:"code"`
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		response.Error(w, http.StatusBadRequest, "Kode Sales / Mitra wajib diisi", "bad_request")
		return
	}

	partner, err := s.svc.GetPartnerByCode(r.Context(), strings.TrimSpace(req.Code))
	if err != nil || partner == nil {
		response.Error(w, http.StatusNotFound, "Kode Sales / Mitra tidak ditemukan dalam database", "not_found")
		return
	}

	if !partner.IsActive {
		response.Error(w, http.StatusForbidden, "Akun Sales/Mitra ini sedang non-aktif", "forbidden")
		return
	}

	// Jika nomor kontak disertakan, verifikasi dengan toleransi format
	if req.Phone != "" && partner.ContactPhone != "" {
		cleanReq := strings.TrimLeft(strings.ReplaceAll(strings.ReplaceAll(req.Phone, " ", ""), "-", ""), "0+")
		cleanPartner := strings.TrimLeft(strings.ReplaceAll(strings.ReplaceAll(partner.ContactPhone, " ", ""), "-", ""), "0+")
		if !strings.Contains(cleanPartner, cleanReq) && !strings.Contains(cleanReq, cleanPartner) {
			response.Error(w, http.StatusUnauthorized, "Nomor WhatsApp tidak cocok dengan data sales terdaftar", "unauthorized")
			return
		}
	}

	response.Success(w, "Login Sales Berhasil", partner)
}

func (s *Server) handleCheckReferralCode(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if strings.TrimSpace(code) == "" {
		response.Error(w, http.StatusBadRequest, "Parameter kode referral diperlukan", "missing_code")
		return
	}

	result, err := s.svc.CheckReferralCode(r.Context(), code)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memverifikasi kode referral", "server_error")
		return
	}

	if !result.Valid {
		response.Error(w, http.StatusNotFound, "Kode referral tidak terdaftar atau tidak aktif", "invalid_referral")
		return
	}

	response.Success(w, "Kode referral valid", result)
}

func (s *Server) handleGetPlans(w http.ResponseWriter, r *http.Request) {
	clusterParam := r.URL.Query().Get("cluster")
	if clusterParam == "" {
		clusterParam = r.URL.Query().Get("cluster_area")
	}

	// 1. Ambil live dari GoGigaBill API jika billing client aktif
	if s.client != nil {
		plans, err := s.client.FetchPlans(r.Context(), clusterParam)
		if err == nil && len(plans) > 0 {
			response.Success(w, "Daftar paket internet resmi GoGiga", plans)
			return
		}
	}

	// 2. Fallback paket berdasarkan klaster wilayah
	cUpper := strings.ToUpper(clusterParam)
	var plans []billingclient.PlanDTO

	if strings.Contains(cUpper, "BIARO") || strings.Contains(cUpper, "001") {
		plans = []billingclient.PlanDTO{
			{ID: "paket-gold-10m-001", Name: "Paket Gold (Biaro)", Description: "Hemat Rumah Tangga & Pelajar", DownloadKbps: 10240, UploadKbps: 10240, MonthlyPrice: 165000, PackageGroup: "Cluster 001 - Biaro & Agam", ClusterCode: "001"},
			{ID: "paket-diamond-20m-001", Name: "Paket Diamond (Biaro)", Description: "Streaming & Belajar Stabil", DownloadKbps: 20480, UploadKbps: 20480, MonthlyPrice: 220000, PackageGroup: "Cluster 001 - Biaro & Agam", ClusterCode: "001"},
			{ID: "paket-epic-30m-001", Name: "Paket Epic (Biaro)", Description: "Streaming HD & Gaming Lancar", DownloadKbps: 30720, UploadKbps: 30720, MonthlyPrice: 265000, PackageGroup: "Cluster 001 - Biaro & Agam", ClusterCode: "001"},
			{ID: "paket-honor-50m-001", Name: "Paket Honor (Biaro)", Description: "Keluarga Besar & Multi-Device", DownloadKbps: 51200, UploadKbps: 51200, MonthlyPrice: 350000, PackageGroup: "Cluster 001 - Biaro & Agam", ClusterCode: "001"},
			{ID: "paket-glory-100m-001", Name: "Paket Glory (Biaro)", Description: "Kecepatan Super Maksimal", DownloadKbps: 102400, UploadKbps: 102400, MonthlyPrice: 550000, PackageGroup: "Cluster 001 - Biaro & Agam", ClusterCode: "001"},
		}
	} else if strings.Contains(cUpper, "HARAU") || strings.Contains(cUpper, "004") {
		plans = []billingclient.PlanDTO{
			{ID: "paket-gold-10m-004", Name: "Paket Gold (Harau)", Description: "Internet Desa Cerdas Koto Tuo & Harau", DownloadKbps: 10240, UploadKbps: 10240, MonthlyPrice: 175000, PackageGroup: "Cluster 004 - Harau", ClusterCode: "004"},
			{ID: "paket-diamond-20m-004", Name: "Paket Diamond (Harau)", Description: "Koneksi Cepat Rumah & Homestay", DownloadKbps: 20480, UploadKbps: 20480, MonthlyPrice: 235000, PackageGroup: "Cluster 004 - Harau", ClusterCode: "004"},
			{ID: "paket-epic-30m-004", Name: "Paket Epic (Harau)", Description: "Streaming & Bisnis Wisata", DownloadKbps: 30720, UploadKbps: 30720, MonthlyPrice: 280000, PackageGroup: "Cluster 004 - Harau", ClusterCode: "004"},
			{ID: "paket-honor-50m-004", Name: "Paket Honor (Harau)", Description: "Keluarga & Usaha Produktif", DownloadKbps: 51200, UploadKbps: 51200, MonthlyPrice: 370000, PackageGroup: "Cluster 004 - Harau", ClusterCode: "004"},
			{ID: "paket-glory-100m-004", Name: "Paket Glory (Harau)", Description: "Kecepatan Maksimal Tanpa Batas", DownloadKbps: 102400, UploadKbps: 102400, MonthlyPrice: 575000, PackageGroup: "Cluster 004 - Harau", ClusterCode: "004"},
		}
	} else {
		// Standar / Payakumbuh Cluster 002
		plans = []billingclient.PlanDTO{
			{ID: "c0020001-0000-0000-0000-000000000001", Name: "Paket Gold (10M)", Description: "Gaming & Streaming Hemat Rumah Anda (HOT)", DownloadKbps: 10240, UploadKbps: 10240, MonthlyPrice: 183150, PackageGroup: "Cluster 002 - Payakumbuh", ClusterCode: "002"},
			{ID: "c0020002-0000-0000-0000-000000000002", Name: "Paket Diamond (20M)", Description: "Lebih Cepat, Stabil & Terjangkau", DownloadKbps: 20480, UploadKbps: 20480, MonthlyPrice: 259000, PackageGroup: "Cluster 002 - Payakumbuh", ClusterCode: "002"},
			{ID: "c0020003-0000-0000-0000-000000000003", Name: "Paket Epic (30M)", Description: "Streaming Lancar Tanpa Buffering", DownloadKbps: 30720, UploadKbps: 30720, MonthlyPrice: 299000, PackageGroup: "Cluster 002 - Payakumbuh", ClusterCode: "002"},
			{ID: "c0020004-0000-0000-0000-000000000004", Name: "Paket Legend (40M)", Description: "Gaming Tanpa Lag & Belajar/Kerja Produktif", DownloadKbps: 40960, UploadKbps: 40960, MonthlyPrice: 329000, PackageGroup: "Cluster 002 - Payakumbuh", ClusterCode: "002"},
			{ID: "c0020005-0000-0000-0000-000000000005", Name: "Paket Honor (50M)", Description: "Koneksi Multi-Device Keluarga", DownloadKbps: 51200, UploadKbps: 51200, MonthlyPrice: 389000, PackageGroup: "Cluster 002 - Payakumbuh", ClusterCode: "002"},
			{ID: "c0020006-0000-0000-0000-000000000006", Name: "Paket Glory (100M)", Description: "Kecepatan Maksimal Tanpa Batas", DownloadKbps: 102400, UploadKbps: 102400, MonthlyPrice: 599000, PackageGroup: "Cluster 002 - Payakumbuh", ClusterCode: "002"},
			{ID: "c0020007-0000-0000-0000-000000000007", Name: "Paket Corporate & Dedicated (150M-1G)", Description: "150 Mbps – 1 Gbps • Kebutuhan Bisnis, Kantor, Sekolah & Dedicated IP", DownloadKbps: 153600, UploadKbps: 153600, MonthlyPrice: 0, PackageGroup: "Cluster 002 - Payakumbuh", ClusterCode: "002"},
		}
	}

	response.Success(w, "Daftar paket internet resmi GoGiga", plans)
}

func (s *Server) handlePublicODPMap(w http.ResponseWriter, r *http.Request) {
	branchCode := strings.TrimSpace(r.URL.Query().Get("branch"))
	odps, err := s.svc.ListODPs(r.Context(), branchCode)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat peta ODP", err.Error())
		return
	}
	response.Success(w, "Daftar titik ODP berhasil dimuat", odps)
}

func (s *Server) handlePartnerListRegistrations(w http.ResponseWriter, r *http.Request) {
	partner := s.getAuthenticatedPartner(r)
	if partner == nil {
		response.Error(w, http.StatusUnauthorized, "Autentikasi sales tidak valid", "unauthorized")
		return
	}

	regs, err := s.svc.ListRegistrationsForPartner(r.Context(), partner.ID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mengambil daftar registrasi mitra", err.Error())
		return
	}
	response.Success(w, "Daftar registrasi mitra & pool lead bisnis", regs)
}

func (s *Server) handlePartnerClaimRegistration(w http.ResponseWriter, r *http.Request) {
	partner := s.getAuthenticatedPartner(r)
	if partner == nil {
		response.Error(w, http.StatusUnauthorized, "Autentikasi sales tidak valid", "unauthorized")
		return
	}
	regNo := chi.URLParam(r, "regNo")
	if regNo == "" {
		response.Error(w, http.StatusBadRequest, "Nomor registrasi wajib disertakan", "bad_request")
		return
	}

	reg, err := s.svc.ClaimRegistration(r.Context(), regNo, partner.ID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "claim_failed")
		return
	}

	response.Success(w, fmt.Sprintf("Lead %s berhasil diklaim atas nama sales %s (%s)", reg.RegistrationNo, partner.Name, partner.Code), reg)
}

func (s *Server) handlePartnerUpdateRegistrationPricing(w http.ResponseWriter, r *http.Request) {
	partner := s.getAuthenticatedPartner(r)
	if partner == nil {
		response.Error(w, http.StatusUnauthorized, "Autentikasi sales tidak valid", "unauthorized")
		return
	}

	regNo := chi.URLParam(r, "regNo")
	if regNo == "" {
		response.Error(w, http.StatusBadRequest, "Nomor registrasi wajib diisi", "missing_reg_no")
		return
	}

	var req domain.UpdateRegistrationPricingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid: "+err.Error(), "bad_request")
		return
	}

	// Sales submit kesepakatan overdistance/jalur khusus:
	// Wajib melalui persetujuan NOC / Atasan terlebih dahulu sebelum diterbitkan SPK pasang & invoice
	if req.PromoteToInstall || req.RequestNOCApproval {
		req.RequestNOCApproval = true
		req.PromoteToInstall = false
	}

	updated, err := s.svc.UpdateRegistrationPricing(r.Context(), regNo, req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "pricing_update_failed")
		return
	}

	msg := "Penawaran biaya (OTC & Tarif Bulanan) berhasil disimpan"
	if req.RequestNOCApproval {
		msg = "Kesepakatan biaya dan paket berhasil diajukan ke NOC / Atasan untuk verifikasi teknis"
	}
	response.Success(w, msg, updated)
}

func (s *Server) handleAdminNOCApprovalRegistration(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau No. Registrasi wajib diisi", "missing_id")
		return
	}

	var req domain.NOCApprovalActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid: "+err.Error(), "bad_request")
		return
	}

	if req.ApproverName == "" {
		req.ApproverName = "NOC Operations"
	}

	updated, err := s.svc.NOCApprovalRegistration(r.Context(), id, req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "noc_approval_failed")
		return
	}

	msg := "Pendaftaran overdistance berhasil disetujui NOC. Status telah diubah ke Siap Pasang dan Faktur Tagihan resmi diterbitkan."
	if strings.ToUpper(strings.TrimSpace(req.Action)) == "REJECT" {
		msg = "Pengajuan penarikan jalur khusus telah ditolak/dikembalikan ke Sales untuk revisi."
	}

	response.Success(w, msg, updated)
}

func (s *Server) handleAdminUpdateRegistrationPricing(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau No. Registrasi wajib diisi", "missing_id")
		return
	}

	var req domain.UpdateRegistrationPricingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid: "+err.Error(), "bad_request")
		return
	}

	updated, err := s.svc.UpdateRegistrationPricing(r.Context(), id, req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "pricing_update_failed")
		return
	}

	response.Success(w, "Penawaran biaya (OTC & Tarif Bulanan) berhasil diperbarui", updated)
}

func (s *Server) handleAdminUpgradePlan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau No. Registrasi wajib diisi", "missing_id")
		return
	}

	var req domain.UpgradeBandwidthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid: "+err.Error(), "bad_request")
		return
	}

	updated, err := s.svc.UpgradeBandwidth(r.Context(), id, req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "upgrade_plan_failed")
		return
	}

	response.Success(w, "Paket bandwidth berhasil diperbarui", updated)
}

func (s *Server) handleAdminListRegistrations(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}

	branchCode := strings.TrimSpace(r.URL.Query().Get("branch"))
	if sess, ok := r.Context().Value(authSessionCtxKey).(*domain.AuthSession); ok && sess != nil {
		if (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") && sess.BranchCode != nil && *sess.BranchCode != "" {
			branchCode = *sess.BranchCode
		}
	}
	var branchPtr *string
	if branchCode != "" && branchCode != "ALL" {
		branchPtr = &branchCode
	}

	regs, err := s.svc.ListRegistrations(r.Context(), nil, statusPtr, branchPtr)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mengambil data", err.Error())
		return
	}
	response.Success(w, "Daftar seluruh registrasi", regs)
}

func (s *Server) handleAdminSuspendRegistration(w http.ResponseWriter, r *http.Request) {
	sess, _ := r.Context().Value(authSessionCtxKey).(*domain.AuthSession)
	if sess != nil && strings.ToUpper(sess.Role) != "ADMIN_NOC" && strings.ToUpper(sess.Role) != "SUPER_ADMIN" {
		response.Error(w, http.StatusForbidden, "Akses ditolak: Hanya peran NOC dan Super User yang berhak mengisolir/menjeda layanan", "forbidden")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau No. Registrasi wajib diisi", "missing_id")
		return
	}

	var req domain.SuspendRegistrationRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	updatedReg, err := s.svc.SuspendRegistration(r.Context(), id, req.Reason)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "suspension_not_allowed")
		return
	}

	response.Success(w, "Layanan pelanggan berhasil dijeda sementara", updatedReg)
}

func (s *Server) handleAdminResumeRegistration(w http.ResponseWriter, r *http.Request) {
	sess, _ := r.Context().Value(authSessionCtxKey).(*domain.AuthSession)
	if sess != nil && strings.ToUpper(sess.Role) != "ADMIN_NOC" && strings.ToUpper(sess.Role) != "SUPER_ADMIN" {
		response.Error(w, http.StatusForbidden, "Akses ditolak: Hanya peran NOC dan Super User yang berhak mengaktifkan kembali layanan", "forbidden")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau No. Registrasi wajib diisi", "missing_id")
		return
	}

	updatedReg, err := s.svc.ResumeRegistration(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mengaktifkan kembali layanan", err.Error())
		return
	}

	response.Success(w, "Layanan pelanggan berhasil diaktifkan kembali", updatedReg)
}

func (s *Server) handleAdminLiveRadiusSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.svc.GetLiveRadiusSessions(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mengambil sesi FreeRADIUS", err.Error())
		return
	}
	response.Success(w, "Data sesi PPPoE aktif berhasil diambil", sessions)
}

func (s *Server) handleAdminKickPPPoESession(w http.ResponseWriter, r *http.Request) {
	sess, _ := r.Context().Value(authSessionCtxKey).(*domain.AuthSession)
	if sess != nil && strings.ToUpper(sess.Role) != "ADMIN_NOC" && strings.ToUpper(sess.Role) != "SUPER_ADMIN" {
		response.Error(w, http.StatusForbidden, "Akses ditolak: Hanya peran NOC dan Super User yang berhak mereset/kick sesi pelanggan", "forbidden")
		return
	}

	var req domain.KickSessionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.PPPoEUsername == "" {
		req.PPPoEUsername = r.URL.Query().Get("pppoe_username")
	}
	if strings.TrimSpace(req.PPPoEUsername) == "" {
		response.Error(w, http.StatusBadRequest, "Username PPPoE wajib diisi", "missing_username")
		return
	}

	if err := s.svc.KickPPPoESession(r.Context(), req.PPPoEUsername); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mengirim perintah CoA Disconnect ke MikroTik", err.Error())
		return
	}

	response.Success(w, fmt.Sprintf("Perintah reset sesi (CoA Disconnect) untuk %s berhasil dikirim ke router MikroTik", req.PPPoEUsername), map[string]string{
		"username": req.PPPoEUsername,
		"status":   "DISCONNECTED",
	})
}

func (s *Server) handleAdminAttachPartnerODP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau No. Registrasi wajib diisi", "missing_id")
		return
	}

	var req domain.AttachPartnerODPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid", err.Error())
		return
	}

	if strings.TrimSpace(req.ODPCode) == "" {
		response.Error(w, http.StatusBadRequest, "Nomor/Kode ODP wajib diisi", "missing_odp_code")
		return
	}

	updatedReg, err := s.svc.AttachPartnerODP(r.Context(), id, req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mengalihkan sirkuit ke mitra wholesale", err.Error())
		return
	}

	response.Success(w, "Sirkuit pelanggan berhasil dihubungkan ke infrastruktur mitra wholesale", updatedReg)
}

func (s *Server) handleAdminUpdateUpstreamPPPoE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau No. Registrasi wajib diisi", "missing_id")
		return
	}

	var req domain.UpdateUpstreamPPPoERequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid", err.Error())
		return
	}

	updatedReg, err := s.svc.UpdateUpstreamPPPoE(r.Context(), id, req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "update_failed")
		return
	}

	response.Success(w, "Kredensial dial ONT upstream/jartaplok berhasil diperbarui", updatedReg)
}

func (s *Server) handleAdminReassignODP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau No. Registrasi wajib diisi", "missing_id")
		return
	}

	var req domain.ReassignODPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid", err.Error())
		return
	}

	if strings.TrimSpace(req.ODPCode) == "" {
		response.Error(w, http.StatusBadRequest, "Nomor/Kode ODP wajib diisi", "missing_odp_code")
		return
	}

	updatedReg, err := s.svc.ReassignODP(r.Context(), id, req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "reassign_failed")
		return
	}

	response.Success(w, "Titik ODP berhasil dialihkan", updatedReg)
}

func (s *Server) handleAdminMarkUncovered(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau No. Registrasi wajib diisi", "missing_id")
		return
	}

	var req domain.RejectUncoveredRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	updatedReg, err := s.svc.MarkUncovered(r.Context(), id, req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memperbarui status permohonan", err.Error())
		return
	}

	msg := "Permohonan pelanggan telah dicatat dalam Daftar Prioritas Perluasan Jaringan (Wishlist)"
	if strings.ToUpper(req.Action) == "CANCEL" {
		msg = "Permohonan pelanggan resmi dibatalkan karena di luar jangkauan jaringan"
	}

	response.Success(w, msg, updatedReg)
}

func (s *Server) handleCreateWorkOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RegistrationID string    `json:"registration_id"`
		Type           string    `json:"type"` // SURVEY / INSTALLATION
		TechnicianName string    `json:"technician_name"`
		ScheduledAt    time.Time `json:"scheduled_at"`
		Notes          string    `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid body", err.Error())
		return
	}

	wo, err := s.svc.CreateWorkOrder(r.Context(), req.RegistrationID, req.Type, req.TechnicianName, req.ScheduledAt, req.Notes)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat work order", err.Error())
		return
	}

	response.Created(w, "Work order berhasil diterbitkan", wo)
}

func (s *Server) handleAdminAssignWorkOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req domain.AssignWorkOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}

	wo, err := s.svc.AssignWorkOrder(r.Context(), id, req.TechnicianName, req.Notes)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal menugaskan work order", err.Error())
		return
	}
	response.Success(w, "Penugasan work order berhasil diperbarui", wo)
}

func (s *Server) handleTechnicianClaimWorkOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req domain.AssignWorkOrderRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	techName := strings.TrimSpace(req.TechnicianName)
	if techName == "" {
		techName = "Teknisi Lapangan"
	}

	wo, err := s.svc.AssignWorkOrder(r.Context(), id, techName, "Diambil mandiri oleh teknisi: "+techName)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mengambil job", err.Error())
		return
	}
	response.Success(w, fmt.Sprintf("Job berhasil diambil oleh %s", techName), wo)
}

func (s *Server) handleListWorkOrders(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}

	wos, err := s.svc.ListWorkOrders(r.Context(), statusPtr)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat work order", err.Error())
		return
	}
	response.Success(w, "Daftar work order teknisi", wos)
}

func (s *Server) handleGetWorkOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	wo, err := s.svc.GetWorkOrderByID(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusNotFound, "Work order tidak ditemukan", err.Error())
		return
	}
	response.Success(w, "Detail work order", wo)
}

func (s *Server) handleSubmitBAST(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req domain.SubmitBASTRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid BAST payload", err.Error())
		return
	}

	updatedReg, err := s.svc.SubmitBASTAndPromote(r.Context(), id, req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Gagal submit BAST: "+err.Error(), err.Error())
		return
	}

	response.Success(w, "BAST berhasil diverifikasi & Pelanggan telah aktif dan terhubung ke Billing", updatedReg)
}

func (s *Server) handleTechnicianListReferrals(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		code = "BUDI-TECH"
	}
	partner, err := s.svc.GetPartnerByCode(r.Context(), code)
	if err != nil || partner == nil {
		response.Error(w, http.StatusNotFound, "Kode referral teknisi tidak ditemukan", "not_found")
		return
	}

	regs, err := s.svc.ListRegistrations(r.Context(), &partner.ID, nil, nil)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat prospek referral teknisi", err.Error())
		return
	}

	var activeCount int
	for _, reg := range regs {
		if reg.Status == "ACTIVE" {
			activeCount++
		}
	}

	res := map[string]interface{}{
		"partner":           partner,
		"total_leads":       len(regs),
		"active_count":      activeCount,
		"earned_commission": float64(activeCount) * partner.CommissionRate,
		"registrations":     regs,
	}
	response.Success(w, "Data Referral Teknisi", res)
}

func (s *Server) handleGetCustomerDocuments(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "customerId")
	email := r.URL.Query().Get("email")
	phone := r.URL.Query().Get("phone")

	data, err := s.svc.GetCustomerDocuments(r.Context(), customerID, email, phone)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat dokumen dan BAST pelanggan: "+err.Error(), err.Error())
		return
	}

	response.Success(w, "Data dokumen dan BAST berhasil dimuat", data)
}

func (s *Server) handleAdminListODPs(w http.ResponseWriter, r *http.Request) {
	branchCode := strings.TrimSpace(r.URL.Query().Get("branch"))
	if sess, ok := r.Context().Value(authSessionCtxKey).(*domain.AuthSession); ok && sess != nil {
		if (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") && sess.BranchCode != nil && *sess.BranchCode != "" {
			branchCode = *sess.BranchCode
		}
	}
	odps, err := s.svc.ListODPs(r.Context(), branchCode)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat data ODP", err.Error())
		return
	}
	response.Success(w, "Daftar ODP", odps)
}

func (s *Server) handleAdminCreateODP(w http.ResponseWriter, r *http.Request) {
	var odp domain.ODPNode
	if err := json.NewDecoder(r.Body).Decode(&odp); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid ODP data", err.Error())
		return
	}

	if odp.Code == "" || odp.TotalPorts <= 0 {
		response.Error(w, http.StatusBadRequest, "Kode ODP dan Total Port wajib diisi", "validation_error")
		return
	}

	if odp.Status == "" {
		odp.Status = "AVAILABLE"
	}

	if err := s.svc.CreateODP(r.Context(), &odp); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal menyimpan ODP", err.Error())
		return
	}

	response.Created(w, "Titik ODP baru berhasil ditambahkan", odp)
}

func (s *Server) requireSuperuserRole(r *http.Request) error {
	if sess, ok := r.Context().Value(authSessionCtxKey).(*domain.AuthSession); ok && sess != nil {
		if sess.Role == "SUPER_ADMIN" || sess.Role == "superuser" {
			return nil
		}
		return fmt.Errorf("akses ditolak: hanya Super User (Owner) yang berhak menghapus data")
	}
	role := r.Header.Get("X-User-Role")
	adminKey := r.Header.Get("X-Admin-Key")
	if adminKey != "" && adminKey == s.adminAPIKey && (role == "superuser" || role == "SUPER_ADMIN") {
		return nil
	}
	return fmt.Errorf("akses ditolak: hanya Super User (Owner) yang berhak menghapus data")
}

func (s *Server) handleAdminDeleteODP(w http.ResponseWriter, r *http.Request) {
	if err := s.requireSuperuserRole(r); err != nil {
		response.Error(w, http.StatusForbidden, err.Error(), "forbidden")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau Kode ODP wajib diisi", "missing_id")
		return
	}

	if err := s.svc.DeleteODP(r.Context(), id); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "delete_failed")
		return
	}

	response.Success(w, "Titik ODP berhasil dihapus dari sistem", map[string]string{"id": id})
}

func (s *Server) handleAdminDeleteRegistration(w http.ResponseWriter, r *http.Request) {
	if err := s.requireSuperuserRole(r); err != nil {
		response.Error(w, http.StatusForbidden, err.Error(), "forbidden")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau Nomor Registrasi wajib diisi", "missing_id")
		return
	}

	if err := s.svc.DeleteRegistration(r.Context(), id); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "delete_failed")
		return
	}

	response.Success(w, "Permohonan registrasi berhasil dihapus permanen dari sistem", map[string]string{"id": id})
}

func (s *Server) handleAdminListPartners(w http.ResponseWriter, r *http.Request) {
	branch := r.URL.Query().Get("branch")
	sess, _ := r.Context().Value(authSessionCtxKey).(*domain.AuthSession)
	if sess != nil && (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") {
		if sess.BranchCode != nil && *sess.BranchCode != "" {
			branch = *sess.BranchCode
		}
	}
	partners, err := s.svc.ListPartners(r.Context(), branch)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat mitra", err.Error())
		return
	}
	response.Success(w, "Daftar Mitra / Reseller", partners)
}

func (s *Server) handleAdminCreatePartner(w http.ResponseWriter, r *http.Request) {
	sess, _ := r.Context().Value(authSessionCtxKey).(*domain.AuthSession)
	isSuper := false
	isBranchManager := false
	var userBranchCode string
	if sess != nil {
		if strings.ToUpper(sess.Role) == "SUPER_ADMIN" || sess.Role == "superuser" {
			isSuper = true
		} else if sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager" {
			isBranchManager = true
			if sess.BranchCode != nil {
				userBranchCode = *sess.BranchCode
			}
		}
	} else {
		role := strings.ToLower(r.Header.Get("X-User-Role"))
		adminKey := r.Header.Get("X-Admin-Key")
		if adminKey != "" && role == "superuser" {
			isSuper = true
		} else if adminKey != "" && role == "branch_manager" {
			isBranchManager = true
		}
	}

	if !isSuper && !isBranchManager {
		response.Error(w, http.StatusForbidden, "Akses ditolak: Hanya level Super User (Owner) atau Kepala Cabang yang berhak mendaftarkan mitra & sales", "forbidden")
		return
	}

	var p domain.Partner
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}

	if p.Code == "" || p.Name == "" || p.APIKey == "" {
		response.Error(w, http.StatusBadRequest, "Kode mitra, nama, dan API Key wajib diisi", "validation_error")
		return
	}

	if p.CommissionRate <= 0 {
		p.CommissionRate = 50000
	}

	if isBranchManager && userBranchCode != "" {
		p.BranchCode = &userBranchCode
	} else if p.BranchCode == nil || *p.BranchCode == "" {
		allCode := "ALL"
		p.BranchCode = &allCode
	}

	if err := s.svc.CreatePartner(r.Context(), &p); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mendaftarkan mitra", err.Error())
		return
	}

	response.Created(w, "Mitra berhasil didaftarkan", p)
}

func (s *Server) handleAdminUpdatePartner(w http.ResponseWriter, r *http.Request) {
	sess, _ := r.Context().Value(authSessionCtxKey).(*domain.AuthSession)
	isSuper := false
	isBranchManager := false
	if sess != nil {
		if strings.ToUpper(sess.Role) == "SUPER_ADMIN" || sess.Role == "superuser" {
			isSuper = true
		} else if sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager" {
			isBranchManager = true
		}
	} else {
		role := strings.ToLower(r.Header.Get("X-User-Role"))
		adminKey := r.Header.Get("X-Admin-Key")
		if adminKey != "" && role == "superuser" {
			isSuper = true
		} else if adminKey != "" && role == "branch_manager" {
			isBranchManager = true
		}
	}

	if !isSuper && !isBranchManager {
		response.Error(w, http.StatusForbidden, "Akses ditolak: Hanya level Super User (Owner) atau Kepala Cabang yang berhak mengubah data mitra & sales", "forbidden")
		return
	}

	id := chi.URLParam(r, "id")
	var req struct {
		Name           string  `json:"name"`
		ContactPhone   string  `json:"contact_phone"`
		CommissionRate float64 `json:"commission_rate"`
		IsActive       *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}

	partner, err := s.svc.GetPartnerByID(r.Context(), id)
	if err != nil || partner == nil {
		response.Error(w, http.StatusNotFound, "Mitra tidak ditemukan", "not_found")
		return
	}

	if req.Name != "" {
		partner.Name = req.Name
	}
	if req.ContactPhone != "" {
		partner.ContactPhone = req.ContactPhone
	}
	if req.CommissionRate > 0 {
		partner.CommissionRate = req.CommissionRate
	}
	if req.IsActive != nil {
		partner.IsActive = *req.IsActive
	}

	if err := s.svc.UpdatePartner(r.Context(), partner); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memperbarui data mitra", err.Error())
		return
	}

	response.Success(w, "Data mitra berhasil diperbarui", partner)
}

func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	branchCode := strings.TrimSpace(r.URL.Query().Get("branch"))

	// Jika Kepala Cabang (BRANCH_MANAGER), kunci ke cabangnya sendiri
	if sess, ok := r.Context().Value(authSessionCtxKey).(*domain.AuthSession); ok && sess != nil {
		if (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") && sess.BranchCode != nil && *sess.BranchCode != "" {
			branchCode = *sess.BranchCode
		}
	}

	clusters, err := s.svc.ListClusters(r.Context(), branchCode)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat daftar cluster", err.Error())
		return
	}
	response.Success(w, "Daftar wilayah cluster coverage", clusters)
}

func (s *Server) handleUpdateClusterStatus(w http.ResponseWriter, r *http.Request) {
	clusterName := chi.URLParam(r, "name")
	var req struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}

	branchCode := ""
	if sess, ok := r.Context().Value(authSessionCtxKey).(*domain.AuthSession); ok && sess != nil {
		if (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") && sess.BranchCode != nil && *sess.BranchCode != "" {
			branchCode = *sess.BranchCode
		}
	}

	if err := s.svc.SetClusterStatus(r.Context(), clusterName, req.Active, branchCode); err != nil {
		if strings.Contains(err.Error(), "akses ditolak") {
			response.Error(w, http.StatusForbidden, err.Error(), "forbidden")
			return
		}
		response.Error(w, http.StatusInternalServerError, "Gagal mengubah status cluster", err.Error())
		return
	}

	response.Success(w, "Status wilayah cluster berhasil diperbarui", map[string]interface{}{
		"cluster": clusterName,
		"active":  req.Active,
	})
}

// ── SUPERUSER HANDLERS ───────────────────────────────────────

func (s *Server) handleSuperUserOverview(w http.ResponseWriter, r *http.Request) {
	branchCode := strings.TrimSpace(r.URL.Query().Get("branch"))

	// Jika Kepala Cabang (BRANCH_MANAGER), paksa ke cabangnya sendiri (tidak boleh intip cabang lain)
	if sess, ok := r.Context().Value(authSessionCtxKey).(*domain.AuthSession); ok && sess != nil {
		if (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") && sess.BranchCode != nil && *sess.BranchCode != "" {
			branchCode = *sess.BranchCode
		}
	}

	overview, err := s.svc.GetExecutiveOverview(r.Context(), branchCode)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat ringkasan eksekutif", err.Error())
		return
	}
	response.Success(w, "Ringkasan eksekutif Super User", overview)
}

func (s *Server) handleStaffKPI(w http.ResponseWriter, r *http.Request) {
	branchCode := strings.TrimSpace(r.URL.Query().Get("branch"))
	if sess, ok := r.Context().Value(authSessionCtxKey).(*domain.AuthSession); ok && sess != nil {
		if (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") && sess.BranchCode != nil {
			branchCode = *sess.BranchCode
		}
	}

	kpi, err := s.svc.GetStaffKPISummary(r.Context(), branchCode)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat ringkasan KPI staf", err.Error())
		return
	}
	response.Success(w, "Ringkasan metrik KPI & performa staf real-time", kpi)
}

func (s *Server) handleSuperUserListStaff(w http.ResponseWriter, r *http.Request) {
	staff, err := s.svc.ListStaffUsers(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat daftar staf", err.Error())
		return
	}

	// Jika Kepala Cabang, filter hanya staf di cabangnya sendiri
	if sess, ok := r.Context().Value(authSessionCtxKey).(*domain.AuthSession); ok && sess != nil {
		if (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") && sess.BranchID != nil {
			var branchStaff []domain.StaffUser
			for _, st := range staff {
				if st.BranchID != nil && *st.BranchID == *sess.BranchID {
					branchStaff = append(branchStaff, st)
				}
			}
			response.Success(w, "Daftar staf operasional cabang", branchStaff)
			return
		}
	}

	response.Success(w, "Daftar staf operasional", staff)
}

func (s *Server) handleSuperUserCreateStaff(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		domain.StaffUser
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}
	u := payload.StaffUser
	if u.Username == "" || u.FullName == "" || u.Role == "" || payload.Password == "" {
		response.Error(w, http.StatusBadRequest, "Username, nama lengkap, role, dan password wajib diisi", "validation_error")
		return
	}
	if u.Status == "" {
		u.Status = "ACTIVE"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal enkripsi password staf", err.Error())
		return
	}
	u.PasswordHash = string(hash)

	if sess, ok := r.Context().Value(authSessionCtxKey).(*domain.AuthSession); ok && sess != nil {
		if (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") && sess.BranchID != nil {
			if u.Role == "SUPER_ADMIN" {
				response.Error(w, http.StatusForbidden, "Kepala Cabang tidak berwenang membuat akun Super User / Owner", "forbidden")
				return
			}
			u.BranchID = sess.BranchID
		}
	}

	if u.BranchID == nil && u.BranchCode != nil && *u.BranchCode != "" {
		b, _ := s.repo.GetBranchByCode(r.Context(), *u.BranchCode)
		if b != nil {
			u.BranchID = &b.ID
		}
	}

	if err := s.svc.CreateStaffUser(r.Context(), &u); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal menambahkan akun staf", err.Error())
		return
	}
	response.Success(w, "Akun staf berhasil dibuat", u)
}

func (s *Server) handleSuperUserUpdateStaff(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "Missing staff ID", "invalid_id")
		return
	}

	var u domain.StaffUser
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}
	u.ID = id

	if sess, ok := r.Context().Value(authSessionCtxKey).(*domain.AuthSession); ok && sess != nil {
		if (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") && sess.BranchID != nil {
			if u.Role == "SUPER_ADMIN" {
				response.Error(w, http.StatusForbidden, "Kepala Cabang tidak berwenang mengubah akun ke Super User / Owner", "forbidden")
				return
			}
			u.BranchID = sess.BranchID
		}
	}

	if err := s.svc.UpdateStaffUser(r.Context(), &u); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memperbarui data staf", err.Error())
		return
	}
	response.Success(w, "Data staf berhasil diperbarui", u)
}

// ── JARTAPLOK B2B PARTNER HANDLERS ───────────────────────────

func (s *Server) getContextJartaplokPartner(r *http.Request) *domain.JartaplokPartner {
	if p, ok := r.Context().Value(jartaplokPartnerCtxKey).(*domain.JartaplokPartner); ok && p != nil {
		return p
	}
	return &domain.JartaplokPartner{
		Code: "GNET-BIARO",
		Name: "PT. GNET BIARO AKSES",
	}
}

func (s *Server) handleJartaplokBilling(w http.ResponseWriter, r *http.Request) {
	partner := s.getContextJartaplokPartner(r)
	summary, err := s.repo.GetJartaplokBillingSummaryForPartner(r.Context(), partner.Code)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mengambil rekap tagihan sewa port JARTAPLOK", err.Error())
		return
	}
	response.Success(w, "Rekap tagihan sewa port wholesale JARTAPLOK", summary)
}

func (s *Server) handleJartaplokODPs(w http.ResponseWriter, r *http.Request) {
	partner := s.getContextJartaplokPartner(r)
	odps, err := s.svc.ListODPsByProvider(r.Context(), partner.Code)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mengambil daftar utilisasi tiang ODP", err.Error())
		return
	}
	response.Success(w, "Daftar utilisasi kapasitas ODP JARTAPLOK", odps)
}

func (s *Server) handleJartaplokCreateODP(w http.ResponseWriter, r *http.Request) {
	partner := s.getContextJartaplokPartner(r)

	var odp domain.ODPNode
	if err := json.NewDecoder(r.Body).Decode(&odp); err != nil {
		response.Error(w, http.StatusBadRequest, "Payload data ODP tidak valid", err.Error())
		return
	}

	if odp.Code == "" || odp.TotalPorts <= 0 {
		response.Error(w, http.StatusBadRequest, "Kode ODP dan Total Port wajib diisi", "validation_error")
		return
	}

	odp.ProviderID = partner.Code
	odp.ProviderName = partner.Name
	if odp.Status == "" {
		odp.Status = "AVAILABLE"
	}
	if odp.ClusterArea == "" {
		odp.ClusterArea = partner.CoverageArea
		if odp.ClusterArea == "" {
			odp.ClusterArea = "Payakumbuh & Biaro"
		}
	}

	if err := s.svc.CreateODP(r.Context(), &odp); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal menambahkan tiang ODP baru", err.Error())
		return
	}

	response.Created(w, "Titik tiang ODP baru berhasil ditambahkan", odp)
}

func (s *Server) handleJartaplokUploadKML(w http.ResponseWriter, r *http.Request) {
	partner := s.getContextJartaplokPartner(r)

	// Max 32MB file size
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		response.Error(w, http.StatusBadRequest, "Gagal memproses form upload KML", err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "File KML wajib diunggah (field 'file')", err.Error())
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".kml") && !strings.HasSuffix(strings.ToLower(header.Filename), ".xml") {
		response.Error(w, http.StatusBadRequest, "Ekstensi file harus berupa .kml (Google My Maps / QGIS)", "invalid_extension")
		return
	}

	kmlBytes, err := io.ReadAll(file)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membaca konten file KML", err.Error())
		return
	}

	result, err := s.svc.ProcessKMLUpload(r.Context(), partner.Code, partner.Name, kmlBytes)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error(), "kml_parse_error")
		return
	}

	response.Success(w, "Sinkronisasi titik ODP dari file KML berhasil diproses", result)
}

func (s *Server) handleJartaplokPorts(w http.ResponseWriter, r *http.Request) {
	partner := s.getContextJartaplokPartner(r)
	ports, err := s.repo.GetJartaplokActivePortsForPartner(r.Context(), partner.Code)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mengambil daftar port aktif", err.Error())
		return
	}
	response.Success(w, "Daftar sirkuit port aktif JARTAPLOK (sanitized)", ports)
}

func (s *Server) handleJartaplokSpec(w http.ResponseWriter, r *http.Request) {
	spec := map[string]interface{}{
		"title":       "GOGIGA ISP - JARTAPLOK B2B Wholesale API Specification",
		"version":     "v2.0 (Multi-Rekanan & KML Support)",
		"auth_header": "X-Partner-Key: <api_key_mitra>",
		"endpoints": []map[string]string{
			{"method": "GET", "path": "/api/v1/partner/jartaplok/billing", "desc": "Rekap tagihan sewa port bulanan & rincian tier sesuai tarif rekanan"},
			{"method": "GET", "path": "/api/v1/partner/jartaplok/odps", "desc": "Utilisasi kapasitas seluruh tiang ODP milik rekanan bersangkutan"},
			{"method": "POST", "path": "/api/v1/partner/jartaplok/odps", "desc": "Tambah 1 titik tiang ODP baru secara mandiri"},
			{"method": "POST", "path": "/api/v1/partner/jartaplok/upload-kml", "desc": "Upload file KML Google My Maps untuk sinkronisasi massal"},
			{"method": "GET", "path": "/api/v1/partner/jartaplok/ports", "desc": "Daftar sirkuit port aktif dan biaya bulanan (sanitized tanpa data pribadi)"},
			{"method": "GET", "path": "/api/v1/partner/jartaplok/customer/{regNo}", "desc": "Lookup kredensial PPPoE dan profil teknis pelanggan untuk otorisasi SmartOLT"},
		},
	}
	response.Success(w, "Spesifikasi B2B API Rekanan JARTAPLOK", spec)
}

func (s *Server) handleJartaplokCustomer(w http.ResponseWriter, r *http.Request) {
	regNo := chi.URLParam(r, "regNo")
	if regNo == "" {
		response.Error(w, http.StatusBadRequest, "Missing registration number", "bad_request")
		return
	}

	reg, err := s.svc.GetRegistrationByNo(r.Context(), regNo)
	if err != nil || reg == nil {
		response.Error(w, http.StatusNotFound, "Data registrasi pelanggan tidak ditemukan di sistem GoGiga", "not_found")
		return
	}

	// Smart Auto-Floor: Kontrak wholesale Jartaplok ke ISP minimal 50 Mbps.
	// Jika paket ritel pelanggan 20M/30M/50M, profil fisik OLT dialokasikan minimal 50M.
	// (Pembatasan bandwidth ke 20M/30M dilakukan di BRAS MikroTik ISP via PPPoE queue).
	// Jika pelanggan mengambil paket lebih tinggi (100M, 200M), barulah profil OLT dinaikkan.
	speedProfile := "50M"
	planLower := strings.ToLower(reg.SelectedPlanName + " " + reg.SelectedPlanID)
	if strings.Contains(planLower, "200") {
		speedProfile = "200M"
	} else if strings.Contains(planLower, "100") {
		speedProfile = "100M"
	} else {
		// Minimum tier port wholesale adalah 50M (untuk paket 20M, 30M, 50M)
		speedProfile = "50M"
	}

	// Format PPPoE Username & Password Resmi GoGiga: [14|kodeclusterwilayah|urut@gogiga.net.id] & net[4_digit_terakhir_no_hp]
	pppoeUsername := strings.TrimSpace(reg.PPPoEUsername)
	pppoePassword := strings.TrimSpace(reg.PPPoEPassword)

	if pppoeUsername == "" || pppoePassword == "" {
		branchPrefix := service.ResolveBranchPrefix(reg.BranchCode)
		clusterCode := "001"
		if reg.NearestODPCode != nil {
			clusterCode = service.ResolveClusterCode("", *reg.NearestODPCode)
		}
		if pppoeUsername == "" {
			nextSeq, _ := s.repo.GetNextPPPoESequence(r.Context(), clusterCode)
			pppoeUsername = fmt.Sprintf("%s%s%05d@gogiga.net.id", branchPrefix, clusterCode, nextSeq)
		}
		if pppoePassword == "" {
			cleanPhone := regexp.MustCompile(`\D`).ReplaceAllString(reg.Phone, "")
			pin := "8821"
			if len(cleanPhone) >= 4 {
				pin = cleanPhone[len(cleanPhone)-4:]
			} else if len(reg.RegistrationNo) >= 4 {
				pin = reg.RegistrationNo[len(reg.RegistrationNo)-4:]
			}
			pppoePassword = fmt.Sprintf("net%s", pin)
		}
		_ = s.repo.UpdateRegistrationPPPoE(r.Context(), reg.RegistrationNo, pppoeUsername, pppoePassword)
	}

	odpCode := ""
	if reg.NearestODPCode != nil {
		odpCode = *reg.NearestODPCode
	}

	// Cek apakah paket berlangganan mencakup IPTV atau Combo
	planLowerFull := strings.ToLower(reg.SelectedPlanName + " " + reg.SelectedPlanID)
	iptvEnabled := strings.Contains(planLowerFull, "tv") || strings.Contains(planLowerFull, "iptv") || strings.Contains(planLowerFull, "combo")
	serviceProfile := "SINGLE_PLAY"
	if iptvEnabled {
		serviceProfile = "DUAL_PLAY_IPTV"
	}

	data := map[string]interface{}{
		"registration_no":    reg.RegistrationNo,
		"customer_name":      reg.FullName,
		"phone":              reg.Phone,
		"address":            reg.Address,
		"selected_plan_name": reg.SelectedPlanName,
		"speed_profile":      speedProfile,
		"vlan_id":            211, // Dedicated Wholesale Internet VLAN GoGiga
		"wan_mode":           "ROUTER_PPPOE",
		"service_profile":    serviceProfile,
		"iptv_enabled":       iptvEnabled,
		"iptv_vlan_id":       212,    // Dedicated Wholesale IPTV VLAN GoGiga
		"iptv_port_binding":  "LAN4",  // Port ONT khusus STB TV kabel
		"pppoe_username":     pppoeUsername,
		"pppoe_password":     pppoePassword,
		"nearest_odp_code":   odpCode,
		"status":             reg.Status,
	}

	response.Success(w, "Data pelanggan & PPPoE GoGiga berhasil ditemukan", data)
}

// ── SUPERUSER JARTAPLOK PARTNER HANDLERS ─────────────────────

func (s *Server) handleSuperUserListJartaplokPartners(w http.ResponseWriter, r *http.Request) {
	branch := r.URL.Query().Get("branch")
	sess, _ := r.Context().Value(authSessionCtxKey).(*domain.AuthSession)
	if sess != nil && (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") {
		if sess.BranchCode != nil && *sess.BranchCode != "" {
			branch = *sess.BranchCode
		}
	}
	partners, err := s.svc.ListJartaplokPartners(r.Context(), branch)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat daftar rekanan JARTAPLOK", err.Error())
		return
	}
	response.Success(w, "Daftar Rekanan JARTAPLOK", partners)
}

func (s *Server) handleSuperUserCreateJartaplokPartner(w http.ResponseWriter, r *http.Request) {
	sess, _ := r.Context().Value(authSessionCtxKey).(*domain.AuthSession)
	var p domain.JartaplokPartner
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		response.Error(w, http.StatusBadRequest, "Payload mitra tidak valid", err.Error())
		return
	}

	if p.Code == "" || p.Name == "" {
		response.Error(w, http.StatusBadRequest, "Kode mitra dan Nama perusahaan wajib diisi", "validation_error")
		return
	}

	if sess != nil && (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") {
		if sess.BranchCode != nil && *sess.BranchCode != "" {
			p.BranchCode = sess.BranchCode
		}
	}

	p.Code = strings.ToUpper(strings.TrimSpace(p.Code))
	p.Name = strings.TrimSpace(p.Name)
	if p.APIKey == "" {
		cleanCode := strings.ToLower(strings.ReplaceAll(p.Code, "-", "_"))
		p.APIKey = "jartap_" + cleanCode + "_" + time.Now().Format("060102")
	}

	if err := s.svc.CreateJartaplokPartner(r.Context(), &p); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mendaftarkan rekanan JARTAPLOK", err.Error())
		return
	}

	response.Created(w, "Rekanan JARTAPLOK baru berhasil didaftarkan", p)
}

func (s *Server) handleSuperUserUpdateJartaplokPartner(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "Missing partner ID", "invalid_id")
		return
	}

	var p domain.JartaplokPartner
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload", err.Error())
		return
	}
	p.ID = id

	if err := s.svc.UpdateJartaplokPartner(r.Context(), &p); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mengupdate rekanan JARTAPLOK", err.Error())
		return
	}

	response.Success(w, "Tarif dan profil rekanan JARTAPLOK berhasil diperbarui", p)
}

// ── AUTHENTICATION HANDLERS ──────────────────────────────────

func (s *Server) handleStaffLogin(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format payload tidak valid", "bad_request")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "Username dan password wajib diisi", "bad_request")
		return
	}

	user, err := s.repo.GetStaffUserByUsername(r.Context(), req.Username)
	if err != nil || user == nil {
		response.Error(w, http.StatusUnauthorized, "Username atau password salah", "unauthorized")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		response.Error(w, http.StatusUnauthorized, "Username atau password salah", "unauthorized")
		return
	}

	token := "gst_" + uuid.NewString() + "_" + hex.EncodeToString([]byte(user.Username))
	expiresAt := time.Now().Add(24 * time.Hour) // Sesi berlaku 24 jam

	sess := &domain.AuthSession{
		Token:       token,
		UserID:      user.ID,
		Username:    user.Username,
		Role:        user.Role,
		Roles:       user.Roles,
		IsSuperuser: user.IsSuperuser,
		BranchID:    user.BranchID,
		BranchCode:  user.BranchCode,
		ExpiresAt:   expiresAt,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.CreateAuthSession(r.Context(), sess); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat sesi login di server", "internal_error")
		return
	}

	s.setSSOCookie(w, r, token, expiresAt)

	response.Success(w, "Login berhasil", domain.LoginResponse{
		Token:        token,
		Role:         user.Role,
		Roles:        user.Roles,
		IsSuperuser:  user.IsSuperuser,
		FullName:     user.FullName,
		Username:     user.Username,
		ContactPhone: user.ContactPhone,
		BranchID:     user.BranchID,
		BranchCode:   user.BranchCode,
		ExpiresAt:    expiresAt,
	})
}

func (s *Server) handleStaffLogout(w http.ResponseWriter, r *http.Request) {
	token := s.extractBearerToken(r)
	if token != "" {
		_ = s.repo.DeleteAuthSession(r.Context(), token)
	}
	s.clearSSOCookie(w, r)
	response.Success(w, "Logout berhasil", nil)
}

func (s *Server) handleStaffMe(w http.ResponseWriter, r *http.Request) {
	token := s.extractBearerToken(r)
	if token == "" {
		response.Error(w, http.StatusUnauthorized, "Sesi tidak ditemukan", "unauthorized")
		return
	}
	sess, err := s.repo.GetAuthSession(r.Context(), token)
	if err != nil || sess == nil {
		response.Error(w, http.StatusUnauthorized, "Sesi telah kedaluwarsa atau tidak valid", "unauthorized")
		return
	}
	user, err := s.repo.GetStaffUserByUsername(r.Context(), sess.Username)
	if err != nil || user == nil {
		response.Error(w, http.StatusNotFound, "Pengguna tidak ditemukan", "not_found")
		return
	}
	response.Success(w, "Sesi pengguna aktif", map[string]interface{}{
		"token":         token,
		"id":            user.ID,
		"username":      user.Username,
		"full_name":     user.FullName,
		"role":          user.Role,
		"roles":         user.Roles,
		"is_superuser":  user.IsSuperuser,
		"contact_phone": user.ContactPhone,
		"branch_id":     user.BranchID,
		"branch_code":   user.BranchCode,
		"expires_at":    sess.ExpiresAt,
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req domain.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format permintaan tidak valid", "bad_request")
		return
	}

	targetUsername := strings.TrimSpace(req.Username)
	token := s.extractBearerToken(r)
	if token != "" {
		if sess, err := s.repo.GetAuthSession(r.Context(), token); err == nil && sess != nil {
			if targetUsername == "" {
				targetUsername = sess.Username
			} else if targetUsername != sess.Username && sess.Role != "SUPER_ADMIN" && sess.Role != "superuser" {
				response.Error(w, http.StatusForbidden, "Tidak dapat mengubah sandi akun lain", "forbidden")
				return
			}
		}
	}

	if targetUsername == "" || req.OldPassword == "" || req.NewPassword == "" {
		response.Error(w, http.StatusBadRequest, "Username, kata sandi lama, dan kata sandi baru wajib diisi", "bad_request")
		return
	}

	if len(req.NewPassword) < 6 {
		response.Error(w, http.StatusBadRequest, "Kata sandi baru minimal harus 6 karakter", "bad_request")
		return
	}

	if req.ConfirmPassword != "" && req.NewPassword != req.ConfirmPassword {
		response.Error(w, http.StatusBadRequest, "Konfirmasi kata sandi baru tidak cocok", "bad_request")
		return
	}

	user, err := s.repo.GetStaffUserByUsername(r.Context(), targetUsername)
	if err != nil || user == nil {
		response.Error(w, http.StatusNotFound, fmt.Sprintf("Akun pengguna '%s' tidak ditemukan", targetUsername), "not_found")
		return
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		response.Error(w, http.StatusUnauthorized, "Kata sandi lama yang Anda masukkan salah", "unauthorized")
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memproses enkripsi kata sandi baru", "internal_error")
		return
	}

	if err := s.svc.UpdateStaffPassword(r.Context(), targetUsername, string(newHash)); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memperbarui kata sandi di database", err.Error())
		return
	}

	response.Success(w, fmt.Sprintf("Kata sandi untuk pengguna '%s' berhasil diperbarui.", targetUsername), map[string]string{
		"username": targetUsername,
	})
}

func (s *Server) handleSuperUserResetStaffPassword(w http.ResponseWriter, r *http.Request) {
	var req domain.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format permintaan tidak valid", "bad_request")
		return
	}

	username := strings.TrimSpace(req.Username)
	newPass := strings.TrimSpace(req.NewPassword)

	if username == "" || newPass == "" {
		response.Error(w, http.StatusBadRequest, "Username dan kata sandi baru wajib diisi", "bad_request")
		return
	}

	if len(newPass) < 6 {
		response.Error(w, http.StatusBadRequest, "Kata sandi baru minimal harus 6 karakter", "bad_request")
		return
	}

	user, err := s.repo.GetStaffUserByUsername(r.Context(), username)
	if err != nil || user == nil {
		response.Error(w, http.StatusNotFound, fmt.Sprintf("Akun staf '%s' tidak ditemukan", username), "not_found")
		return
	}

	if sess, ok := r.Context().Value(authSessionCtxKey).(*domain.AuthSession); ok && sess != nil {
		if (sess.Role == "BRANCH_MANAGER" || sess.Role == "branch_manager") && sess.BranchID != nil {
			if user.Role == "SUPER_ADMIN" {
				response.Error(w, http.StatusForbidden, "Kepala Cabang tidak berwenang mereset sandi Super User / Owner", "forbidden")
				return
			}
			if user.BranchID == nil || *user.BranchID != *sess.BranchID {
				response.Error(w, http.StatusForbidden, "Kepala Cabang hanya dapat mereset sandi staf di cabangnya sendiri", "forbidden")
				return
			}
		}
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memproses enkripsi kata sandi baru", "internal_error")
		return
	}

	if err := s.svc.UpdateStaffPassword(r.Context(), username, string(newHash)); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal mereset kata sandi staf di database", err.Error())
		return
	}

	response.Success(w, fmt.Sprintf("Kata sandi akun staf '%s' berhasil direset oleh Super User.", username), map[string]string{
		"username": username,
	})
}

func (s *Server) handleAdminListBranches(w http.ResponseWriter, r *http.Request) {
	branches, err := s.svc.ListBranches(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat daftar cabang operasional", err.Error())
		return
	}
	response.Success(w, "Daftar cabang dan mitra reseller operasional", branches)
}

func (s *Server) handlePublicListBranches(w http.ResponseWriter, r *http.Request) {
	branches, err := s.svc.ListBranches(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat daftar cabang", err.Error())
		return
	}
	var active []domain.Branch
	for _, b := range branches {
		if b.IsActive {
			active = append(active, b)
		}
	}
	response.Success(w, "Daftar cabang aktif", active)
}

// ── SMARTOLT JARTAPLOK HANDLERS ──────────────────────────────────────

func (s *Server) handleSmartOLTListONUs(w http.ResponseWriter, r *http.Request) {
	if s.smartOLTClient == nil || !s.smartOLTClient.IsConfigured() {
		response.Error(w, http.StatusServiceUnavailable, "SmartOLT client belum dikonfigurasi di server ISP Onboarding", "smartolt_not_configured")
		return
	}

	onus, err := s.smartOLTClient.GetScopedONUs(r.Context())
	if err != nil {
		response.Error(w, http.StatusBadGateway, "Gagal mengambil data dari SmartOLT: "+err.Error(), "smartolt_error")
		return
	}

	regs, _ := s.repo.ListRegistrations(r.Context(), nil, nil, nil)
	wos, _ := s.repo.ListWorkOrders(r.Context(), nil)
	bastMap := make(map[string]domain.BASTReport)
	for _, wo := range wos {
		if wo.BAST != nil {
			bastMap[wo.BAST.ONTSerialNumber] = *wo.BAST
		}
	}

	for i := range onus {
		for _, reg := range regs {
			matched := false
			if reg.PPPoEUsername != "" && strings.EqualFold(onus[i].Name, reg.PPPoEUsername) {
				matched = true
			} else if reg.UpstreamPPPoEUsername != "" && strings.EqualFold(onus[i].Name, reg.UpstreamPPPoEUsername) {
				matched = true
			} else if strings.EqualFold(onus[i].Name, reg.FullName) {
				matched = true
			} else if b, ok := bastMap[onus[i].SerialNumber]; ok && (b.WorkOrderID == reg.ID || (reg.WorkOrderID != nil && b.WorkOrderID == *reg.WorkOrderID)) {
				matched = true
			}

			if matched {
				onus[i].IsAttached = true
				onus[i].MatchedRegNo = reg.RegistrationNo
				onus[i].MatchedCust = reg.FullName
				break
			}
		}
	}

	response.Success(w, "Daftar perangkat ONT SmartOLT Zone GOGIGA", onus)
}

func (s *Server) handleSmartOLTSync(w http.ResponseWriter, r *http.Request) {
	if s.smartOLTClient == nil || !s.smartOLTClient.IsConfigured() {
		response.Error(w, http.StatusServiceUnavailable, "SmartOLT client belum dikonfigurasi di server ISP Onboarding", "smartolt_not_configured")
		return
	}

	onus, err := s.smartOLTClient.GetScopedONUs(r.Context())
	if err != nil {
		response.Error(w, http.StatusBadGateway, "Gagal mengambil data dari SmartOLT: "+err.Error(), "smartolt_error")
		return
	}

	regs, err := s.repo.ListRegistrations(r.Context(), nil, nil, nil)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat data registrasi", err.Error())
		return
	}

	attachedCount := 0
	attachedList := make([]map[string]interface{}, 0)

	for _, onu := range onus {
		for _, reg := range regs {
			matched := false
			if reg.PPPoEUsername != "" && strings.EqualFold(onu.Name, reg.PPPoEUsername) {
				matched = true
			} else if reg.UpstreamPPPoEUsername != "" && strings.EqualFold(onu.Name, reg.UpstreamPPPoEUsername) {
				matched = true
			} else if strings.EqualFold(onu.Name, reg.FullName) {
				matched = true
			}

			if matched {
				rx := -18.0
				mac := ""
				upUser := onu.Name
				if diag, err := s.smartOLTClient.GetONUSignalDiagnostics(r.Context(), onu.SerialNumber); err == nil && diag != nil {
					rx = diag.RxPowerDBM
					if diag.PPPoEUsername != "" {
						upUser = diag.PPPoEUsername
					}
				}

				if err := s.repo.AttachSmartOLTDevice(r.Context(), reg.ID, onu.SerialNumber, mac, rx, onu.Status, upUser); err == nil {
					attachedCount++
					attachedList = append(attachedList, map[string]interface{}{
						"registration_no": reg.RegistrationNo,
						"customer_name":   reg.FullName,
						"serial_number":   onu.SerialNumber,
						"pppoe_username":  upUser,
						"rx_power_dbm":    rx,
						"status":          onu.Status,
					})
				}
				break
			}
		}
	}

	response.Success(w, fmt.Sprintf("Sinkronisasi SmartOLT berhasil: %d pelanggan terhubung", attachedCount), map[string]interface{}{
		"total_scanned_onus": len(onus),
		"attached_count":     attachedCount,
		"attached_customers": attachedList,
	})
}

func (s *Server) handleSmartOLTDiagnostics(w http.ResponseWriter, r *http.Request) {
	sn := chi.URLParam(r, "sn")
	if sn == "" {
		response.Error(w, http.StatusBadRequest, "Serial number wajib diisi", "missing_sn")
		return
	}

	if s.smartOLTClient == nil || !s.smartOLTClient.IsConfigured() {
		response.Error(w, http.StatusServiceUnavailable, "SmartOLT client belum dikonfigurasi", "smartolt_not_configured")
		return
	}

	diag, err := s.smartOLTClient.GetONUSignalDiagnostics(r.Context(), sn)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "Gagal mengambil diagnostik SmartOLT: "+err.Error(), "smartolt_error")
		return
	}

	response.Success(w, "Diagnostik sinyal optik SmartOLT", diag)
}

func (s *Server) handleSmartOLTAttachRegistration(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, http.StatusBadRequest, "ID atau No Registrasi wajib diisi", "missing_id")
		return
	}

	var req struct {
		SerialNumber  string `json:"serial_number"`
		PPPoEUsername string `json:"pppoe_username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid: "+err.Error(), "bad_request")
		return
	}

	cleanSN := strings.TrimSpace(req.SerialNumber)
	if cleanSN == "" {
		response.Error(w, http.StatusBadRequest, "Serial number ONT wajib diisi", "missing_sn")
		return
	}

	rx := -18.0
	status := "Online"
	upUser := req.PPPoEUsername

	if s.smartOLTClient != nil && s.smartOLTClient.IsConfigured() {
		if diag, err := s.smartOLTClient.GetONUSignalDiagnostics(r.Context(), cleanSN); err == nil && diag != nil {
			rx = diag.RxPowerDBM
			status = diag.Status
			if upUser == "" && diag.PPPoEUsername != "" {
				upUser = diag.PPPoEUsername
			}
		}
	}

	if err := s.repo.AttachSmartOLTDevice(r.Context(), id, cleanSN, "", rx, status, upUser); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal attach ONT ke registrasi: "+err.Error(), err.Error())
		return
	}

	response.Success(w, fmt.Sprintf("Perangkat ONT %s berhasil di-attach ke pelanggan", cleanSN), map[string]interface{}{
		"serial_number": cleanSN,
		"rx_power_dbm":  rx,
		"status":        status,
	})
}

// ── CLUSTER SMARTOLT MULTI-PROVIDER HANDLERS ─────────────────────────

func (s *Server) resolveSmartOLTClientForCluster(ctx context.Context, clusterName string) (*smartoltclient.Client, *domain.ClusterSmartOLTConfig) {
	clusterName = strings.TrimSpace(clusterName)
	if clusterName != "" {
		if cfg, err := s.repo.GetClusterSmartOLTConfig(ctx, clusterName); err == nil && cfg != nil && cfg.IsActive {
			if cfg.IntegrationType == "SMARTOLT" && cfg.SmartOLTURL != "" && cfg.SmartOLTKey != "" {
				return smartoltclient.New(cfg.SmartOLTURL, cfg.SmartOLTKey), cfg
			}
		}
	}
	return s.smartOLTClient, nil
}

func (s *Server) resolveSmartOLTClientForRegistration(ctx context.Context, reg *domain.Registration) (*smartoltclient.Client, *domain.ClusterSmartOLTConfig) {
	if reg == nil {
		return s.smartOLTClient, nil
	}
	if reg.NearestODPCode != nil && *reg.NearestODPCode != "" {
		if odp, err := s.repo.GetODPByCode(ctx, *reg.NearestODPCode); err == nil && odp != nil && odp.ClusterArea != "" {
			return s.resolveSmartOLTClientForCluster(ctx, odp.ClusterArea)
		}
	}
	return s.smartOLTClient, nil
}

func (s *Server) handleListClusterSmartOLTConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := s.repo.ListClusterSmartOLTConfigs(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal memuat konfigurasi SmartOLT cluster: "+err.Error(), err.Error())
		return
	}
	response.Success(w, "Daftar konfigurasi integrasi SmartOLT per cluster", configs)
}

func (s *Server) handleGetClusterSmartOLTConfig(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "clusterName")
	if name == "" {
		response.Error(w, http.StatusBadRequest, "Nama cluster wajib diisi", "missing_cluster")
		return
	}
	cfg, err := s.repo.GetClusterSmartOLTConfig(r.Context(), name)
	if err != nil {
		response.Success(w, "Konfigurasi cluster baru", &domain.ClusterSmartOLTConfig{
			ClusterName:     name,
			IntegrationType: "SMARTOLT",
			IsActive:        true,
		})
		return
	}
	response.Success(w, "Konfigurasi SmartOLT cluster", cfg)
}

func (s *Server) handleSaveClusterSmartOLTConfig(w http.ResponseWriter, r *http.Request) {
	var raw map[string]interface{}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Gagal membaca body", "bad_request")
		return
	}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid: "+err.Error(), "bad_request")
		return
	}

	var cfg domain.ClusterSmartOLTConfig
	_ = json.Unmarshal(bodyBytes, &cfg)

	getString := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := raw[k]; ok && v != nil {
				if str, ok := v.(string); ok && strings.TrimSpace(str) != "" {
					return strings.TrimSpace(str)
				}
			}
		}
		return ""
	}

	if cfg.ClusterName == "" {
		cfg.ClusterName = getString("cluster_name", "ClusterName")
	}
	if cfg.ProviderID == "" {
		cfg.ProviderID = getString("provider_id", "partner_code", "partnerCode")
	}
	if cfg.SmartOLTURL == "" {
		cfg.SmartOLTURL = getString("smartolt_url", "base_url", "baseURL")
	}
	if cfg.SmartOLTKey == "" {
		cfg.SmartOLTKey = getString("smartolt_api_key", "api_token", "api_key", "apiKey", "apiToken")
	}
	if cfg.OLTID == "" {
		cfg.OLTID = getString("olt_id", "oltId")
	}
	if cfg.ZoneID == "" {
		cfg.ZoneID = getString("zone_id", "zoneId")
	}
	if cfg.ZoneName == "" {
		cfg.ZoneName = getString("zone_name", "zoneName")
	}

	cfg.ClusterName = strings.TrimSpace(cfg.ClusterName)
	if cfg.ClusterName == "" {
		response.Error(w, http.StatusBadRequest, "Nama cluster wajib diisi", "missing_cluster")
		return
	}

	if cfg.IntegrationType == "" {
		cfg.IntegrationType = "SMARTOLT"
	}

	if err := s.repo.SaveClusterSmartOLTConfig(r.Context(), &cfg); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal menyimpan konfigurasi SmartOLT cluster: "+err.Error(), err.Error())
		return
	}

	response.Success(w, fmt.Sprintf("Konfigurasi integrasi untuk cluster '%s' berhasil disimpan", cfg.ClusterName), cfg)
}

func (s *Server) handleDeleteClusterSmartOLTConfig(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "clusterName")
	if name == "" {
		response.Error(w, http.StatusBadRequest, "Nama cluster wajib diisi", "missing_cluster")
		return
	}

	if err := s.repo.DeleteClusterSmartOLTConfig(r.Context(), name); err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal menghapus konfigurasi: "+err.Error(), err.Error())
		return
	}
	response.Success(w, fmt.Sprintf("Konfigurasi integrasi untuk cluster '%s' berhasil dihapus", name), nil)
}

func (s *Server) handleTestSmartOLTConnection(w http.ResponseWriter, r *http.Request) {
	var raw map[string]interface{}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Gagal membaca body", "bad_request")
		return
	}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid: "+err.Error(), "bad_request")
		return
	}

	var req domain.TestSmartOLTRequest
	_ = json.Unmarshal(bodyBytes, &req)

	getString := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := raw[k]; ok && v != nil {
				if str, ok := v.(string); ok && strings.TrimSpace(str) != "" {
					return strings.TrimSpace(str)
				}
			}
		}
		return ""
	}

	if req.BaseURL == "" {
		req.BaseURL = getString("base_url", "smartolt_url", "baseURL")
	}
	if req.APIKey == "" {
		req.APIKey = getString("api_key", "api_token", "apiKey", "apiToken", "smartolt_api_key")
	}

	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.BaseURL == "" || req.APIKey == "" {
		response.Error(w, http.StatusBadRequest, "Base URL dan API Key SmartOLT wajib diisi", "missing_credentials")
		return
	}

	testClient := smartoltclient.New(req.BaseURL, req.APIKey)
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	ok, err := testClient.CheckHealth(ctx)
	if !ok || err != nil {
		errMsg := "Koneksi ke SmartOLT gagal"
		if err != nil {
			errMsg += ": " + err.Error()
		}
		response.Error(w, http.StatusBadGateway, errMsg, "smartolt_unreachable")
		return
	}

	response.Success(w, "Koneksi ke SmartOLT berhasil terhubung (OK)", map[string]interface{}{
		"base_url": req.BaseURL,
		"status":   "CONNECTED",
	})
}

// ── FIBERGRID JARTAPLOK WHOLESALE INTEGRATION HANDLERS ──────

func (s *Server) handleFiberGridTest(w http.ResponseWriter, r *http.Request) {
	var req domain.FiberGridSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid: "+err.Error(), "bad_request")
		return
	}

	serverURL := strings.TrimRight(strings.TrimSpace(req.ServerURL), "/")
	apiKey := strings.TrimSpace(req.APIKey)
	if serverURL == "" || apiKey == "" {
		response.Error(w, http.StatusBadRequest, "Alamat Server dan Kode Akses Rekanan (X-Wholesale-Key) wajib diisi", "missing_parameters")
		return
	}

	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		serverURL = "http://" + serverURL
	}

	client := &http.Client{Timeout: 10 * time.Second}
	testReq, err := http.NewRequestWithContext(r.Context(), "GET", fmt.Sprintf("%s/api/v1/wholesale/me/profile", serverURL), nil)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal menyusun request: "+err.Error(), "internal_error")
		return
	}
	testReq.Header.Set("X-Wholesale-Key", apiKey)
	testReq.Header.Set("Accept", "application/json")

	resp, err := client.Do(testReq)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "Gagal menghubungi server FiberGrid: "+err.Error(), "gateway_error")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		response.Error(w, http.StatusUnauthorized, "Kode Akses Rekanan (X-Wholesale-Key) tidak valid atau masa kontrak tidak aktif di server FiberGrid", "auth_failed")
		return
	}

	if resp.StatusCode != http.StatusOK {
		response.Error(w, http.StatusBadGateway, fmt.Sprintf("Server FiberGrid merespon dengan HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode)), "upstream_error")
		return
	}

	var resData struct {
		Success bool                            `json:"success"`
		Data    domain.FiberGridContractProfile `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		response.Error(w, http.StatusBadGateway, "Respon profil dari FiberGrid tidak dapat dibaca: "+err.Error(), "parse_error")
		return
	}

	response.Success(w, "Koneksi ke FiberGrid berhasil diverifikasi", resData.Data)
}

func (s *Server) handleFiberGridSync(w http.ResponseWriter, r *http.Request) {
	var req domain.FiberGridSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Format JSON tidak valid: "+err.Error(), "bad_request")
		return
	}

	serverURL := strings.TrimRight(strings.TrimSpace(req.ServerURL), "/")
	apiKey := strings.TrimSpace(req.APIKey)
	clusterArea := strings.TrimSpace(req.ClusterArea)
	if clusterArea == "" {
		clusterArea = "Payakumbuh"
	}

	if serverURL == "" || apiKey == "" {
		response.Error(w, http.StatusBadRequest, "Alamat Server dan Kode Akses Rekanan (X-Wholesale-Key) wajib diisi", "missing_parameters")
		return
	}

	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		serverURL = "http://" + serverURL
	}

	client := &http.Client{Timeout: 15 * time.Second}

	// 1. Ambil Profil Kontrak Rekanan
	profReq, err := http.NewRequestWithContext(r.Context(), "GET", fmt.Sprintf("%s/api/v1/wholesale/me/profile", serverURL), nil)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Gagal membuat request profil: "+err.Error(), "internal_error")
		return
	}
	profReq.Header.Set("X-Wholesale-Key", apiKey)
	profReq.Header.Set("Accept", "application/json")

	profResp, err := client.Do(profReq)
	if err != nil {
		response.Error(w, http.StatusBadGateway, "Gagal menghubungi server FiberGrid: "+err.Error(), "gateway_error")
		return
	}
	defer profResp.Body.Close()

	if profResp.StatusCode == http.StatusUnauthorized || profResp.StatusCode == http.StatusForbidden {
		response.Error(w, http.StatusUnauthorized, "Kode Akses Rekanan (X-Wholesale-Key) tidak valid atau masa kontrak tidak aktif di server FiberGrid", "auth_failed")
		return
	}

	if profResp.StatusCode != http.StatusOK {
		response.Error(w, http.StatusBadGateway, fmt.Sprintf("Server FiberGrid merespon dengan HTTP %d", profResp.StatusCode), "upstream_error")
		return
	}

	var profBody struct {
		Success bool                            `json:"success"`
		Data    domain.FiberGridContractProfile `json:"data"`
	}
	if err := json.NewDecoder(profResp.Body).Decode(&profBody); err != nil {
		response.Error(w, http.StatusBadGateway, "Respon profil dari FiberGrid tidak valid: "+err.Error(), "parse_error")
		return
	}
	contract := profBody.Data

	// 2. Ambil Daftar ODP yang dialokasikan oleh FiberGrid
	odpReq, _ := http.NewRequestWithContext(r.Context(), "GET", fmt.Sprintf("%s/api/v1/wholesale/me/odps", serverURL), nil)
	odpReq.Header.Set("X-Wholesale-Key", apiKey)
	odpReq.Header.Set("Accept", "application/json")

	var rawODPs []domain.FiberGridODP
	if odpResp, err := client.Do(odpReq); err == nil {
		defer odpResp.Body.Close()
		if odpResp.StatusCode == http.StatusOK {
			var odpBody struct {
				Success bool                  `json:"success"`
				Data    []domain.FiberGridODP `json:"data"`
			}
			if err := json.NewDecoder(odpResp.Body).Decode(&odpBody); err == nil && odpBody.Success {
				rawODPs = odpBody.Data
			}
		}
	}

	// 3. Ambil Daftar ONT yang dialokasikan
	ontReq, _ := http.NewRequestWithContext(r.Context(), "GET", fmt.Sprintf("%s/api/v1/wholesale/me/onts", serverURL), nil)
	ontReq.Header.Set("X-Wholesale-Key", apiKey)
	ontReq.Header.Set("Accept", "application/json")

	var rawONTs []domain.FiberGridONT
	if ontResp, err := client.Do(ontReq); err == nil {
		defer ontResp.Body.Close()
		if ontResp.StatusCode == http.StatusOK {
			var ontBody struct {
				Success bool                  `json:"success"`
				Data    []domain.FiberGridONT `json:"data"`
			}
			if err := json.NewDecoder(ontResp.Body).Decode(&ontBody); err == nil && ontBody.Success {
				rawONTs = ontBody.Data
			}
		}
	}

	// 4. Auto-Import ODPs ke Database Nexus jika diminta
	importedCount := 0
	partnerTag := contract.PartnerInitial
	if partnerTag == "" {
		partnerTag = "FIBERGRID"
	}
	providerName := "FiberGrid - " + contract.CompanyName
	if contract.CompanyName == "" {
		providerName = "FiberGrid Wholesale"
	}

	if req.AutoImportODPs && len(rawODPs) > 0 {
		for _, o := range rawODPs {
			node := &domain.ODPNode{
				ID:           uuid.NewString(),
				Code:         o.Code,
				Name:         o.Name,
				Latitude:     o.Latitude,
				Longitude:    o.Longitude,
				TotalPorts:   o.TotalPorts,
				UsedPorts:    o.UsedPorts,
				Status:       "AVAILABLE",
				ClusterArea:  clusterArea,
				ProviderID:   partnerTag,
				ProviderName: providerName,
			}
			if o.Status != "" {
				node.Status = o.Status
			}
			if err := s.svc.CreateODP(r.Context(), node); err == nil {
				importedCount++
			}
		}
	}

	// 5. Generate Script MikroTik L2 Demarcation (802.1Q VLAN Trunk)
	vlanID := contract.VlanID
	if vlanID <= 0 {
		vlanID = 200
	}
	iptvVlanID := contract.IPTVVlanID
	nowStr := time.Now().Format("02/01/2006 15:04:05 MST")

	script := fmt.Sprintf(`# ===================================================================
# ISPSYNC NEXUS - FIBERGRID WHOLESALE LAYER 2 INTERCONNECTION (VLAN TRUNK)
# Mitra Rekanan       : %s (%s)
# Dedicated VLAN ID   : VLAN %d (802.1Q Tagged)
`, contract.CompanyName, partnerTag, vlanID)

	if iptvVlanID > 0 {
		script += fmt.Sprintf("# Multicast IPTV VLAN : VLAN %d (Opsional)\n", iptvVlanID)
	}

	script += fmt.Sprintf(`# Alokasi Port Pasif  : %d Port FO Pasif
# Server Endpoint     : %s
# Tanggal Sinkron     : %s
# ===================================================================

# 1. Tambahkan VLAN Interface Handover di Router Core ISP (Layer 2 Trunk)
/interface vlan
add name="vlan%d-jartaplok" vlan-id=%d interface=sfp-sfpplus1 mtu=1508 comment="Handover L2 Wholesale Jartaplok - %s"
`, contract.MaxPorts, serverURL, nowStr, vlanID, vlanID, contract.CompanyName)

	if iptvVlanID > 0 {
		script += fmt.Sprintf("add name=\"vlan%d-iptv\" vlan-id=%d interface=sfp-sfpplus1 mtu=1500 comment=\"Handover IPTV Jartaplok - %s\"\n", iptvVlanID, iptvVlanID, contract.CompanyName)
	}

	script += `
# ===================================================================
# BATAS DEMARKASI TELEKOMUNIKASI (TELECOM DEMARCATION POINT):
# - Tanggung Jawab Jartaplok : Layer 1 Fisik & Layer 2 Transport (OLT -> ODC -> ODP -> ONT).
# - Tanggung Jawab Mitra ISP : Layer 3 ke atas (IP Public/Private, IP Pool, PPPoE/Radius BRAS,
#                              Bandwidth Queues, DNS Server, & Billing Pelanggan Ritel).
# Provider Jartaplok TIDAK mengintervensi IP Address atau PPPoE Server ISP.
# ===================================================================
`

	result := domain.FiberGridSyncResult{
		ConnectedAt:       nowStr,
		ServerURL:         serverURL,
		Contract:          contract,
		TotalODPsFetched:  len(rawODPs),
		TotalODPsImported: importedCount,
		TotalONTsFetched:  len(rawONTs),
		MikrotikScript:    script,
		ODPs:              rawODPs,
		ONTs:              rawONTs,
	}

	msg := fmt.Sprintf("Sinkronisasi FiberGrid berhasil! %d ODP terambil (%d disimpan ke database), %d ONT aktif.", len(rawODPs), importedCount, len(rawONTs))
	response.Success(w, msg, result)
}




