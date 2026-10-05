package notification

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type NotificationService struct {
	db         *sql.DB
	httpClient *http.Client
}

type waConfig struct {
	WAProvider  string `json:"wa_provider"`
	WAApiToken  string `json:"wa_api_token"`
	WAServerURL string `json:"wa_server_url"`
}

type RegistrationNotifData struct {
	TenantName   string
	TenantSlug   string
	CustomerName string
	Phone        string
	Address      string
	RegNo        string
	PlanName     string
	ODPCode      string
	Distance     float64
}

func NewNotificationService(billingDBURL string) *NotificationService {
	client := &http.Client{Timeout: 12 * time.Second}
	if billingDBURL == "" {
		return &NotificationService{httpClient: client}
	}

	db, err := sql.Open("postgres", billingDBURL)
	if err != nil {
		log.Printf("[NOTIFICATION SERVICE] Warning: Gagal membuka koneksi database billing: %v", err)
		return &NotificationService{httpClient: client}
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(10 * time.Minute)

	return &NotificationService{
		db:         db,
		httpClient: client,
	}
}

// NormalizePhone membersihkan nomor telepon ke format internasional Indonesia (62xxx)
func NormalizePhone(phone string) string {
	re := regexp.MustCompile(`[^0-9]`)
	cleaned := re.ReplaceAllString(phone, "")
	if strings.HasPrefix(cleaned, "0") {
		cleaned = "62" + cleaned[1:]
	} else if strings.HasPrefix(cleaned, "8") {
		cleaned = "62" + cleaned
	}
	return cleaned
}

// SendRegistrationNotification membaca template dari Ledger dan mengirimkan pesan WA ke pemohon
func (s *NotificationService) SendRegistrationNotification(ctx context.Context, data RegistrationNotifData) {
	targetPhone := NormalizePhone(data.Phone)
	if targetPhone == "" {
		log.Printf("[NOTIFICATION SERVICE] Nomor telepon %q tidak valid, abaikan kirim WA", data.Phone)
		return
	}

	if s.db == nil {
		log.Printf("[NOTIFICATION SERVICE] Database billing belum terhubung, pengiriman WA otomatis dilewati")
		return
	}

	// 1. Ambil pengaturan WhatsApp Gateway dari app_settings
	var settingsJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT value FROM app_settings WHERE key = 'notification_settings'").Scan(&settingsJSON)
	if err != nil {
		log.Printf("[NOTIFICATION SERVICE] Gagal membaca notification_settings dari database: %v", err)
		return
	}

	var cfg waConfig
	if err := json.Unmarshal(settingsJSON, &cfg); err != nil {
		log.Printf("[NOTIFICATION SERVICE] Gagal memparsing konfigurasi WA: %v", err)
		return
	}

	if strings.TrimSpace(cfg.WAApiToken) == "" {
		log.Printf("[NOTIFICATION SERVICE] Token WhatsApp Gateway belum dikonfigurasi di Ledger, pesan tidak dikirim")
		return
	}

	// 2. Ambil template REGISTRATION_SUBMITTED_WA dari notification_templates
	var templateBody string
	var isActive bool
	var subject string
	err = s.db.QueryRowContext(ctx,
		"SELECT body, is_active, COALESCE(subject, 'Pendaftaran Pasang Baru Berhasil') FROM notification_templates WHERE code = 'REGISTRATION_SUBMITTED_WA' AND channel = 'WHATSAPP'",
	).Scan(&templateBody, &isActive, &subject)

	if err != nil {
		// Fallback template jika belum ada baris di database
		templateBody = "Halo Bapak/Ibu {{customer_name}},\n\nTerima kasih telah mendaftar layanan internet fiber {{company_name}}!\nPermohonan pasang baru Anda telah berhasil kami terima.\n\n📋 Detail Pendaftaran:\n- No. Registrasi: {{registration_no}}\n- Paket Pilihan: {{plan_name}}\n- Titik Distribusi: {{nearest_odp}} (Jarak: {{distance}} meter)\n\n🔍 Pantau progres verifikasi & instalasi teknisi secara mandiri di:\n👉 {{tracking_url}}\n\nTim teknisi kami akan segera menghubungi nomor WhatsApp ini untuk konfirmasi jadwal survei lapangan. Terima kasih!"
		isActive = true
		subject = "Pendaftaran Pasang Baru Berhasil"
	}

	if !isActive {
		log.Printf("[NOTIFICATION SERVICE] Template REGISTRATION_SUBMITTED_WA dinonaktifkan di Ledger oleh owner")
		return
	}

	// 3. Interpolasi variabel dinamis
	trackingURL := fmt.Sprintf("https://portal.%s.ispsync.id/?track=%s", data.TenantSlug, data.RegNo)
	if data.TenantSlug == "" || data.TenantSlug == "dev" {
		trackingURL = fmt.Sprintf("https://portal.dev.ispsync.id/?track=%s", data.RegNo)
	}

	replacer := strings.NewReplacer(
		"{{customer_name}}", data.CustomerName,
		"{{registration_no}}", data.RegNo,
		"{{plan_name}}", data.PlanName,
		"{{nearest_odp}}", data.ODPCode,
		"{{distance}}", fmt.Sprintf("%.1f", data.Distance),
		"{{company_name}}", data.TenantName,
		"{{tracking_url}}", trackingURL,
		"{{customer_phone}}", data.Phone,
		"{{address}}", data.Address,
	)
	finalMessage := replacer.Replace(templateBody)

	// 4. Dispatch ke Provider (WABLAS atau FONNTE)
	provider := strings.ToUpper(strings.TrimSpace(cfg.WAProvider))
	var sendErr error

	if provider == "WABLAS" {
		sendErr = s.sendWablas(ctx, cfg.WAServerURL, cfg.WAApiToken, targetPhone, finalMessage)
	} else if provider == "FONNTE" {
		sendErr = s.sendFonnte(ctx, cfg.WAApiToken, targetPhone, finalMessage)
	} else {
		// Default ke Wablas
		sendErr = s.sendWablas(ctx, cfg.WAServerURL, cfg.WAApiToken, targetPhone, finalMessage)
	}

	// 5. Catat ke tabel notifications di Ledger untuk Riwayat Notifikasi
	status := "SENT"
	var errStr *string
	var sentAt *time.Time
	now := time.Now()
	if sendErr != nil {
		status = "FAILED"
		msg := sendErr.Error()
		errStr = &msg
		log.Printf("[NOTIFICATION SERVICE] Gagal mengirim pesan WA ke %s: %v", targetPhone, sendErr)
	} else {
		sentAt = &now
		log.Printf("[NOTIFICATION SERVICE] ✅ Berhasil mengirim WA konfirmasi pasang baru (%s) ke %s via %s", data.RegNo, targetPhone, provider)
	}

	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO notifications (id, channel, recipient, subject, body, status, error_message, sent_at, created_at)
		 VALUES (gen_random_uuid(), 'WHATSAPP', $1, $2, $3, $4, $5, $6, NOW())`,
		targetPhone, subject, finalMessage, status, errStr, sentAt,
	)
}

func (s *NotificationService) sendWablas(ctx context.Context, serverURL, token, phone, message string) error {
	baseURL := strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if baseURL == "" {
		baseURL = "https://api.wablas.com"
	}
	sendEndpoint := baseURL + "/api/send-message"

	payload := map[string]string{
		"phone":   phone,
		"message": message,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("koneksi wablas gagal: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("wablas HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (s *NotificationService) sendFonnte(ctx context.Context, token, phone, message string) error {
	sendEndpoint := "https://api.fonnte.com/send"

	payload := map[string]string{
		"target":  phone,
		"message": message,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("koneksi fonnte gagal: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("fonnte HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// SendUncoveredStatusNotification mengirimkan notifikasi WA ke pelanggan ketika status permohonan ditandai Wishlist atau Dibatalkan
func (s *NotificationService) SendUncoveredStatusNotification(ctx context.Context, tenantName, customerName, phone, regNo, action, reason string) {
	targetPhone := NormalizePhone(phone)
	if targetPhone == "" {
		log.Printf("[NOTIFICATION SERVICE] Nomor telepon %q tidak valid, abaikan kirim WA uncovered", phone)
		return
	}

	if s.db == nil {
		log.Printf("[NOTIFICATION SERVICE] Database billing belum terhubung, pengiriman WA uncovered dilewati")
		return
	}

	// 1. Ambil pengaturan WhatsApp Gateway dari app_settings
	var settingsJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT value FROM app_settings WHERE key = 'notification_settings'").Scan(&settingsJSON)
	if err != nil {
		log.Printf("[NOTIFICATION SERVICE] Gagal membaca notification_settings dari database: %v", err)
		return
	}

	var cfg waConfig
	if err := json.Unmarshal(settingsJSON, &cfg); err != nil {
		log.Printf("[NOTIFICATION SERVICE] Gagal memparsing konfigurasi WA: %v", err)
		return
	}

	if strings.TrimSpace(cfg.WAApiToken) == "" {
		log.Printf("[NOTIFICATION SERVICE] Token WhatsApp Gateway belum dikonfigurasi di Ledger, pesan tidak dikirim")
		return
	}

	// Tentukan template code berdasarkan action (WISHLIST atau CANCEL)
	isCancel := strings.Contains(strings.ToUpper(action), "CANCEL")
	templateCode := "UNCOVERED_WISHLIST_WA"
	defaultSubject := "Prioritas Perluasan Jaringan Fiber"
	defaultBody := `Halo Bapak/Ibu {{customer_name}},

Terima kasih atas minat Anda berlangganan internet fiber {{company_name}} (No. Reg: {{registration_no}}).

Berdasarkan hasil survei tim teknis kami, saat ini lokasi rumah Anda belum terjangkau jalur distribusi kabel Fiber Optik kami dalam batas jarak aman.

Data permohonan Anda telah kami simpan ke dalam *Daftar Prioritas Perluasan Jaringan (Wishlist)* {{company_name}}. Kami akan segera menghubungi Anda kembali begitu tiang/jalur distribusi baru resmi dibuka di wilayah Anda.

Salam hormat,
Tim Layanan Pelanggan {{company_name}}`

	if isCancel {
		templateCode = "UNCOVERED_CANCELLED_WA"
		defaultSubject = "Pemberitahuan Status Permohonan Pasang Baru"
		defaultBody = `Halo Bapak/Ibu {{customer_name}},

Terima kasih atas minat Anda berlangganan internet fiber {{company_name}} (No. Reg: {{registration_no}}).

Setelah dilakukan pengecekan teknis mendalam, mohon maaf permohonan pasang baru saat ini belum dapat kami proses karena lokasi berada di luar batas jangkauan infrastruktur fiber optik kami.

Terima kasih banyak atas pengertian Anda.

Salam hormat,
Tim Layanan Pelanggan {{company_name}}`
	}

	// 2. Ambil template dari notification_templates
	var templateBody string
	var isActive bool
	var subject string
	err = s.db.QueryRowContext(ctx,
		"SELECT body, is_active, COALESCE(subject, $2) FROM notification_templates WHERE code = $1 AND channel = 'WHATSAPP'",
		templateCode, defaultSubject,
	).Scan(&templateBody, &isActive, &subject)

	if err != nil {
		log.Printf("[NOTIFICATION SERVICE] Template %s tidak ditemukan di DB, menggunakan template bawaan sistem", templateCode)
		templateBody = defaultBody
		isActive = true
		subject = defaultSubject
	}

	if !isActive {
		log.Printf("[NOTIFICATION SERVICE] Template %s dinonaktifkan di Ledger oleh owner", templateCode)
		return
	}

	// 3. Interpolasi variabel dinamis
	replacer := strings.NewReplacer(
		"{{customer_name}}", customerName,
		"{{registration_no}}", regNo,
		"{{company_name}}", tenantName,
		"{{customer_phone}}", phone,
		"{{reason}}", reason,
	)
	finalMessage := replacer.Replace(templateBody)

	// 4. Dispatch ke Provider (WABLAS atau FONNTE)
	provider := strings.ToUpper(strings.TrimSpace(cfg.WAProvider))
	var sendErr error

	if provider == "WABLAS" {
		sendErr = s.sendWablas(ctx, cfg.WAServerURL, cfg.WAApiToken, targetPhone, finalMessage)
	} else if provider == "FONNTE" {
		sendErr = s.sendFonnte(ctx, cfg.WAApiToken, targetPhone, finalMessage)
	} else {
		sendErr = s.sendWablas(ctx, cfg.WAServerURL, cfg.WAApiToken, targetPhone, finalMessage)
	}

	// 5. Catat ke tabel notifications di Ledger
	status := "SENT"
	var errStr *string
	var sentAt *time.Time
	now := time.Now()
	if sendErr != nil {
		status = "FAILED"
		msg := sendErr.Error()
		errStr = &msg
		log.Printf("[NOTIFICATION SERVICE] Gagal mengirim pesan WA uncovered ke %s: %v", targetPhone, sendErr)
	} else {
		sentAt = &now
		log.Printf("[NOTIFICATION SERVICE] ✅ Berhasil mengirim WA status uncovered (%s - %s) ke %s via %s", templateCode, regNo, targetPhone, provider)
	}

	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO notifications (id, channel, recipient, subject, body, status, error_message, sent_at, created_at)
		 VALUES (gen_random_uuid(), 'WHATSAPP', $1, $2, $3, $4, $5, $6, NOW())`,
		targetPhone, subject, finalMessage, status, errStr, sentAt,
	)
}
