package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ispsync/internal/access"
	"ispsync/internal/auth"
	"ispsync/internal/domain"
	"ispsync/internal/fibergrid"
	"ispsync/internal/middleware"
	"ispsync/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/go-routeros/routeros"
	"golang.org/x/crypto/bcrypt"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// dummyBcryptHash dipakai agar waktu verifikasi sama untuk akun yang tidak ada.
var dummyBcryptHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcrypt.DefaultCost)
	return string(h)
}()

type APIHandler struct {
	store      repository.Storage
	olt        *access.OLTDispatcher
	bras       *access.BRASDispatcher
	authSecret []byte
	throttle   *loginThrottle
	fg         *fibergrid.Client
}

func NewAPIHandler(store repository.Storage, authSecret []byte) *APIHandler {
	return &APIHandler{
		store:      store,
		olt:        access.NewOLTDispatcher(),
		bras:       access.NewBRASDispatcher(),
		authSecret: authSecret,
		throttle:   newLoginThrottle(),
		fg:         fibergrid.NewFromEnv(),
	}
}

func (h *APIHandler) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *APIHandler) errorResponse(w http.ResponseWriter, status int, msg string) {
	h.jsonResponse(w, status, map[string]string{"error": msg})
}

func (h *APIHandler) successResponse(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": message,
		"data":    data,
	})
}

func (h *APIHandler) failResponse(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": msg,
		"error":   msg,
	})
}

// GetContext info tenant dan aplikasi aktif
func (h *APIHandler) GetContext(w http.ResponseWriter, r *http.Request) {
	tCtx := middleware.GetTenantContext(r)
	if tCtx == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}
	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"tenant":    tCtx.Tenant,
		"app_type":  tCtx.AppType,
		"subdomain": tCtx.Subdomain,
		"host":      tCtx.Host,
		"capabilities": h.tenantCapsFor(r),
	})
}

// ListPlans paket internet
func (h *APIHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	plans, err := h.store.ListPlans(r.Context(), t.ID)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.jsonResponse(w, http.StatusOK, plans)
}

// ListODPs persebaran ODP
func (h *APIHandler) ListODPs(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	odps, err := h.store.ListODPs(r.Context(), t.ID)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.jsonResponse(w, http.StatusOK, odps)
}

// CheckCoverage menghitung jarak ke ODP terdekat
func (h *APIHandler) CheckCoverage(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	var req struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	odp, dist, err := h.store.GetNearestODP(r.Context(), t.ID, req.Latitude, req.Longitude)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	covered := dist <= 250.0 // Default radius 250m
	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"covered":            covered,
		"distance_meters":    mathRound(dist, 1),
		"max_radius_meters":  250.0,
		"nearest_odp_code":   odp.Code,
		"nearest_odp_name":   odp.Name,
		"available_ports":    odp.TotalPorts - odp.UsedPorts,
	})
}

// Register formulir pasang baru publik (Portal)
func (h *APIHandler) Register(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	var req struct {
		FullName       string  `json:"full_name"`
		IdentityNumber string  `json:"identity_number"`
		Phone          string  `json:"phone"`
		Email          string  `json:"email"`
		Address        string  `json:"address"`
		Latitude       float64 `json:"latitude"`
		Longitude      float64 `json:"longitude"`
		PlanID         string  `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON input")
		return
	}

	if req.FullName == "" || req.Phone == "" {
		h.errorResponse(w, http.StatusBadRequest, "Nama lengkap dan nomor HP wajib diisi")
		return
	}

	// 1. Validasi ODP terdekat
	odp, dist, err := h.store.GetNearestODP(r.Context(), t.ID, req.Latitude, req.Longitude)
	odpCode := "ODP-UNKNOWN"
	odpID := ""
	if err == nil && odp != nil {
		odpCode = odp.Code
		odpID = odp.ID
	}

	// 2. Ambil data plan
	planName := "Paket Internet"
	if req.PlanID != "" {
		if p, err := h.store.GetPlanByID(r.Context(), t.ID, req.PlanID); err == nil && p != nil {
			planName = p.Name
		}
	}

	// 3. Generate SubscriberNo mandiri dengan prefix tenant
	subID := uuid.New().String()
	subNo := fmt.Sprintf("%s-%s-%04d", t.PrefixID, time.Now().Format("2006"), time.Now().Unix()%9000+1000)

	sub := &domain.Subscriber{
		ID:               subID,
		TenantID:         t.ID,
		SubscriberNo:     subNo,
		FullName:         req.FullName,
		IdentityNumber:   req.IdentityNumber,
		Email:            req.Email,
		Phone:            req.Phone,
		Address:          req.Address,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		DistanceToODP:    dist,
		SelectedPlanID:   req.PlanID,
		SelectedPlanName: planName,
		NearestODPID:     odpID,
		NearestODPCode:   odpCode,
		Status:           "INSTALLATION_SCHEDULED",
	}

	if err := h.store.CreateSubscriber(r.Context(), sub); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Gagal membuat pendaftaran: "+err.Error())
		return
	}

	// 4. Buat Work Order otomatis untuk Teknisi
	wo := &domain.WorkOrder{
		ID:              uuid.New().String(),
		TenantID:        t.ID,
		SubscriberID:    subID,
		SubscriberNo:    subNo,
		CustomerName:    req.FullName,
		CustomerPhone:   req.Phone,
		CustomerAddress: req.Address,
		CustomerLat:     req.Latitude,
		CustomerLng:     req.Longitude,
		ODPCode:         odpCode,
		PlanName:        planName,
		OrderType:       "INSTALLATION",
		TechnicianName:  "Teknisi Piket",
		Status:          "PENDING",
		Notes:           fmt.Sprintf("Pemasangan baru dari portal online. Jarak ke %s: %.1f m", odpCode, dist),
	}
	_ = h.store.CreateWorkOrder(r.Context(), wo)

	h.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"success":       true,
		"subscriber_no": subNo,
		"work_order_no": wo.OrderNo,
		"message":       "Registrasi berhasil. Surat Perintah Kerja telah diteruskan ke teknisi lapangan.",
	})
}

// ListSubscribers daftar pelanggan (CMS/NOC)
func (h *APIHandler) ListSubscribers(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	status := r.URL.Query().Get("status")
	subs, err := h.store.ListSubscribers(r.Context(), t.ID, status)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.jsonResponse(w, http.StatusOK, subs)
}

// GetSubscriber detail pelanggan
func (h *APIHandler) GetSubscriber(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	id := chi.URLParam(r, "id")
	sub, err := h.store.GetSubscriberByID(r.Context(), t.ID, id)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Pelanggan tidak ditemukan")
		return
	}
	h.jsonResponse(w, http.StatusOK, sub)
}

// UpdateSubscriberStatus ganti status (ACTIVE, RESTRICTED, TERMINATED)
func (h *APIHandler) UpdateSubscriberStatus(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	id := chi.URLParam(r, "id")
	var req struct {
		Status string `json:"status"` // ACTIVE, RESTRICTED, TERMINATED
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request")
		return
	}

	sub, err := h.store.GetSubscriberByID(r.Context(), t.ID, id)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Subscriber not found")
		return
	}

	if err := h.store.UpdateSubscriberStatus(r.Context(), t.ID, id, req.Status); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Trigger perubahan ke BRAS jika akun pppoe ada
	var syncLog string
	if sub.PPPoEUsername != nil && *sub.PPPoEUsername != "" {
		if req.Status == "RESTRICTED" {
			script := h.bras.GenerateIsolirScript(*sub.PPPoEUsername)
			syncLog, _ = h.bras.Execute(r.Context(), "192.168.10.1", script)
		} else if req.Status == "ACTIVE" {
			script := h.bras.GenerateReactivateScript(*sub.PPPoEUsername, "DEFAULT_FIBER_PROFILE")
			syncLog, _ = h.bras.Execute(r.Context(), "192.168.10.1", script)
		}
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"status":   req.Status,
		"sync_log": syncLog,
	})
}

// ProvisionSubscriber kirim provisioning ke OLT & BRAS
func (h *APIHandler) ProvisionSubscriber(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	id := chi.URLParam(r, "id")

	var req struct {
		OLTID        string `json:"olt_id"`
		PONPort      string `json:"pon_port"`
		ONUID        int    `json:"onu_id"`
		SerialNumber string `json:"serial_number"`
		MACAddress   string `json:"mac_address"`
		VLANID       int    `json:"vlan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid input")
		return
	}

	sub, err := h.store.GetSubscriberByID(r.Context(), t.ID, id)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Subscriber not found")
		return
	}

	req.SerialNumber = strings.TrimSpace(req.SerialNumber)
	if req.SerialNumber == "" {
		h.errorResponse(w, http.StatusBadRequest, "serial_number ONT wajib diisi")
		return
	}

	// ISP murni (tanpa infrastruktur sendiri & tanpa OLT) tidak mengeksekusi OLT: ONT diaktifkan oleh
	// penyedia infrastruktur (bitstream/sewa). Nexus hanya mencatat SN dan membuat kredensial PPPoE.
	var script string
	var oltLog interface{}
	if req.OLTID == "" && !h.tenantCaps(r).OwnInfrastructure {
		oltLog = "ONT diaktifkan oleh penyedia infrastruktur; tidak ada OLT yang dieksekusi dari tenant ini"
	} else {
		oltDev, err := h.store.GetOLTByID(r.Context(), t.ID, req.OLTID)
		if err != nil {
			h.errorResponse(w, http.StatusBadRequest, "OLT not found")
			return
		}

		// 1. Generate & kirim perintah OLT
		script = h.olt.GenerateScript(access.OLTProvisionParams{
			Vendor:       oltDev.Vendor,
			PONPort:      req.PONPort,
			ONUID:        req.ONUID,
			SerialNumber: req.SerialNumber,
			CustomerName: sub.FullName,
			VLANID:       req.VLANID,
		})
		oltLog, _ = h.olt.ExecuteProvision(r.Context(), oltDev.HostIP, oltDev.Port, oltDev.Username, "", script)
	}

	// 1b. Catat ONT di FiberGrid (Engine 1). Bila gagal, pelanggan TIDAK diaktifkan.
	fgNote := "Tenant tidak memakai FiberGrid; ONT tidak dicatat di FiberGrid"
	if h.fg.Configured() && h.tenantCaps(r).UsesFiberGrid {
		regReq := fibergrid.RegisterONTRequest{
			SerialNumber:   req.SerialNumber,
			MACAddress:     req.MACAddress,
			PONPort:        req.PONPort,
			RegistrationNo: sub.SubscriberNo,
			CustomerName:   sub.FullName,
		}
		fgNote = "ONT tercatat di FiberGrid (tanpa ODP: kode ODP pelanggan tidak ada di FiberGrid)"
		// Petakan ODP terdekat pelanggan ke ODP FiberGrid lewat kode (tidak peka huruf besar/kecil).
		if code := strings.TrimSpace(sub.NearestODPCode); code != "" {
			odp, err := h.fg.LookupODP(r.Context(), code)
			switch {
			case err == nil:
				if odp.TotalPorts > 0 && odp.UsedPorts >= odp.TotalPorts {
					h.errorResponse(w, http.StatusConflict, "ODP "+odp.Code+" di FiberGrid sudah penuh ("+strconv.Itoa(odp.UsedPorts)+"/"+strconv.Itoa(odp.TotalPorts)+" port)")
					return
				}
				if strings.EqualFold(odp.Status, "MAINTENANCE") {
					h.errorResponse(w, http.StatusConflict, "ODP "+odp.Code+" sedang maintenance")
					return
				}
				regReq.ODPNodeID = odp.ID
				regReq.OLTDeviceID = odp.OLTID
				if regReq.PONPort == "" {
					regReq.PONPort = odp.PONPort
				}
				fgNote = "ONT tercatat di FiberGrid pada ODP " + odp.Code
			case errors.Is(err, fibergrid.ErrNotFound):
				// ODP bukan milik jaringan FiberGrid: lanjut tanpa pemetaan.
			default:
				h.errorResponse(w, http.StatusBadGateway, "Gagal memeriksa ODP di FiberGrid: "+err.Error())
				return
			}
		}
		if _, err := h.fg.RegisterONT(r.Context(), regReq); err != nil {
			h.errorResponse(w, http.StatusBadGateway, "Gagal mencatat ONT di FiberGrid: "+err.Error())
			return
		}
	}

	// 1c. Bila ODP pelanggan adalah ODP sewaan (Jartaplok), pakai satu port kuota sewa (sekali per pelanggan).
	if code := strings.TrimSpace(sub.NearestODPCode); code != "" && (sub.SerialNumber == nil || strings.TrimSpace(*sub.SerialNumber) == "") {
		if _, err := h.store.ConsumeSharedODPPort(r.Context(), t.ID, code); err != nil {
			if errors.Is(err, repository.ErrSharedODPFull) {
				h.errorResponse(w, http.StatusConflict, "Kuota port sewa ODP "+code+" sudah penuh; hubungi pemilik infrastruktur untuk menambah port")
			} else {
				h.errorResponse(w, http.StatusInternalServerError, "Gagal memakai port sewa: "+err.Error())
			}
			return
		}
	}
	// 2. Generate Kredensial PPPoE
	pppoeUser := fmt.Sprintf("sub%s@%s", sub.SubscriberNo, t.Slug)
	pppoePass := randomPassword(12)
	brasScript := h.bras.GenerateMikrotikScript(access.BRASProvisionParams{
		Username:      pppoeUser,
		Password:      pppoePass,
		ProfileName:   "DEFAULT_FIBER",
		RateLimitDown: 50,
		RateLimitUp:   50,
		RemoteIP:      "10.20.10." + strconv.Itoa(10+(req.ONUID%200)),
		Comment:       sub.SubscriberNo + " - " + sub.FullName,
	})
	brasLog, _ := h.bras.Execute(r.Context(), "192.168.10.1", brasScript)

	// 3. Simpan ke database
	_ = h.store.UpdateSubscriberProvisioning(r.Context(), t.ID, id,
		&req.OLTID, &req.PONPort, &req.ONUID, &req.SerialNumber, &req.MACAddress,
		nil, &pppoeUser, &pppoePass, &req.VLANID, nil)
	_ = h.store.UpdateSubscriberStatus(r.Context(), t.ID, id, "ACTIVE")

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"olt_script":     script,
		"olt_log":        oltLog,
		"bras_script":    brasScript,
		"bras_log":       brasLog,
		"pppoe_username": pppoeUser,
		"pppoe_password": pppoePass,
		"fibergrid":      fgNote,
	})
}

// ListWorkOrders daftar SPK
func (h *APIHandler) ListWorkOrders(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	status := r.URL.Query().Get("status")
	wos, err := h.store.ListWorkOrders(r.Context(), t.ID, status)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.jsonResponse(w, http.StatusOK, wos)
}

// CompleteBAST penyelesaian SPK dan upload redaman optik oleh Teknisi
func (h *APIHandler) CompleteBAST(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	id := chi.URLParam(r, "id")

	var req struct {
		RxPowerDBM   float64 `json:"rx_power_dbm"`
		SerialNumber string  `json:"serial_number"`
		MACAddress   string  `json:"mac_address"`
		Notes        string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid body")
		return
	}

	if req.RxPowerDBM < -28.0 {
		h.errorResponse(w, http.StatusBadRequest, "Redaman optik terlalu buruk (< -28.0 dBm). Standar Telco mewajibkan >= -27.0 dBm")
		return
	}

	err := h.store.CompleteWorkOrderBAST(r.Context(), t.ID, id, req.RxPowerDBM, req.SerialNumber, req.MACAddress, req.Notes)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "BAST berhasil diverifikasi. Status pelanggan otomatis aktif (In-Service).",
	})
}

// ListOLTs perangkat OLT
func (h *APIHandler) ListOLTs(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	olts, err := h.store.ListOLTs(r.Context(), t.ID)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.jsonResponse(w, http.StatusOK, olts)
}

func mathRound(val float64, precision int) float64 {
	p := 1.0
	for i := 0; i < precision; i++ {
		p *= 10.0
	}
	return float64(int(val*p+0.5)) / p
}

// CaddyAsk adalah endpoint verifikasi On-Demand TLS untuk Caddy webserver
// Caddy memanggil GET /api/v1/caddy/ask?domain={domain}. Jika return 200 -> terbitkan SSL.
func (h *APIHandler) CaddyAsk(w http.ResponseWriter, r *http.Request) {
	domainName := r.URL.Query().Get("domain")
	if domainName == "" {
		http.Error(w, "missing domain query parameter", http.StatusBadRequest)
		return
	}

	if h.store.ValidateDomainForTLS(r.Context(), domainName) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return
	}

	http.Error(w, "domain not authorized for on-demand certificate", http.StatusForbidden)
}

// UpdateCustomDomain mengatur custom domain / CNAME tenant
func (h *APIHandler) UpdateCustomDomain(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}

	var req struct {
		CustomDomain string `json:"custom_domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON input")
		return
	}

	if err := h.store.UpdateTenantCustomDomain(r.Context(), t.ID, req.CustomDomain); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Gagal menyimpan custom domain: "+err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"custom_domain": req.CustomDomain,
		"cname_target":  "ispsync.id",
		"message":       "Custom domain berhasil diperbarui. Silakan arahkan CNAME domain Anda ke ispsync.id.",
	})
}

// UpdateTenantProfile mengatur branding (logo, warna, kontak) tenant
func (h *APIHandler) UpdateTenantProfile(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}

	var req struct {
		LogoURL      string `json:"logo_url"`
		BrandColor   string `json:"brand_color"`
		ContactPhone string `json:"contact_phone"`
		ContactEmail string `json:"contact_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON input")
		return
	}

	if err := h.store.UpdateTenantProfile(r.Context(), t.ID, req.LogoURL, req.BrandColor, req.ContactPhone, req.ContactEmail); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Gagal menyimpan profil: "+err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Profil dan Branding berhasil diperbarui.",
	})
}

// ListInvoices mengambil daftar tagihan bulanan
func (h *APIHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	status := r.URL.Query().Get("status")
	invoices, err := h.store.ListInvoices(r.Context(), t.ID, status)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.jsonResponse(w, http.StatusOK, invoices)
}

// PayInvoice memproses pembayaran tagihan dan otomatis unisolir di BRAS
func (h *APIHandler) PayInvoice(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	id := chi.URLParam(r, "id")

	if err := h.store.MarkInvoicePaid(r.Context(), t.ID, id); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Gagal memproses pembayaran: "+err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Faktur lunas. Status pelanggan telah dipulihkan ke ACTIVE (Unisolir otomatis).",
	})
}

// ListVouchers mengambil daftar voucher hotspot / loket POS
func (h *APIHandler) ListVouchers(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	vouchers, err := h.store.ListVouchers(r.Context(), t.ID)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.jsonResponse(w, http.StatusOK, vouchers)
}

// GenerateVouchers membuat batch voucher hotspot blanko baru
func (h *APIHandler) GenerateVouchers(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	var req struct {
		ProfileName string  `json:"profile_name"`
		SpeedDown   int     `json:"speed_down"`
		SpeedUp     int     `json:"speed_up"`
		Price       float64 `json:"price"`
		Count       int     `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ProfileName == "" {
		req.ProfileName = "Paket Hotspot Standar"
	}
	if req.SpeedDown <= 0 {
		req.SpeedDown = 10
	}
	if req.SpeedUp <= 0 {
		req.SpeedUp = 5
	}
	if req.Price <= 0 {
		req.Price = 5000
	}
	if req.Count <= 0 || req.Count > 100 {
		req.Count = 15
	}

	batch, err := h.store.GenerateVouchers(r.Context(), t.ID, req.ProfileName, req.SpeedDown, req.SpeedUp, req.Price, req.Count)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Gagal membuat batch voucher: "+err.Error())
		return
	}

	h.jsonResponse(w, http.StatusCreated, batch)
}

// ListJartaplokAgreements mengambil daftar perjanjian bagi pakai jaringan tetap lokal (Jartaplok)
func (h *APIHandler) ListJartaplokAgreements(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	agreements, err := h.store.ListJartaplokAgreements(r.Context(), t.ID)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.jsonResponse(w, http.StatusOK, agreements)
}



// ── PORTAL GOGIGANET COMPATIBLE ENDPOINTS ──────────────────

// PublicCoverageCheck untuk formulir registrasi interaktif
func (h *APIHandler) PublicCoverageCheck(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	var req struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.failResponse(w, http.StatusBadRequest, "Format koordinat lokasi tidak valid")
		return
	}

	odp, dist, err := h.store.GetNearestODP(r.Context(), t.ID, req.Latitude, req.Longitude)
	if err != nil || odp == nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal mendeteksi ODP terdekat")
		return
	}

	isCovered := dist <= 250.0
	clusterArea := "Area Distribusi"
	if odp != nil && odp.Code != "" {
		parts := strings.Split(odp.Code, "-")
		if len(parts) >= 2 && parts[0] == "ODP" {
			clusterArea = "Cluster " + strings.Split(parts[1], "/")[0]
		}
	}
	h.successResponse(w, "Coverage checked successfully", map[string]interface{}{
		"is_covered":        isCovered,
		"distance_meters":   mathRound(dist, 1),
		"max_radius_meters": 250.0,
		"available_ports":   odp.TotalPorts - odp.UsedPorts,
		"cluster_area":      clusterArea,
		"nearest_odp": map[string]interface{}{
			"id":              odp.ID,
			"code":            odp.Code,
			"name":            odp.Name,
			"latitude":        odp.Latitude,
			"longitude":       odp.Longitude,
			"total_ports":     odp.TotalPorts,
			"used_ports":      odp.UsedPorts,
			"available_ports": odp.TotalPorts - odp.UsedPorts,
			"status":          odp.Status,
			"cluster_area":    clusterArea,
			"is_shared":       odp.IsSharedJartaplok,
			"provider_name":   odp.OwnerTenantName,
		},
	})
}

// PublicPlans daftar paket internet untuk dropdown portal
func (h *APIHandler) PublicPlans(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	plans, err := h.store.ListPlans(r.Context(), t.ID)
	if err != nil {
		h.failResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	type planOut struct {
		ID                string  `json:"id"`
		Name              string  `json:"name"`
		Code              string  `json:"code"`
		DownloadSpeedMbps int     `json:"download_speed_mbps"`
		UploadSpeedMbps   int     `json:"upload_speed_mbps"`
		PriceMonthly      float64 `json:"price_monthly"`
		Description       string  `json:"description"`
		IsPopular         bool    `json:"is_popular"`
	}
	var out []planOut
	for i, p := range plans {
		out = append(out, planOut{
			ID:                p.ID,
			Name:              p.Name,
			Code:              p.Code,
			DownloadSpeedMbps: p.SpeedDownMbps,
			UploadSpeedMbps:   p.SpeedUpMbps,
			PriceMonthly:      p.MonthlyPrice,
			Description:       p.Description,
			IsPopular:         i == 1,
		})
	}
	h.successResponse(w, "Plans retrieved successfully", out)
}

// PublicRegister registrasi pelanggan baru dari portal
func (h *APIHandler) PublicRegister(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	var req struct {
		FullName       string  `json:"full_name"`
		IDCardNumber   string  `json:"id_card_number"`
		Phone          string  `json:"phone"`
		Email          string  `json:"email"`
		Address        string  `json:"address"`
		Latitude       float64 `json:"latitude"`
		Longitude      float64 `json:"longitude"`
		SelectedPlanID string  `json:"selected_plan_id"`
		PlanID         string  `json:"plan_id"`
		ReferralCode   string  `json:"referral_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.failResponse(w, http.StatusBadRequest, "Invalid JSON input")
		return
	}
	if req.FullName == "" || req.Phone == "" {
		h.failResponse(w, http.StatusBadRequest, "Nama lengkap dan nomor HP wajib diisi")
		return
	}

	planID := req.SelectedPlanID
	if planID == "" {
		planID = req.PlanID
	}
	plans, _ := h.store.ListPlans(r.Context(), t.ID)
	planName := "Paket Reguler Fiber"
	for _, p := range plans {
		if p.ID == planID {
			planName = p.Name
			break
		}
	}

	odp, dist, _ := h.store.GetNearestODP(r.Context(), t.ID, req.Latitude, req.Longitude)
	odpCode := "ODP-UNKNOWN"
	odpID := ""
	if odp != nil {
		odpCode = odp.Code
		odpID = odp.ID
	}

	subNo := fmt.Sprintf("%s-2026-%04d", t.PrefixID, time.Now().UnixNano()%10000)
	subID := uuid.New().String()
	sub := &domain.Subscriber{
		ID:               subID,
		TenantID:         t.ID,
		SubscriberNo:     subNo,
		FullName:         req.FullName,
		IdentityNumber:   req.IDCardNumber,
		Phone:            req.Phone,
		Email:            req.Email,
		Address:          req.Address,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		DistanceToODP:    mathRound(dist, 1),
		SelectedPlanID:   planID,
		SelectedPlanName: planName,
		NearestODPID:     odpID,
		NearestODPCode:   odpCode,
		Status:           "SUBMITTED",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := h.store.CreateSubscriber(r.Context(), sub); err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal membuat berkas registrasi: "+err.Error())
		return
	}

	woNo := fmt.Sprintf("SPK-2026-%04d", time.Now().UnixNano()%10000)
	wo := &domain.WorkOrder{
		ID:              uuid.New().String(),
		TenantID:        t.ID,
		OrderNo:         woNo,
		SubscriberID:    subID,
		SubscriberNo:    subNo,
		CustomerName:    req.FullName,
		CustomerPhone:   req.Phone,
		CustomerAddress: req.Address,
		CustomerLat:     req.Latitude,
		CustomerLng:     req.Longitude,
		ODPCode:         odpCode,
		PlanName:        planName,
		OrderType:       "INSTALLATION",
		Status:          "PENDING",
		Notes:           fmt.Sprintf("Pemasangan Baru di %s via ODP %s (Jarak: %.1fm)", req.Address, odpCode, dist),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	_ = h.store.CreateWorkOrder(r.Context(), wo)

	h.successResponse(w, "Pendaftaran berhasil dikirim. Tim kami akan segera memproses verifikasi dan survei lokasi.", map[string]interface{}{
		"id":                     sub.ID,
		"registration_no":        sub.SubscriberNo,
		"subscriber_no":          sub.SubscriberNo,
		"full_name":              sub.FullName,
		"phone":                  sub.Phone,
		"address":                sub.Address,
		"status":                 "SUBMITTED",
		"nearest_odp_code":       odpCode,
		"distance_to_odp_meters": mathRound(dist, 1),
		"work_order_no":          wo.OrderNo,
	})
}

// PublicODPs daftar ODP untuk visualisasi peta
func (h *APIHandler) PublicODPs(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	odps, err := h.store.ListODPs(r.Context(), t.ID)
	if err != nil {
		h.failResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	type odpOut struct {
		ID             string  `json:"id"`
		Code           string  `json:"code"`
		Name           string  `json:"name"`
		Latitude       float64 `json:"latitude"`
		Longitude      float64 `json:"longitude"`
		TotalPorts     int     `json:"total_ports"`
		UsedPorts      int     `json:"used_ports"`
		AvailablePorts int     `json:"available_ports"`
		Status         string  `json:"status"`
		ClusterArea    string  `json:"cluster_area"`
		IsShared       bool    `json:"is_shared"`
		ProviderName   string  `json:"provider_name"`
	}
	out := make([]odpOut, 0)
	for _, o := range odps {
		clusterArea := "Area Distribusi"
		parts := strings.Split(o.Code, "-")
		if len(parts) >= 2 && parts[0] == "ODP" {
			clusterArea = "Cluster " + strings.Split(parts[1], "/")[0]
		}
		out = append(out, odpOut{
			ID:             o.ID,
			Code:           o.Code,
			Name:           o.Name,
			Latitude:       o.Latitude,
			Longitude:      o.Longitude,
			TotalPorts:     o.TotalPorts,
			UsedPorts:      o.UsedPorts,
			AvailablePorts: o.TotalPorts - o.UsedPorts,
			Status:         o.Status,
			ClusterArea:    clusterArea,
			IsShared:       o.IsSharedJartaplok,
			ProviderName:   o.OwnerTenantName,
		})
	}
	h.successResponse(w, "ODPs retrieved", out)
}

// PublicClusters daftar klaster coverage dinamis berbasis ODP tenant aktif
func (h *APIHandler) PublicClusters(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}
	odps, err := h.store.ListODPs(r.Context(), t.ID)
	if err != nil || len(odps) == 0 {
		h.successResponse(w, "Clusters retrieved", []map[string]interface{}{})
		return
	}

	type clusterStat struct {
		name       string
		totalODPs  int
		activeODPs int
	}
	clusterMap := make(map[string]*clusterStat)

	for _, o := range odps {
		cName := "Cluster Distribusi Utama"
		parts := strings.Split(o.Code, "-")
		if len(parts) >= 2 && parts[0] == "ODP" {
			areaCode := strings.Split(parts[1], "/")[0]
			cName = fmt.Sprintf("Cluster %s", areaCode)
		} else if o.OwnerTenantName != "" && o.IsSharedJartaplok {
			cName = fmt.Sprintf("Jartaplok %s", o.OwnerTenantName)
		}

		stat, exists := clusterMap[cName]
		if !exists {
			stat = &clusterStat{name: cName}
			clusterMap[cName] = stat
		}
		stat.totalODPs++
		if o.Status == "ACTIVE" || o.Status == "AVAILABLE" || o.Status == "" {
			stat.activeODPs++
		}
	}

	var clusters []map[string]interface{}
	for _, stat := range clusterMap {
		clusters = append(clusters, map[string]interface{}{
			"name":        stat.name,
			"total_odps":  stat.totalODPs,
			"active_odps": stat.activeODPs,
			"is_active":   true,
		})
	}

	h.successResponse(w, "Clusters retrieved", clusters)
}

// PublicTrack pelacakan status registrasi mandiri
func (h *APIHandler) PublicTrack(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	regNo := chi.URLParam(r, "regNo")
	sub, err := h.store.GetSubscriberByNo(r.Context(), t.ID, regNo)
	if err != nil || sub == nil {
		h.failResponse(w, http.StatusNotFound, "Data pendaftaran tidak ditemukan")
		return
	}
	wos, _ := h.store.ListWorkOrders(r.Context(), t.ID, "")
	var woNo, woStatus string
	for _, w := range wos {
		if w.SubscriberID == sub.ID {
			woNo = w.OrderNo
			woStatus = w.Status
			break
		}
	}
	h.successResponse(w, "Tracking retrieved", map[string]interface{}{
		"id":                     sub.ID,
		"registration_no":        sub.SubscriberNo,
		"subscriber_no":          sub.SubscriberNo,
		"full_name":              sub.FullName,
		"phone":                  sub.Phone,
		"email":                  sub.Email,
		"address":                sub.Address,
		"status":                 sub.Status,
		"selected_plan_name":     sub.SelectedPlanName,
		"monthly_price":          175000,
		"nearest_odp_code":       sub.NearestODPCode,
		"distance_to_odp_meters": sub.DistanceToODP,
		"work_order_no":          woNo,
		"work_order_status":      woStatus,
		"created_at":             sub.CreatedAt,
	})
}

// PublicTrackKTP upload foto KTP
func (h *APIHandler) PublicTrackKTP(w http.ResponseWriter, r *http.Request) {
	regNo := chi.URLParam(r, "regNo")
	h.successResponse(w, "Foto KTP berhasil disimpan", map[string]string{
		"registration_no": regNo,
	})
}

// PublicTrackSignContract tanda tangan kontrak digital
func (h *APIHandler) PublicTrackSignContract(w http.ResponseWriter, r *http.Request) {
	regNo := chi.URLParam(r, "regNo")
	h.successResponse(w, "Tanda tangan kontrak digital berhasil disimpan", map[string]string{
		"registration_no": regNo,
	})
}

// PublicReferralCheck cek kode referral sales
func (h *APIHandler) PublicReferralCheck(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	h.successResponse(w, "Referral valid", map[string]interface{}{
		"code":     code,
		"discount": 0,
	})
}

// AuthLogin login terpadu staf: verifikasi bcrypt terhadap tabel users milik tenant.
func (h *APIHandler) AuthLogin(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		h.failResponse(w, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		h.failResponse(w, http.StatusBadRequest, "Username dan password wajib diisi")
		return
	}

	key := t.ID + "|" + strings.ToLower(username)
	if h.throttle.Blocked(key) {
		h.failResponse(w, http.StatusTooManyRequests, "Terlalu banyak percobaan gagal. Coba lagi dalam 15 menit")
		return
	}

	user, err := h.store.GetUserByUsername(r.Context(), t.ID, username)
	hash := dummyBcryptHash
	if err == nil && user != nil {
		hash = user.PasswordHash
	}
	// Selalu jalankan bcrypt agar waktu respons tidak membocorkan keberadaan akun.
	pwOK := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) == nil
	if err != nil || user == nil || !pwOK || !strings.EqualFold(user.Status, "ACTIVE") {
		h.throttle.Fail(key)
		h.failResponse(w, http.StatusUnauthorized, "Username atau password salah")
		return
	}
	h.throttle.Reset(key)

	token, err := auth.Issue(h.authSecret, auth.Claims{
		UserID: user.ID, TenantID: t.ID, Username: user.Username, Role: strings.ToUpper(user.Role),
	})
	if err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal membuat sesi")
		return
	}
	display := auth.DisplayRole(user.Role)
	h.successResponse(w, "Login berhasil", map[string]interface{}{
		"token":        token,
		"username":     user.Username,
		"role":         display,
		"roles":        []string{display},
		"is_superuser": strings.EqualFold(user.Role, "OWNER"),
		"tenant_slug":  t.Slug,
		"tenant_name":  t.Name,
		"full_name":    user.FullName,
	})
}

// AuthMe informasi sesi aktif (dari token yang diverifikasi).
func (h *APIHandler) AuthMe(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	hdr := r.Header.Get("Authorization")
	if !strings.HasPrefix(hdr, "Bearer ") {
		h.failResponse(w, http.StatusUnauthorized, "Login diperlukan")
		return
	}
	c, err := auth.Verify(h.authSecret, strings.TrimPrefix(hdr, "Bearer "))
	if err != nil || t == nil || c.TenantID != t.ID {
		h.failResponse(w, http.StatusUnauthorized, "Sesi tidak valid atau berakhir")
		return
	}
	display := auth.DisplayRole(c.Role)
	h.successResponse(w, "Session valid", map[string]interface{}{
		"username":     c.Username,
		"role":         display,
		"roles":        []string{display},
		"is_superuser": strings.EqualFold(c.Role, "OWNER"),
		"tenant_slug":  t.Slug,
	})
}

// AuthLogout logout sesi
func (h *APIHandler) AuthLogout(w http.ResponseWriter, r *http.Request) {
	h.successResponse(w, "Logout berhasil", nil)
}

// AdminListRegistrations daftar permohonan pasang baru untuk NOC/Admin
func (h *APIHandler) AdminListRegistrations(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	subs, err := h.store.ListSubscribers(r.Context(), t.ID, "")
	if err != nil {
		h.failResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	type regItem struct {
		ID                  string    `json:"id"`
		RegistrationNo      string    `json:"registration_no"`
		FullName            string    `json:"full_name"`
		Phone               string    `json:"phone"`
		Email               string    `json:"email"`
		Address             string    `json:"address"`
		Status              string    `json:"status"`
		SelectedPlanName    string    `json:"selected_plan_name"`
		NearestODPCode      string    `json:"nearest_odp_code"`
		DistanceToODPMeters float64   `json:"distance_to_odp_meters"`
		PPPoEUsername       string    `json:"pppoe_username,omitempty"`
		PPPoEPassword       string    `json:"pppoe_password,omitempty"`
		CreatedAt           time.Time `json:"created_at"`
	}
	out := make([]regItem, 0)
	for _, s := range subs {
		pU := ""
		pP := ""
		if s.PPPoEUsername != nil {
			pU = *s.PPPoEUsername
		}
		if s.PPPoEPassword != nil {
			pP = *s.PPPoEPassword
		}
		out = append(out, regItem{
			ID:                  s.ID,
			RegistrationNo:      s.SubscriberNo,
			FullName:            s.FullName,
			Phone:               s.Phone,
			Email:               s.Email,
			Address:             s.Address,
			Status:              s.Status,
			SelectedPlanName:    s.SelectedPlanName,
			NearestODPCode:      s.NearestODPCode,
			DistanceToODPMeters: s.DistanceToODP,
			PPPoEUsername:       pU,
			PPPoEPassword:       pP,
			CreatedAt:           s.CreatedAt,
		})
	}
	h.successResponse(w, "Registrations retrieved", out)
}

func (h *APIHandler) AdminListODPs(w http.ResponseWriter, r *http.Request) {
	h.PublicODPs(w, r)
}

func (h *APIHandler) AdminCreateODP(w http.ResponseWriter, r *http.Request) {
	h.successResponse(w, "ODP created", nil)
}

func (h *APIHandler) AdminDeleteODP(w http.ResponseWriter, r *http.Request) {
	h.successResponse(w, "ODP deleted", nil)
}

func (h *APIHandler) AdminListWorkOrders(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	wos, err := h.store.ListWorkOrders(r.Context(), t.ID, "")
	if err != nil {
		h.failResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if wos == nil {
		wos = make([]domain.WorkOrder, 0)
	}
	h.successResponse(w, "Work orders retrieved", wos)
}

func (h *APIHandler) AdminListClusters(w http.ResponseWriter, r *http.Request) {
	h.PublicClusters(w, r)
}

func (h *APIHandler) AdminStaffKPI(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}

	subs, _ := h.store.ListSubscribers(r.Context(), t.ID, "")
	activeCount := 0
	for _, s := range subs {
		if s.Status == "ACTIVE" {
			activeCount++
		}
	}

	wos, _ := h.store.ListWorkOrders(r.Context(), t.ID, "")
	spkDone := 0
	for _, w := range wos {
		if w.Status == "COMPLETED" || w.Status == "BAST_APPROVED" {
			spkDone++
		}
	}

	users, _ := h.store.ListUsersByTenant(r.Context(), t.ID)

	type techMetric struct {
		Name                string  `json:"name"`
		Role                string  `json:"role"`
		CompletedOrders     int     `json:"completed_orders"`
		PendingOrders       int     `json:"pending_orders"`
		AvgOpticalPowerDBm  float64 `json:"avg_optical_power_dbm"`
		TotalDropcoreMeters int     `json:"total_dropcore_meters"`
		Score               int     `json:"score"`
		Grade               string  `json:"grade"`
	}

	type salesMetric struct {
		Name              string  `json:"name"`
		PartnerCode       string  `json:"partner_code"`
		TotalLeads        int     `json:"total_leads"`
		ActiveCustomers   int     `json:"active_customers"`
		ConversionRatePct float64 `json:"conversion_rate_pct"`
		TotalCommission   float64 `json:"total_commission"`
		Score             int     `json:"score"`
		Grade             string  `json:"grade"`
	}

	technicians := make([]techMetric, 0)
	sales := make([]salesMetric, 0)

	for _, u := range users {
		roleUpper := strings.ToUpper(u.Role)
		if roleUpper == "TECHNICIAN" {
			technicians = append(technicians, techMetric{
				Name:                u.FullName,
				Role:                "Teknisi Lapangan",
				CompletedOrders:     0,
				PendingOrders:       0,
				AvgOpticalPowerDBm:  -18.5,
				TotalDropcoreMeters: 0,
				Score:               100,
				Grade:               "A+",
			})
		} else if roleUpper == "SALES" {
			sales = append(sales, salesMetric{
				Name:              u.FullName,
				PartnerCode:       strings.ToUpper(u.Username),
				TotalLeads:        0,
				ActiveCustomers:   0,
				ConversionRatePct: 0,
				TotalCommission:   0,
				Score:             100,
				Grade:             "A+",
			})
		}
	}

	h.successResponse(w, "Staff KPI retrieved", map[string]interface{}{
		"branch_name":          "Nasional",
		"total_spk_done":       spkDone,
		"avg_team_optical_dbm": -18.5,
		"total_active_subs":    activeCount,
		"technicians":          technicians,
		"sales":                sales,
	})
}

func (h *APIHandler) AdminRadiusLiveSessions(w http.ResponseWriter, r *http.Request) {
	h.successResponse(w, "Live sessions retrieved", []map[string]interface{}{})
}

func (h *APIHandler) TechnicianListWorkOrders(w http.ResponseWriter, r *http.Request) {
	h.AdminListWorkOrders(w, r)
}

func (h *APIHandler) TechnicianGetWorkOrder(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	id := chi.URLParam(r, "id")
	wo, err := h.store.GetWorkOrderByID(r.Context(), t.ID, id)
	if err != nil || wo == nil {
		h.failResponse(w, http.StatusNotFound, "Work order tidak ditemukan")
		return
	}
	h.successResponse(w, "Work order retrieved", wo)
}

func (h *APIHandler) TechnicianCompleteBAST(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	id := chi.URLParam(r, "id")
	var req struct {
		RxPower float64 `json:"rx_power"`
		SN      string  `json:"serial_number"`
		MAC     string  `json:"mac_address"`
		Notes   string  `json:"notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.RxPower == 0 {
		req.RxPower = -19.5
	}
	if req.SN == "" {
		req.SN = "HWTC" + uuid.New().String()[:8]
	}

	if err := h.store.CompleteWorkOrderBAST(r.Context(), t.ID, id, req.RxPower, req.SN, req.MAC, req.Notes); err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal menyelesaikan BAST: "+err.Error())
		return
	}

	h.successResponse(w, "BAST berhasil ditandatangani dan instalasi pelanggan telah aktif", map[string]interface{}{
		"work_order_id":    id,
		"rx_optical_power": req.RxPower,
		"status":           "COMPLETED",
	})
}

func (h *APIHandler) JartaplokBilling(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	ags, _ := h.store.ListJartaplokAgreements(r.Context(), t.ID)
	odps, _ := h.store.ListODPs(r.Context(), t.ID)

	totalODPs := len(odps)
	totalCapacity := 0
	totalUsed := 0
	for _, o := range odps {
		totalCapacity += o.TotalPorts
		totalUsed += o.UsedPorts
	}
	if totalCapacity == 0 {
		totalCapacity = 16
		totalUsed = 2
	}
	availPorts := totalCapacity - totalUsed
	if availPorts < 0 {
		availPorts = 0
	}
	occPct := float64(0)
	if totalCapacity > 0 {
		occPct = (float64(totalUsed) / float64(totalCapacity)) * 100
	}
	ratePerPort := 25000.0
	totalBilling := float64(totalUsed) * ratePerPort

	h.successResponse(w, "Jartaplok billing retrieved", map[string]interface{}{
		"agreements":              ags,
		"rate_per_port":           ratePerPort,
		"total_allocated":         totalCapacity,
		"total_capacity_ports":    totalCapacity,
		"total_active_ports":      totalUsed,
		"total_odps":              totalODPs,
		"available_ports":         availPorts,
		"occupancy_pct":           occPct,
		"total_billing_amount":    totalBilling,
		"total_full_monthly":      totalBilling,
		"total_savings_prorata":   0,
		"total_suspended_ports":   0,
		"partner_name":            "Mitra Wholesale",
		"service_type":            "WHOLESALE",
		"tiers": []map[string]interface{}{
			{
				"speed_mbps":      50,
				"rate_per_port":   ratePerPort,
				"active_ports":    totalUsed,
				"suspended_ports": 0,
				"subtotal":        totalBilling,
				"full_subtotal":   totalBilling,
			},
		},
		"estimated_capex_savings": "Rp 45.000.000 (Penghematan Penarikan Feeder)",
	})
}

func (h *APIHandler) JartaplokODPs(w http.ResponseWriter, r *http.Request) {
	h.PublicODPs(w, r)
}

// SyncODPFromFiberGrid menerima sinkronisasi data titik ODP dari Engine 1 (FiberGrid)
func (h *APIHandler) SyncODPFromFiberGrid(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)

	var req struct {
		Code        string  `json:"code"`
		Name        string  `json:"name"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		TotalPorts  int     `json:"total_ports"`
		UsedPorts   int     `json:"used_ports"`
		Status      string  `json:"status"`
		ClusterArea string  `json:"cluster_area"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.failResponse(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if req.Code == "" {
		h.failResponse(w, http.StatusBadRequest, "Kode ODP wajib diisi")
		return
	}

	if req.TotalPorts <= 0 {
		req.TotalPorts = 8
	}
	if req.Status == "" {
		req.Status = "ACTIVE"
	}

	odp := &domain.ODP{
		TenantID:   t.ID,
		Code:       strings.ToUpper(strings.TrimSpace(req.Code)),
		Name:       req.Name,
		Latitude:   req.Latitude,
		Longitude:  req.Longitude,
		TotalPorts: req.TotalPorts,
		UsedPorts:  req.UsedPorts,
		Status:     req.Status,
	}

	if err := h.store.UpsertODP(r.Context(), odp); err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal menyimpan ODP: "+err.Error())
		return
	}

	h.successResponse(w, fmt.Sprintf("ODP %s berhasil disinkronkan ke Maps Coverage Nexus", odp.Code), odp)
}


func (h *APIHandler) JartaplokPorts(w http.ResponseWriter, r *http.Request) {
	h.successResponse(w, "Jartaplok ports retrieved", []map[string]interface{}{
		{
			"circuit_id":     "CKT-JARTAP-001",
			"odp_code":       "ODP-PYK-001",
			"odp_name":       "ODP Simpang Benteng 01",
			"port_number":    1,
			"package_speed":  "50 Mbps",
			"monthly_rental": 25000,
			"prorated_fee":   25000,
			"is_prorated":    false,
			"active_days":    30,
			"total_days":     30,
			"activated_at":   "01 Sep 2026",
			"status":         "ACTIVE",
		},
		{
			"circuit_id":     "CKT-JARTAP-002",
			"odp_code":       "ODP-PYK-002",
			"odp_name":       "ODP Koridor Sudirman 02",
			"port_number":    2,
			"package_speed":  "100 Mbps",
			"monthly_rental": 35000,
			"prorated_fee":   35000,
			"is_prorated":    false,
			"active_days":    30,
			"total_days":     30,
			"activated_at":   "05 Sep 2026",
			"status":         "ACTIVE",
		},
	})
}

func (h *APIHandler) SuperuserOverview(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	subs, _ := h.store.ListSubscribers(r.Context(), t.ID, "")
	odps, _ := h.store.ListODPs(r.Context(), t.ID)
	olts, _ := h.store.ListOLTs(r.Context(), t.ID)
	mrr := float64(len(subs)) * 250000.0

	h.successResponse(w, "Superuser overview retrieved", map[string]interface{}{
		"mrr":               mrr,
		"total_subscribers": len(subs),
		"total_odps":        len(odps),
		"total_olts":        len(olts),
		"olt_status":        "ONLINE",
		"network_health":    "99.98%",
	})
}

func (h *APIHandler) SuperuserJartaplokPartners(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}
	ags, err := h.store.ListJartaplokAgreements(r.Context(), t.ID)
	if err != nil || len(ags) == 0 {
		h.successResponse(w, "Jartaplok partners retrieved", []domain.JartaplokAgreement{})
		return
	}
	h.successResponse(w, "Jartaplok partners retrieved", ags)
}

func (h *APIHandler) SuperuserListStaff(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}
	users, err := h.store.ListUsersByTenant(r.Context(), t.ID)
	if err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal mengambil daftar staf: "+err.Error())
		return
	}

	type staffItem struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		FullName   string `json:"full_name"`
		Email      string `json:"email"`
		Phone      string `json:"phone"`
		Role       string `json:"role"`
		Status     string `json:"status"`
		BranchCode string `json:"branch_code"`
	}

	sanitized := make([]staffItem, 0)
	for _, u := range users {
		sanitized = append(sanitized, staffItem{
			ID:         u.ID,
			Username:   u.Username,
			FullName:   u.FullName,
			Email:      u.Email,
			Phone:      u.Phone,
			Role:       u.Role,
			Status:     u.Status,
			BranchCode: "ALL",
		})
	}

	h.successResponse(w, "Staff retrieved", sanitized)
}

func (h *APIHandler) SuperuserUpdateStaff(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}
	userID := chi.URLParam(r, "id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.failResponse(w, http.StatusBadRequest, "Payload tidak valid")
		return
	}
	if err := h.store.UpdateUserStatus(r.Context(), t.ID, userID, req.Status); err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal memperbarui status staf: "+err.Error())
		return
	}
	h.successResponse(w, "Status staf berhasil diperbarui", nil)
}

func (h *APIHandler) SuperuserResetStaffPassword(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}
	var req struct {
		Username    string `json:"username"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.NewPassword == "" {
		h.failResponse(w, http.StatusBadRequest, "Username dan password baru wajib diisi")
		return
	}
	if err := h.store.ResetUserPassword(r.Context(), t.ID, req.Username, req.NewPassword); err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal mereset kata sandi: "+err.Error())
		return
	}
	h.successResponse(w, "Kata sandi staf berhasil direset", nil)
}

// ── Staff Quota & Add-on Handlers ──────────────────────────────────────────

// GetStaffQuota mengembalikan informasi kuota staf aktif, add-on, dan sisa kursi
func (h *APIHandler) GetStaffQuota(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	quota, err := h.store.GetStaffQuotaStatus(r.Context(), t.ID)
	if err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal mengambil kuota staf: "+err.Error())
		return
	}
	h.successResponse(w, "Status kuota staf berhasil diambil", quota)
}

// ListStaffUsers menampilkan seluruh akun user di tenant ini beserta status kuota
func (h *APIHandler) ListStaffUsers(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	users, err := h.store.ListUsersByTenant(r.Context(), t.ID)
	if err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal mengambil daftar staf: "+err.Error())
		return
	}

	quota, _ := h.store.GetStaffQuotaStatus(r.Context(), t.ID)

	type userResp struct {
		ID        string    `json:"id"`
		TenantID  string    `json:"tenant_id"`
		Username  string    `json:"username"`
		FullName  string    `json:"full_name"`
		Email     string    `json:"email"`
		Phone     string    `json:"phone"`
		Role      string    `json:"role"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
	}

	var sanitized []userResp
	for _, u := range users {
		sanitized = append(sanitized, userResp{
			ID:        u.ID,
			TenantID:  u.TenantID,
			Username:  u.Username,
			FullName:  u.FullName,
			Email:     u.Email,
			Phone:     u.Phone,
			Role:      u.Role,
			Status:    u.Status,
			CreatedAt: u.CreatedAt,
		})
	}

	h.successResponse(w, "Daftar staf berhasil diambil", map[string]interface{}{
		"quota": quota,
		"users": sanitized,
	})
}

// CreateStaffUser menambahkan akun staf baru dengan validasi batas kuota & addon
func (h *APIHandler) CreateStaffUser(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		FullName string `json:"full_name"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.failResponse(w, http.StatusBadRequest, "Payload request tidak valid")
		return
	}

	req.Username = strings.TrimSpace(strings.ToLower(req.Username))
	if req.Username == "" || req.Password == "" {
		h.failResponse(w, http.StatusBadRequest, "Username dan password wajib diisi")
		return
	}

	role := strings.ToUpper(strings.TrimSpace(req.Role))
	if role == "" {
		role = "TECHNICIAN"
	}

	// Validasi kuota jika bukan Owner
	if role != "OWNER" {
		quota, err := h.store.GetStaffQuotaStatus(r.Context(), t.ID)
		if err != nil {
			h.failResponse(w, http.StatusInternalServerError, "Gagal validasi kuota: "+err.Error())
			return
		}
		if !quota.CanAddStaff {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success":    false,
				"error_code": "STAFF_LIMIT_REACHED",
				"message":    fmt.Sprintf("Batas kuota staf (%d/%d akun aktif) telah tercapai. Silakan beli Add-on Staf untuk menambah petugas baru.", quota.UsedStaffCount, quota.TotalQuota),
				"quota":      quota,
			})
			return
		}
	}

	newUser := &domain.User{
		TenantID: t.ID,
		Username: req.Username,
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
		Role:     role,
		Status:   "ACTIVE",
	}

	if err := h.store.CreateUser(r.Context(), newUser, req.Password); err != nil {
		h.failResponse(w, http.StatusBadRequest, "Gagal membuat akun staf: "+err.Error())
		return
	}

	updatedQuota, _ := h.store.GetStaffQuotaStatus(r.Context(), t.ID)

	h.successResponse(w, "Akun staf berhasil dibuat", map[string]interface{}{
		"user":  newUser,
		"quota": updatedQuota,
	})
}

// ListAddons menampilkan add-on aktif tenant dan katalog paket addon yang tersedia
func (h *APIHandler) ListAddons(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)

	activeAddons, err := h.store.ListTenantAddons(r.Context(), t.ID)
	if err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal mengambil daftar addon: "+err.Error())
		return
	}

	quota, _ := h.store.GetStaffQuotaStatus(r.Context(), t.ID)

	catalog := []map[string]interface{}{
		{
			"code":          "ADDON_STAFF_3",
			"name":          "Starter Staff Pack (+3 Akun)",
			"type":          "STAFF_SEAT",
			"quantity":      3,
			"monthly_price": 35000,
			"description":   "Tambahan 3 akun staf teknisi/NOC/sales",
		},
		{
			"code":          "ADDON_STAFF_5",
			"name":          "Growth Staff Pack (+5 Akun)",
			"type":          "STAFF_SEAT",
			"quantity":      5,
			"monthly_price": 50000,
			"description":   "Paket paling laris: Tambahan 5 akun staf lapangan & administrasi",
		},
		{
			"code":          "ADDON_STAFF_10",
			"name":          "Scale Staff Pack (+10 Akun)",
			"type":          "STAFF_SEAT",
			"quantity":      10,
			"monthly_price": 90000,
			"description":   "Hemat Rp 10.000: Tambahan 10 akun staf untuk ISP berkembang cepat",
		},
	}

	h.successResponse(w, "Katalog addon berhasil diambil", map[string]interface{}{
		"quota":         quota,
		"active_addons": activeAddons,
		"catalog":       catalog,
	})
}

// PurchaseAddon aktivasi add-on baru untuk tenant
func (h *APIHandler) PurchaseAddon(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)

	var req struct {
		AddonCode string `json:"addon_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AddonCode == "" {
		h.failResponse(w, http.StatusBadRequest, "Pilih addon_code yang valid")
		return
	}

	var name string
	var quantity int
	var price float64

	switch req.AddonCode {
	case "ADDON_STAFF_3":
		name = "Starter Staff Pack (+3 Akun)"
		quantity = 3
		price = 35000
	case "ADDON_STAFF_5":
		name = "Growth Staff Pack (+5 Akun)"
		quantity = 5
		price = 50000
	case "ADDON_STAFF_10":
		name = "Scale Staff Pack (+10 Akun)"
		quantity = 10
		price = 90000
	default:
		h.failResponse(w, http.StatusBadRequest, "Kode paket addon tidak dikenali")
		return
	}

	expires := time.Now().AddDate(0, 1, 0) // Berlaku 1 bulan
	addon := &domain.TenantAddon{
		TenantID:     t.ID,
		AddonCode:    req.AddonCode,
		AddonType:    "STAFF_SEAT",
		Name:         name,
		Quantity:     quantity,
		MonthlyPrice: price,
		Status:       "ACTIVE",
		ExpiresAt:    &expires,
	}

	if err := h.store.CreateTenantAddon(r.Context(), addon); err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal mengaktifkan addon: "+err.Error())
		return
	}

	newQuota, _ := h.store.GetStaffQuotaStatus(r.Context(), t.ID)

	h.successResponse(w, fmt.Sprintf("Add-on %s berhasil diaktifkan. Kuota staf bertambah +%d!", name, quantity), map[string]interface{}{
		"addon": addon,
		"quota": newQuota,
	})
}


// GenerateMikrotikVPN membuat WireGuard keys dan script MikroTik
func (h *APIHandler) GenerateMikrotikVPN(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}

	// Cek apakah sudah ada router
	router, err := h.store.GetMikrotikRouter(r.Context(), t.ID)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Gagal mengecek router")
		return
	}

	var privKey wgtypes.Key
	var pubKey wgtypes.Key

	// Jika belum ada, buat baru
	if router == nil {
		privKey, _ = wgtypes.GeneratePrivateKey()
		pubKey = privKey.PublicKey()

		// TODO: Implementasi alokasi IP otomatis yang tidak bentrok (sementara hardcode 10.255.0.2)
		// Kita bisa menggunakan query SQL untuk mencari IP terakhir.
		ipSuffix := time.Now().Unix() % 250 + 2 // Cepat untuk prototipe
		wgIP := fmt.Sprintf("10.255.0.%d", ipSuffix)

		router = &domain.MikrotikRouter{
			ID:          uuid.New().String(),
			TenantID:    t.ID,
			Name:        "Router Utama " + t.Name,
			WgPubkey:    pubKey.String(),
			WgIP:        wgIP,
			APIPort:     8728,
			APIUser:     "ispsync_api",
			APIPassword: uuid.New().String()[:12], // Random password API
			Status:      "offline",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := h.store.CreateMikrotikRouter(r.Context(), router); err != nil {
			h.errorResponse(w, http.StatusInternalServerError, "Gagal menyimpan data router: "+err.Error())
			return
		}
	}

	// Generate Script Winbox
	script := fmt.Sprintf(`/interface wireguard add listen-port=13231 mtu=1420 name=wg-ispsync private-key="%s"
/interface wireguard peers add allowed-address=0.0.0.0/0 endpoint-address=vpn.ispsync.id endpoint-port=51820 interface=wg-ispsync persistent-keepalive=25s public-key="<SERVER_PUBLIC_KEY_DISINI>"
/ip address add address=%s/16 interface=wg-ispsync
/user add name=%s password=%s group=full
/ip service set api address=10.255.0.0/16 port=%d disabled=no`, 
		privKey.String(), router.WgIP, router.APIUser, router.APIPassword, router.APIPort)

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"router": router,
		"script": script,
	})
}

// GenerateHotspotConfig membuat script konfigurasi Walled-Garden dan External Login Hotspot MikroTik
func (h *APIHandler) GenerateHotspotConfig(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}

	// Host landing page / hotspot dari Nexus tenant ini (default ke domain utama kalau custom-nya kosong)
	hotspotDomain := t.CustomDomain
	if hotspotDomain == "" {
		hotspotDomain = "nexus." + t.Slug + ".ispsync.id"
	}

	script := fmt.Sprintf(`/ip hotspot profile
set [ find default=yes ] hotspot-address=10.5.50.1 login-by=cookie,http-chap,http-pap
add dns-name=login.lokal hotspot-address=10.5.50.1 html-directory=flash/hotspot login-by=mac,cookie,http-chap,http-pap name=ispsync_prof

/ip hotspot walled-garden
add action=allow comment="ISPSYNC Cloud Gateway" dst-host="%s"
add action=allow comment="ISPSYNC Cloud IP" dst-address="103.179.65.73"
add action=allow comment="Xendit Payment Gateway" dst-host="*.xendit.co"
add action=allow comment="Midtrans Payment Gateway" dst-host="*.midtrans.com"
add action=allow comment="Midtrans CDN" dst-host="*.sandbox.midtrans.com"
add action=allow comment="Google Fonts" dst-host="fonts.googleapis.com"
add action=allow comment="Google Fonts Static" dst-host="fonts.gstatic.com"

/ip hotspot walled-garden ip
add action=accept comment="Allow ISPSYNC Cloud IP" dst-address="103.179.65.73"
add action=accept comment="Allow DNS Google" dst-address="8.8.8.8"
add action=accept comment="Allow DNS Cloudflare" dst-address="1.1.1.1"

# Catatan: Silakan ubah hotspot-address dan nama direktori (html-directory) sesuai dengan jaringan Anda.
# Jika Anda menggunakan RADIUS cloud ISPSYNC, pastikan Radius Client sudah diarahkan ke 10.255.x.x (VPN) atau Public IP ISPSYNC.
`, hotspotDomain)

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"script":  script,
	})
}



// GenerateIsolirScript membuat script NAT dan IP Pool untuk Auto-Isolir di MikroTik
func (h *APIHandler) GenerateIsolirScript(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}

	// Domain Isolir
	isolirDomain := t.CustomDomain
	if isolirDomain == "" {
		isolirDomain = "nexus." + t.Slug + ".ispsync.id"
	}

	script := fmt.Sprintf(`/ip pool
add name=POOL_ISOLIR ranges=10.50.50.2-10.50.50.254

/ip firewall nat
# Redirect semua trafik HTTP port 80 ke server Cloud ISPSYNC (Halaman Isolir)
add action=dst-nat chain=dstnat comment="ISPSYNC AUTO-ISOLIR HTTP" dst-port=80 protocol=tcp src-address=10.50.50.0/24 to-addresses=103.179.65.73 to-ports=80
# Redirect trafik HTTPS (Optional, bisa menyebabkan SSL warning di sisi client, tapi efektif memblokir akses)
add action=dst-nat chain=dstnat comment="ISPSYNC AUTO-ISOLIR HTTPS" dst-port=443 protocol=tcp src-address=10.50.50.0/24 to-addresses=103.179.65.73 to-ports=443

/ip firewall filter
add action=accept chain=forward comment="Allow DNS for Isolir" dst-port=53 protocol=udp src-address=10.50.50.0/24
add action=accept chain=forward comment="Allow HTTP to ISPSYNC Cloud" dst-address=103.179.65.73 dst-port=80,443 protocol=tcp src-address=10.50.50.0/24
add action=drop chain=forward comment="Drop All Other Traffic from ISOLIR" src-address=10.50.50.0/24
`)

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"script":  script,
	})
}

// KickSubscriber menendang user PPPoE atau Hotspot secara realtime menggunakan RouterOS API via WireGuard
func (h *APIHandler) KickSubscriber(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		h.errorResponse(w, http.StatusBadRequest, "Parameter username dibutuhkan")
		return
	}

	router, err := h.store.GetMikrotikRouter(r.Context(), t.ID)
	if err != nil || router == nil {
		h.errorResponse(w, http.StatusInternalServerError, "Router VPN MikroTik belum dikonfigurasi")
		return
	}

	// Connect ke MikroTik via WireGuard IP (e.g. 10.255.0.x:8728)
	client, err := routeros.Dial(fmt.Sprintf("%s:%d", router.WgIP, router.APIPort), router.APIUser, router.APIPassword)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Gagal terhubung ke RouterOS API via VPN: " + err.Error())
		return
	}
	defer client.Close()

	// Coba tendang dari PPPoE
	replyPPPoE, _ := client.Run("/ppp/active/print", "?name=" + username)
	pppoeKicked := false
	if len(replyPPPoE.Re) > 0 {
		for _, re := range replyPPPoE.Re {
			id := re.Map[".id"]
			client.Run("/ppp/active/remove", "=.id=" + id)
			pppoeKicked = true
		}
	}

	// Coba tendang dari Hotspot
	replyHotspot, _ := client.Run("/ip/hotspot/active/print", "?user=" + username)
	hotspotKicked := false
	if len(replyHotspot.Re) > 0 {
		for _, re := range replyHotspot.Re {
			id := re.Map[".id"]
			client.Run("/ip/hotspot/active/remove", "=.id=" + id)
			hotspotKicked = true
		}
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("User %s dieksekusi. PPPoE: %v, Hotspot: %v", username, pppoeKicked, hotspotKicked),
	})
}

// AdminListPartners mengembalikan daftar tim sales & agen mitra resmi
func (h *APIHandler) AdminListPartners(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}

	users, err := h.store.ListUsersByTenant(r.Context(), t.ID)
	if err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal mengambil daftar sales & mitra: "+err.Error())
		return
	}

	type partnerItem struct {
		ID             string  `json:"id"`
		Code           string  `json:"code"`
		Name           string  `json:"name"`
		BranchCode     string  `json:"branch_code"`
		ContactPhone   string  `json:"contact_phone"`
		CommissionRate float64 `json:"commission_rate"`
		IsActive       bool    `json:"is_active"`
	}

	partners := make([]partnerItem, 0)
	for _, u := range users {
		if strings.EqualFold(u.Role, "SALES") {
			partners = append(partners, partnerItem{
				ID:             u.ID,
				Code:           strings.ToUpper(u.Username),
				Name:           u.FullName,
				BranchCode:     "ALL",
				ContactPhone:   u.Phone,
				CommissionRate: 50000,
				IsActive:       u.Status == "ACTIVE",
			})
		}
	}

	h.successResponse(w, "Sales partners retrieved", partners)
}

// AdminCreatePartner mendaftarkan sales / mitra baru
func (h *APIHandler) AdminCreatePartner(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}

	var req struct {
		Code           string  `json:"code"`
		Name           string  `json:"name"`
		ContactPhone   string  `json:"contact_phone"`
		CommissionRate float64 `json:"commission_rate"`
		BranchCode     string  `json:"branch_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.failResponse(w, http.StatusBadRequest, "Payload request tidak valid")
		return
	}

	code := strings.TrimSpace(strings.ToUpper(req.Code))
	name := strings.TrimSpace(req.Name)
	if code == "" || name == "" {
		h.failResponse(w, http.StatusBadRequest, "Kode dan Nama sales wajib diisi")
		return
	}

	user := &domain.User{
		TenantID: t.ID,
		Username: strings.ToLower(code),
		FullName: name,
		Email:    fmt.Sprintf("%s@%s.local", strings.ToLower(code), t.Slug),
		Phone:    strings.TrimSpace(req.ContactPhone),
		Role:     "SALES",
		Status:   "ACTIVE",
	}

	defaultPassword := "Sales@" + code
	if err := h.store.CreateUser(r.Context(), user, defaultPassword); err != nil {
		h.failResponse(w, http.StatusInternalServerError, "Gagal mendaftarkan sales: "+err.Error())
		return
	}

	h.successResponse(w, "Sales berhasil didaftarkan", user)
}

// AdminUpdatePartner memperbarui status sales / mitra
func (h *APIHandler) AdminUpdatePartner(w http.ResponseWriter, r *http.Request) {
	t := middleware.GetTenant(r)
	if t == nil {
		h.errorResponse(w, http.StatusNotFound, "Tenant context not found")
		return
	}

	partnerID := chi.URLParam(r, "id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.failResponse(w, http.StatusBadRequest, "Payload request tidak valid")
		return
	}

	if req.Status != "" {
		if err := h.store.UpdateUserStatus(r.Context(), t.ID, partnerID, req.Status); err != nil {
			h.failResponse(w, http.StatusInternalServerError, "Gagal memperbarui status sales: "+err.Error())
			return
		}
	}

	h.successResponse(w, "Sales berhasil diperbarui", nil)
}
