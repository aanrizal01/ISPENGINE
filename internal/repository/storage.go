package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"isp-onboarding/internal/domain"
	_ "modernc.org/sqlite"
)

type Storage interface {
	// ODP operations
	ListODPs(ctx context.Context, branchCode string) ([]domain.ODPNode, error)
	GetODPByID(ctx context.Context, id string) (*domain.ODPNode, error)
	GetODPByCode(ctx context.Context, code string) (*domain.ODPNode, error)
	CreateODP(ctx context.Context, odp *domain.ODPNode) error
	DeleteODP(ctx context.Context, idOrCode string) error
	FindNearestAvailableODP(ctx context.Context, lat, lng float64, maxDistanceMeters float64) (*domain.ODPNode, float64, error)
	IncrementODPUsedPort(ctx context.Context, odpID string) error
	DecrementODPUsedPort(ctx context.Context, odpID string) error
	ListClusters(ctx context.Context, branchCode string) ([]domain.ClusterSummary, error)
	SetClusterStatus(ctx context.Context, clusterArea string, active bool, branchCode string) error

	// Registration operations
	CreateRegistration(ctx context.Context, reg *domain.Registration) error
	GetRegistrationByID(ctx context.Context, id string) (*domain.Registration, error)
	GetRegistrationByNo(ctx context.Context, regNo string) (*domain.Registration, error)
	GetRegistrationByGigabillCustomerID(ctx context.Context, customerID string) (*domain.Registration, error)
	UpdateRegistrationContact(ctx context.Context, id, email, phone string) error
	ListRegistrationsByCustomer(ctx context.Context, identifier string) ([]domain.Registration, error)
	ListRegistrations(ctx context.Context, partnerID *string, status *string, branchCode *string) ([]domain.Registration, error)
	ListRegistrationsForPartner(ctx context.Context, partnerID string) ([]domain.Registration, error)
	ClaimRegistration(ctx context.Context, regNo string, partnerID string) (*domain.Registration, error)
	UpdateRegistrationStatus(ctx context.Context, id string, status string, gigabillCustID, gigabillSubID *string) error
	UpdateRegistrationKTP(ctx context.Context, regNo string, ktpURL string) error
	UpdateRegistrationContract(ctx context.Context, regNo string, signatureURL string) error
	UpdateRegistrationHousePhoto(ctx context.Context, regNo string, housePhotoURL string) error
	UpdateRegistrationSitePIC(ctx context.Context, regNo string, picName string, picPhone string) error
	SuspendRegistration(ctx context.Context, idOrNo string, reason string) error
	ResumeRegistration(ctx context.Context, idOrNo string) error
	AttachRegistrationToPartnerODP(ctx context.Context, idOrNo string, req domain.AttachPartnerODPRequest) (*domain.Registration, error)
	MarkRegistrationUncovered(ctx context.Context, idOrNo string, action string, reason string) (*domain.Registration, error)
	UpdateRegistrationPricing(ctx context.Context, idOrNo string, req domain.UpdateRegistrationPricingRequest) (*domain.Registration, error)
	NOCApprovalRegistration(ctx context.Context, idOrNo string, action string, approver string, notes string) (*domain.Registration, error)
	UpgradeBandwidth(ctx context.Context, idOrNo string, req domain.UpgradeBandwidthRequest) (*domain.Registration, error)
	UpdateRegistrationODP(ctx context.Context, regID string, odpID string, odpCode string, distanceMeters float64, notes string) error
	UpdateRegistrationPPPoE(ctx context.Context, regNo string, pppoeUser string, pppoePass string) error
	UpdateRegistrationUpstreamPPPoE(ctx context.Context, regNo string, upstreamUser string, upstreamPass string) error
	GetNextPPPoESequence(ctx context.Context, clusterCode string) (int, error)
	DeleteRegistration(ctx context.Context, idOrRegNo string) error

	// Work Order & BAST
	CreateWorkOrder(ctx context.Context, wo *domain.WorkOrder) error
	GetWorkOrderByID(ctx context.Context, id string) (*domain.WorkOrder, error)
	GetWorkOrderByRegistrationID(ctx context.Context, regID string) (*domain.WorkOrder, error)
	ListWorkOrders(ctx context.Context, status *string) ([]domain.WorkOrder, error)
	AssignWorkOrder(ctx context.Context, idOrRegID string, techName string, notes string) error
	SaveBAST(ctx context.Context, bast *domain.BASTReport) error
	AttachSmartOLTDevice(ctx context.Context, idOrRegNo string, sn string, mac string, opticalPower float64, status string, pppoeUser string) error

	// Partner
	GetPartnerByAPIKey(ctx context.Context, apiKey string) (*domain.Partner, error)
	GetPartnerByID(ctx context.Context, id string) (*domain.Partner, error)
	GetPartnerByCode(ctx context.Context, code string) (*domain.Partner, error)
	CreatePartner(ctx context.Context, p *domain.Partner) error
	UpdatePartner(ctx context.Context, p *domain.Partner) error
	ListPartners(ctx context.Context, branchCode string) ([]domain.Partner, error)
	GetActiveCustomerByReferralCode(ctx context.Context, code string) (*domain.Registration, error)

	// Staff Users & Super User
	ListStaffUsers(ctx context.Context) ([]domain.StaffUser, error)
	CreateStaffUser(ctx context.Context, u *domain.StaffUser) error
	UpdateStaffUser(ctx context.Context, u *domain.StaffUser) error
	UpdateStaffPassword(ctx context.Context, username, newPasswordHash string) error
	GetExecutiveOverview(ctx context.Context, branchCode string) (*domain.ExecutiveOverview, error)

	// JARTAPLOK Partner B2B & Multi-Rekanan
	ListODPsByProvider(ctx context.Context, providerID string) ([]domain.ODPNode, error)
	BatchUpsertODPs(ctx context.Context, providerID, providerName string, odps []domain.ODPNode) (*domain.KMLUploadResult, error)
	ListJartaplokPartners(ctx context.Context, branchCode string) ([]domain.JartaplokPartner, error)
	GetJartaplokPartnerByAPIKey(ctx context.Context, apiKey string) (*domain.JartaplokPartner, error)
	GetJartaplokPartnerByCode(ctx context.Context, code string) (*domain.JartaplokPartner, error)
	CreateJartaplokPartner(ctx context.Context, p *domain.JartaplokPartner) error
	UpdateJartaplokPartner(ctx context.Context, p *domain.JartaplokPartner) error
	GetJartaplokBillingSummary(ctx context.Context) (*domain.JartaplokBillingSummary, error)

	// Branch operations
	ListBranches(ctx context.Context) ([]domain.Branch, error)
	GetBranchByID(ctx context.Context, id string) (*domain.Branch, error)
	GetBranchByCode(ctx context.Context, code string) (*domain.Branch, error)
	GetJartaplokBillingSummaryForPartner(ctx context.Context, partnerCode string) (*domain.JartaplokBillingSummary, error)
	GetJartaplokActivePorts(ctx context.Context) ([]domain.JartaplokActivePort, error)
	GetJartaplokActivePortsForPartner(ctx context.Context, partnerCode string) ([]domain.JartaplokActivePort, error)

	// Staff Auth & Session operations
	GetStaffUserByUsername(ctx context.Context, username string) (*domain.StaffUser, error)
	CreateAuthSession(ctx context.Context, sess *domain.AuthSession) error
	GetAuthSession(ctx context.Context, token string) (*domain.AuthSession, error)
	DeleteAuthSession(ctx context.Context, token string) error

	// Customer Auth (Password & OTP)
	GetCustomerPasswordHash(ctx context.Context, regID string) (string, error)
	UpdateCustomerPassword(ctx context.Context, phoneOrEmailOrID string, passwordHash string) error
	StoreCustomerOTP(ctx context.Context, phone string, otpCode string, expiresAt time.Time) error
	VerifyCustomerOTP(ctx context.Context, phone string, otpCode string) (bool, error)

	// Staff KPI & Performance
	GetStaffKPISummary(ctx context.Context, branchCode string) (*domain.StaffKPISummary, error)

	// Cluster SmartOLT Operations
	ListClusterSmartOLTConfigs(ctx context.Context) ([]domain.ClusterSmartOLTConfig, error)
	GetClusterSmartOLTConfig(ctx context.Context, clusterName string) (*domain.ClusterSmartOLTConfig, error)
	SaveClusterSmartOLTConfig(ctx context.Context, cfg *domain.ClusterSmartOLTConfig) error
	DeleteClusterSmartOLTConfig(ctx context.Context, clusterName string) error

	Close() error
}

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	s := &SQLiteStorage{db: db}
	if err := s.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return s, nil
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

func (s *SQLiteStorage) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS partners (
			id TEXT PRIMARY KEY,
			code TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			api_key TEXT UNIQUE NOT NULL,
			commission_rate REAL NOT NULL DEFAULT 0.0,
			contact_phone TEXT NOT NULL,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS odp_nodes (
			id TEXT PRIMARY KEY,
			code TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			latitude REAL NOT NULL,
			longitude REAL NOT NULL,
			total_ports INTEGER NOT NULL DEFAULT 8,
			used_ports INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'AVAILABLE',
			cluster_area TEXT NOT NULL,
			provider_id TEXT DEFAULT 'GNET-BIARO',
			provider_name TEXT DEFAULT 'PT. GNET BIARO AKSES',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS registrations (
			id TEXT PRIMARY KEY,
			registration_no TEXT UNIQUE NOT NULL,
			partner_id TEXT,
			partner_code TEXT,
			full_name TEXT NOT NULL,
			email TEXT,
			phone TEXT NOT NULL,
			id_card_number TEXT NOT NULL,
			tax_id TEXT DEFAULT '',
			address TEXT NOT NULL,
			latitude REAL NOT NULL,
			longitude REAL NOT NULL,
			selected_plan_id TEXT NOT NULL,
			selected_plan_name TEXT NOT NULL,
			nearest_odp_id TEXT,
			nearest_odp_code TEXT,
			distance_to_odp_meters REAL NOT NULL,
			status TEXT NOT NULL DEFAULT 'SUBMITTED',
			ktp_photo_url TEXT,
			house_photo_url TEXT,
			site_pic_name TEXT,
			site_pic_phone TEXT,
			contract_signature_url TEXT,
			contract_signed_at DATETIME,
			activated_at DATETIME,
			suspended_at DATETIME,
			suspension_reason TEXT,
			gigabill_customer_id TEXT,
			gigabill_subscription_id TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS work_orders (
			id TEXT PRIMARY KEY,
			order_no TEXT UNIQUE NOT NULL,
			registration_id TEXT NOT NULL,
			type TEXT NOT NULL,
			technician_name TEXT NOT NULL,
			scheduled_at DATETIME NOT NULL,
			status TEXT NOT NULL DEFAULT 'ASSIGNED',
			notes TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS bast_reports (
			id TEXT PRIMARY KEY,
			work_order_id TEXT UNIQUE NOT NULL,
			optical_power_dbm REAL NOT NULL,
			ont_serial_number TEXT NOT NULL,
			ont_mac_address TEXT NOT NULL,
			dropcore_length_meters INTEGER NOT NULL,
			customer_signature_url TEXT,
			proof_photo_url TEXT,
			house_photo_url TEXT,
			speedtest_down_mbps REAL NOT NULL,
			speedtest_up_mbps REAL NOT NULL,
			notes TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS staff_users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			full_name TEXT NOT NULL,
			role TEXT NOT NULL,
			contact_phone TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'ACTIVE',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS jartaplok_partners (
			id TEXT PRIMARY KEY,
			code TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			api_key TEXT UNIQUE NOT NULL,
			contact_phone TEXT NOT NULL,
			coverage_area TEXT NOT NULL,
			rate_50m INTEGER NOT NULL DEFAULT 50000,
			rate_100m INTEGER NOT NULL DEFAULT 90000,
			rate_150m INTEGER NOT NULL DEFAULT 135000,
			rate_200m INTEGER NOT NULL DEFAULT 180000,
			rate_300m INTEGER NOT NULL DEFAULT 270000,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS auth_sessions (
			token TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			username TEXT NOT NULL,
			role TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}

	// Idempotent column migrations for existing tables
	_, _ = s.db.Exec(`ALTER TABLE staff_users ADD COLUMN password_hash TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE staff_users ADD COLUMN roles TEXT DEFAULT '[]';`)
	_, _ = s.db.Exec(`ALTER TABLE staff_users ADD COLUMN is_superuser INTEGER DEFAULT 0;`)
	_, _ = s.db.Exec(`ALTER TABLE auth_sessions ADD COLUMN roles TEXT DEFAULT '[]';`)
	_, _ = s.db.Exec(`ALTER TABLE auth_sessions ADD COLUMN is_superuser INTEGER DEFAULT 0;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN contract_signature_url TEXT;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN contract_signed_at DATETIME;`)
	_, _ = s.db.Exec(`ALTER TABLE bast_reports ADD COLUMN house_photo_url TEXT;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN site_pic_name TEXT;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN site_pic_phone TEXT;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN activated_at DATETIME;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN suspended_at DATETIME;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN suspension_reason TEXT;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN dispatch_notes TEXT;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN custom_notes TEXT;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN otc_fee INTEGER DEFAULT 0;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN monthly_price INTEGER DEFAULT 0;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN otc_notes TEXT;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN tax_id TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN pppoe_username TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN pppoe_password TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN upstream_pppoe_username TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN upstream_pppoe_password TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE bast_reports ADD COLUMN upstream_pppoe_username TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE bast_reports ADD COLUMN upstream_pppoe_password TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN customer_password_hash TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN ont_serial_number TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN ont_optical_power REAL DEFAULT 0;`)
	_, _ = s.db.Exec(`ALTER TABLE registrations ADD COLUMN ont_status TEXT DEFAULT '';`)
	_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS customer_otps (
		id TEXT PRIMARY KEY,
		phone TEXT NOT NULL,
		otp_code TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		used INTEGER DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_customer_otps_phone ON customer_otps(phone);`)
	_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS cluster_smartolt_configs (
		id TEXT PRIMARY KEY,
		cluster_name TEXT UNIQUE NOT NULL,
		provider_id TEXT NOT NULL DEFAULT '',
		integration_type TEXT NOT NULL DEFAULT 'SMARTOLT',
		smartolt_url TEXT NOT NULL DEFAULT '',
		smartolt_api_key TEXT NOT NULL DEFAULT '',
		olt_id TEXT NOT NULL DEFAULT '',
		zone_id TEXT NOT NULL DEFAULT '',
		zone_name TEXT NOT NULL DEFAULT '',
		is_active INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`)
	_, _ = s.db.Exec(`INSERT OR IGNORE INTO cluster_smartolt_configs (id, cluster_name, provider_id, integration_type, smartolt_url, smartolt_api_key, olt_id, zone_id, zone_name, is_active)
		VALUES ('cfg-payakumbuh-001', 'Golden Payakumbuh', 'GNET-BIARO', 'SMARTOLT', 'https://gnet-biaro.smartolt.com', 'b71455fab579457d98d8f4b6d28cfdff', '4', '122', 'GOGIGA', 1);`)
	_, _ = s.db.Exec(`INSERT OR IGNORE INTO cluster_smartolt_configs (id, cluster_name, provider_id, integration_type, smartolt_url, smartolt_api_key, olt_id, zone_id, zone_name, is_active)
		VALUES ('cfg-biaro-002', 'Golden Net Biaro', 'GNET-BIARO', 'SMARTOLT', 'https://gnet-biaro.smartolt.com', 'b71455fab579457d98d8f4b6d28cfdff', '', '', 'GOGIGA', 1);`)
	_, _ = s.db.Exec(`UPDATE registrations SET activated_at = created_at WHERE activated_at IS NULL AND (status = 'ACTIVE' OR status = 'SUSPENDED');`)
	_, _ = s.db.Exec(`ALTER TABLE odp_nodes ADD COLUMN provider_id TEXT DEFAULT 'GNET-BIARO';`)
	_, _ = s.db.Exec(`ALTER TABLE odp_nodes ADD COLUMN provider_name TEXT DEFAULT 'PT. GNET BIARO AKSES';`)
	_, _ = s.db.Exec(`ALTER TABLE jartaplok_partners ADD COLUMN service_type TEXT DEFAULT 'SEWA_PORT_FO';`)
	_, _ = s.db.Exec(`ALTER TABLE jartaplok_partners ADD COLUMN rate_20m INTEGER DEFAULT 0;`)
	_, _ = s.db.Exec(`ALTER TABLE jartaplok_partners ADD COLUMN rate_30m INTEGER DEFAULT 0;`)
	_, _ = s.db.Exec(`ALTER TABLE jartaplok_partners ADD COLUMN rate_40m INTEGER DEFAULT 0;`)
	_, _ = s.db.Exec(`ALTER TABLE jartaplok_partners ADD COLUMN otc_fee INTEGER DEFAULT 0;`)
	_, _ = s.db.Exec(`ALTER TABLE jartaplok_partners ADD COLUMN max_distance_meters REAL DEFAULT 250.0;`)
	_, _ = s.db.Exec(`ALTER TABLE jartaplok_partners ADD COLUMN pricing_model TEXT DEFAULT 'Standar Jartaplok';`)
	_, _ = s.db.Exec(`ALTER TABLE jartaplok_partners ADD COLUMN suspension_policy TEXT DEFAULT 'ALLOWED_WITH_WAIVER';`)

	// Insert Default Demo ODPs if table is empty
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM odp_nodes`).Scan(&count)
	if count == 0 {
		_ = s.seedDemoData()
	}

	// Backfill existing ODP nodes with default provider
	_, _ = s.db.Exec(`UPDATE odp_nodes SET provider_id = 'GNET-BIARO' WHERE provider_id IS NULL OR provider_id = ''`)
	_, _ = s.db.Exec(`UPDATE odp_nodes SET provider_name = (SELECT name FROM jartaplok_partners WHERE jartaplok_partners.code = odp_nodes.provider_id) WHERE provider_id IN (SELECT code FROM jartaplok_partners);`)

	// Seed default JARTAPLOK Partner (PT. GNET BIARO AKSES)
	var jpCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM jartaplok_partners`).Scan(&jpCount)
	if jpCount == 0 {
		_, _ = s.db.Exec(`
			INSERT OR IGNORE INTO jartaplok_partners (
				id, code, name, api_key, contact_phone, coverage_area,
				service_type, suspension_policy, pricing_model, rate_20m, rate_30m, rate_40m,
				rate_50m, rate_100m, rate_150m, rate_200m, rate_300m, otc_fee, max_distance_meters,
				is_active, created_at, updated_at
			) VALUES (
				'gnet-biaro-001', 'GNET-BIARO', 'PT. GNET BIARO AKSES', 'jartaplok2026', '085186866164', 'Bukittinggi, Biaro & Agam',
				'SEWA_PORT_FO', 'ALLOWED_WITH_WAIVER', 'Sewa Port FO Pasif (Jartaplok)', 0, 0, 0,
				50000, 90000, 135000, 180000, 270000, 0, 250.0,
				1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
			)
		`)
	}

	// Ensure PT Telkom Infrastruktur Indonesia (TIF) exists with BAKES TIF GMT.pdf pricing & specifications
	_, _ = s.db.Exec(`
		UPDATE jartaplok_partners
		SET name = 'PT Telkom Infrastruktur Indonesia',
		    service_type = 'BITSTREAM',
		    suspension_policy = 'DISALLOWED',
		    pricing_model = 'Bitstream Intra Standard Symetric 1:1 (Zona-2 Sumatera)',
		    rate_20m = 154000,
		    rate_30m = 169000,
		    rate_40m = 184000,
		    rate_50m = 200000,
		    rate_100m = 270000,
		    rate_150m = 0,
		    rate_200m = 400000,
		    rate_300m = 0,
		    otc_fee = 500000,
		    max_distance_meters = 150.0,
		    coverage_area = 'Sumatera Barat (Zona-2 Sumatera)'
		WHERE code = 'TELKO-PYK' OR code = 'TIF' OR name LIKE '%Telkom%'
	`)

	_ = s.seedDemoCustomerData()
	_ = s.seedStaffUsers()
	_ = s.seedDefaultPartners()

	return nil
}

func (s *SQLiteStorage) seedDefaultPartners() error {
	partners := []domain.Partner{
		{
			ID:             "5db117c6-5d41-4155-ad22-c0c1b7a94c47",
			Code:           "PARTNER-OFFICIAL",
			Name:           "PT GOGIGA MEDIA TEKNOLOGI (Kantor Pusat Direct)",
			APIKey:         "part_official_key_9988",
			CommissionRate: 0.0,
			ContactPhone:   "081200000000",
			IsActive:       true,
		},
	}
	for _, p := range partners {
		_, _ = s.db.Exec(
			`INSERT OR IGNORE INTO partners (id, code, name, api_key, commission_rate, contact_phone, is_active, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			p.ID, p.Code, p.Name, p.APIKey, p.CommissionRate, p.ContactPhone, 1, time.Now(),
		)
	}
	_, _ = s.db.Exec(`UPDATE partners SET name = 'PT GOGIGA MEDIA TEKNOLOGI (Kantor Pusat Direct)' WHERE code = 'PARTNER-OFFICIAL'`)
	return nil
}

func (s *SQLiteStorage) seedDemoData() error {
	demoODPs := []domain.ODPNode{
		{
			ID:          uuid.NewString(),
			Code:        "ODP-CKR-001",
			Name:        "Tiang Jl. Merpati No 12",
			Latitude:    -6.2088,
			Longitude:   106.8456,
			TotalPorts:  8,
			UsedPorts:   3,
			Status:      "AVAILABLE",
			ClusterArea: "Cluster Cikarang Barat",
		},
		{
			ID:          uuid.NewString(),
			Code:        "ODP-CKR-002",
			Name:        "Tiang Depan Masjid Al-Ikhlas",
			Latitude:    -6.2095,
			Longitude:   106.8462,
			TotalPorts:  8,
			UsedPorts:   5,
			Status:      "AVAILABLE",
			ClusterArea: "Cluster Cikarang Barat",
		},
		{
			ID:          uuid.NewString(),
			Code:        "ODP-CKR-003",
			Name:        "Tiang Blok B Pertigaan",
			Latitude:    -6.2110,
			Longitude:   106.8480,
			TotalPorts:  16,
			UsedPorts:   8,
			Status:      "AVAILABLE",
			ClusterArea: "Cluster Cikarang Barat",
		},
	}

	for _, o := range demoODPs {
		_, _ = s.db.Exec(
			`INSERT INTO odp_nodes (id, code, name, latitude, longitude, total_ports, used_ports, status, cluster_area, provider_id, provider_name, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'GNET-BIARO', 'PT. GNET BIARO AKSES', ?, ?)`,
			o.ID, o.Code, o.Name, o.Latitude, o.Longitude, o.TotalPorts, o.UsedPorts, o.Status, o.ClusterArea, time.Now(), time.Now(),
		)
	}

	return s.seedDefaultPartners()
}

func (s *SQLiteStorage) seedDemoCustomerData() error {
	// Demo customer seeding disabled for production
	return nil
}

// Haversine formula to compute distance in meters between two lat/lng points
func HaversineDistanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371000 // Earth's radius in meters
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	lat1Rad := lat1 * (math.Pi / 180.0)
	lat2Rad := lat2 * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1Rad)*math.Cos(lat2Rad)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

func (s *SQLiteStorage) FindNearestAvailableODP(ctx context.Context, lat, lng float64, maxDistanceMeters float64) (*domain.ODPNode, float64, error) {
	odps, err := s.ListODPs(ctx, "")
	if err != nil {
		return nil, 0, err
	}

	var nearest *domain.ODPNode
	minDist := math.MaxFloat64

	for i := range odps {
		odp := &odps[i]
		if odp.Status != "AVAILABLE" || odp.AvailablePorts() <= 0 {
			continue
		}

		dist := HaversineDistanceMeters(lat, lng, odp.Latitude, odp.Longitude)
		if dist < minDist {
			minDist = dist
			nearest = odp
		}
	}

	if nearest == nil {
		return nil, 0, nil
	}

	return nearest, minDist, nil
}

func (s *SQLiteStorage) ListODPs(ctx context.Context, branchCode string) ([]domain.ODPNode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.id, o.code, o.name, o.latitude, o.longitude, o.total_ports, o.used_ports, o.status, o.cluster_area, 
		       COALESCE(o.provider_id, 'GNET-BIARO'), 
		       COALESCE(jp.name, o.provider_name, 'PT. GNET BIARO DATA'), 
		       o.created_at, o.updated_at 
		FROM odp_nodes o 
		LEFT JOIN jartaplok_partners jp ON o.provider_id = jp.code 
		WHERE o.status = 'AVAILABLE'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.ODPNode
	for rows.Next() {
		var o domain.ODPNode
		if err := rows.Scan(&o.ID, &o.Code, &o.Name, &o.Latitude, &o.Longitude, &o.TotalPorts, &o.UsedPorts, &o.Status, &o.ClusterArea, &o.ProviderID, &o.ProviderName, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, nil
}

func (s *SQLiteStorage) ListODPsByProvider(ctx context.Context, providerID string) ([]domain.ODPNode, error) {
	query := `
		SELECT o.id, o.code, o.name, o.latitude, o.longitude, o.total_ports, o.used_ports, o.status, o.cluster_area, 
		       COALESCE(o.provider_id, 'GNET-BIARO'), 
		       COALESCE(jp.name, o.provider_name, 'PT. GNET BIARO DATA'), 
		       o.created_at, o.updated_at 
		FROM odp_nodes o 
		LEFT JOIN jartaplok_partners jp ON o.provider_id = jp.code
	`
	var rows *sql.Rows
	var err error
	if providerID != "" && providerID != "ALL" {
		query += ` WHERE o.provider_id = ? ORDER BY o.code ASC`
		rows, err = s.db.QueryContext(ctx, query, providerID)
	} else {
		query += ` ORDER BY o.code ASC`
		rows, err = s.db.QueryContext(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.ODPNode
	for rows.Next() {
		var o domain.ODPNode
		if err := rows.Scan(&o.ID, &o.Code, &o.Name, &o.Latitude, &o.Longitude, &o.TotalPorts, &o.UsedPorts, &o.Status, &o.ClusterArea, &o.ProviderID, &o.ProviderName, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, nil
}

func (s *SQLiteStorage) ListClusters(ctx context.Context, branchCode string) ([]domain.ClusterSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cluster_area, 
		       COUNT(*) as total, 
		       COALESCE(SUM(CASE WHEN status = 'AVAILABLE' THEN 1 ELSE 0 END), 0) as active
		FROM odp_nodes
		GROUP BY cluster_area
		ORDER BY total DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.ClusterSummary
	for rows.Next() {
		var c domain.ClusterSummary
		if err := rows.Scan(&c.Name, &c.TotalODPs, &c.ActiveODPs); err != nil {
			return nil, err
		}
		c.IsActive = c.ActiveODPs > 0
		list = append(list, c)
	}
	return list, nil
}

func (s *SQLiteStorage) SetClusterStatus(ctx context.Context, clusterArea string, active bool, branchCode string) error {
	status := "INACTIVE"
	if active {
		status = "AVAILABLE"
	}
	_, err := s.db.ExecContext(ctx, `UPDATE odp_nodes SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE cluster_area = ?`, status, clusterArea)
	return err
}

func (s *SQLiteStorage) GetODPByID(ctx context.Context, id string) (*domain.ODPNode, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, code, name, latitude, longitude, total_ports, used_ports, status, cluster_area, COALESCE(provider_id, 'GNET-BIARO'), COALESCE(provider_name, 'PT. GNET BIARO AKSES'), created_at, updated_at FROM odp_nodes WHERE id = ?`, id)
	var o domain.ODPNode
	if err := row.Scan(&o.ID, &o.Code, &o.Name, &o.Latitude, &o.Longitude, &o.TotalPorts, &o.UsedPorts, &o.Status, &o.ClusterArea, &o.ProviderID, &o.ProviderName, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *SQLiteStorage) CreateODP(ctx context.Context, odp *domain.ODPNode) error {
	if odp.ID == "" {
		odp.ID = uuid.NewString()
	}
	if odp.ProviderID == "" {
		odp.ProviderID = "GNET-BIARO"
	}
	if odp.ProviderName == "" {
		odp.ProviderName = "PT. GNET BIARO AKSES"
	}
	if odp.TotalPorts <= 0 {
		odp.TotalPorts = 8
	}
	if odp.Status == "" {
		odp.Status = "AVAILABLE"
	}
	odp.CreatedAt = time.Now()
	odp.UpdatedAt = time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO odp_nodes (id, code, name, latitude, longitude, total_ports, used_ports, status, cluster_area, provider_id, provider_name, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(code) DO UPDATE SET
		   name = excluded.name,
		   latitude = excluded.latitude,
		   longitude = excluded.longitude,
		   total_ports = excluded.total_ports,
		   used_ports = excluded.used_ports,
		   status = excluded.status,
		   cluster_area = excluded.cluster_area,
		   provider_id = excluded.provider_id,
		   provider_name = excluded.provider_name,
		   updated_at = excluded.updated_at`,
		odp.ID, odp.Code, odp.Name, odp.Latitude, odp.Longitude, odp.TotalPorts, odp.UsedPorts, odp.Status, odp.ClusterArea, odp.ProviderID, odp.ProviderName, odp.CreatedAt, odp.UpdatedAt,
	)
	return err
}

func (s *SQLiteStorage) DeleteODP(ctx context.Context, idOrCode string) error {
	var providerID, providerName string
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(provider_id, ''), COALESCE(provider_name, '') FROM odp_nodes WHERE id = ? OR code = ?`, idOrCode, idOrCode).Scan(&providerID, &providerName)
	if err == nil && (providerID == "GNET2" || strings.Contains(providerName, "(2)")) {
		return fmt.Errorf("tiang ODP ini adalah aset mitra Jartaplok (%s) yang dikelola otomatis via API FTTX. Pengelolaan tiang hanya dapat dilakukan melalui FTTX Management System penyedia", providerName)
	}

	var activeCustCount int
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM registrations 
		WHERE (nearest_odp_id = ? OR nearest_odp_code = ?) 
		  AND status NOT IN ('CANCELLED_NO_COVERAGE', 'REJECTED')
	`, idOrCode, idOrCode).Scan(&activeCustCount)
	if err != nil {
		return err
	}
	if activeCustCount > 0 {
		return fmt.Errorf("tiang ODP tidak dapat dihapus karena masih melayani %d pelanggan aktif/antrean. Silakan alihkan pelanggan ke tiang lain terlebih dahulu", activeCustCount)
	}

	res, err := s.db.ExecContext(ctx, `DELETE FROM odp_nodes WHERE id = ? OR code = ?`, idOrCode, idOrCode)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("tiang ODP tidak ditemukan")
	}
	return nil
}

func (s *SQLiteStorage) IncrementODPUsedPort(ctx context.Context, odpID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE odp_nodes 
		 SET used_ports = used_ports + 1,
		     status = CASE WHEN used_ports + 1 >= total_ports THEN 'FULL' ELSE status END,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		odpID,
	)
	return err
}

func (s *SQLiteStorage) DecrementODPUsedPort(ctx context.Context, odpID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE odp_nodes 
		 SET used_ports = MAX(0, used_ports - 1),
		     status = CASE WHEN (used_ports - 1) < total_ports AND status = 'FULL' THEN 'AVAILABLE' ELSE status END,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		odpID,
	)
	return err
}

func (s *SQLiteStorage) GetODPByCode(ctx context.Context, code string) (*domain.ODPNode, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, code, name, latitude, longitude, total_ports, used_ports, status, cluster_area, 
		        COALESCE(provider_id, 'GNET-BIARO'), COALESCE(provider_name, 'PT. GNET BIARO AKSES'), 
		        created_at, updated_at 
		 FROM odp_nodes WHERE UPPER(code) = UPPER(?) LIMIT 1`,
		strings.TrimSpace(code),
	)
	var o domain.ODPNode
	if err := row.Scan(&o.ID, &o.Code, &o.Name, &o.Latitude, &o.Longitude, &o.TotalPorts, &o.UsedPorts, &o.Status, &o.ClusterArea, &o.ProviderID, &o.ProviderName, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *SQLiteStorage) DeleteRegistration(ctx context.Context, idOrRegNo string) error {
	var regID, nearestODPID, nearestODPCode string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(nearest_odp_id, ''), COALESCE(nearest_odp_code, '') 
		FROM registrations 
		WHERE id = ? OR registration_no = ?
	`, idOrRegNo, idOrRegNo).Scan(&regID, &nearestODPID, &nearestODPCode)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("permohonan registrasi tidak ditemukan")
		}
		return err
	}

	// 1. Delete BAST reports associated with work orders of this registration
	_, _ = s.db.ExecContext(ctx, `DELETE FROM bast_reports WHERE work_order_id IN (SELECT id FROM work_orders WHERE registration_id = ?)`, regID)

	// 2. Delete work orders of this registration
	_, _ = s.db.ExecContext(ctx, `DELETE FROM work_orders WHERE registration_id = ?`, regID)

	// 3. Delete registration record
	res, err := s.db.ExecContext(ctx, `DELETE FROM registrations WHERE id = ?`, regID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("permohonan registrasi tidak ditemukan")
	}

	// 4. Recalculate ODP port usage cleanly
	if nearestODPID != "" || nearestODPCode != "" {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE odp_nodes 
			SET used_ports = (
				SELECT count(id) FROM registrations 
				WHERE (nearest_odp_id = odp_nodes.id OR nearest_odp_code = odp_nodes.code) 
				  AND status IN ('ACTIVE', 'INSTALLATION_SCHEDULED', 'SURVEY_SCHEDULED')
			),
			status = CASE 
				WHEN (
					SELECT count(id) FROM registrations 
					WHERE (nearest_odp_id = odp_nodes.id OR nearest_odp_code = odp_nodes.code) 
					  AND status IN ('ACTIVE', 'INSTALLATION_SCHEDULED', 'SURVEY_SCHEDULED')
				) >= total_ports THEN 'FULL' 
				ELSE 'AVAILABLE' 
			END,
			updated_at = CURRENT_TIMESTAMP
			WHERE id = ? OR code = ?
		`, nearestODPID, nearestODPCode)
	}

	return nil
}

func (s *SQLiteStorage) CreateRegistration(ctx context.Context, reg *domain.Registration) error {
	if reg.ID == "" {
		reg.ID = uuid.NewString()
	}
	reg.CreatedAt = time.Now()
	reg.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO registrations (
			id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, tax_id,
			address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code,
			distance_to_odp_meters, status, ktp_photo_url, house_photo_url, site_pic_name, site_pic_phone, dispatch_notes, custom_notes, otc_fee, monthly_price, otc_notes, pppoe_username, pppoe_password, upstream_pppoe_username, upstream_pppoe_password, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		reg.ID, reg.RegistrationNo, reg.PartnerID, reg.PartnerCode, reg.FullName, reg.Email, reg.Phone, reg.IDCardNumber, reg.TaxID,
		reg.Address, reg.Latitude, reg.Longitude, reg.SelectedPlanID, reg.SelectedPlanName, reg.NearestODPID, reg.NearestODPCode,
		reg.DistanceToODPMeters, reg.Status, reg.KTPPhotoURL, reg.HousePhotoURL, reg.SitePICName, reg.SitePICPhone, reg.DispatchNotes, reg.CustomNotes, reg.OTCFee, reg.MonthlyPrice, reg.OTCNotes, reg.PPPoEUsername, reg.PPPoEPassword, reg.UpstreamPPPoEUsername, reg.UpstreamPPPoEPassword, reg.CreatedAt, reg.UpdatedAt,
	)
	return err
}

func (s *SQLiteStorage) GetRegistrationByID(ctx context.Context, id string) (*domain.Registration, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, COALESCE(tax_id, ''), address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code, distance_to_odp_meters, status, COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''), COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''), COALESCE(contract_signature_url, ''), contract_signed_at, gigabill_customer_id, gigabill_subscription_id, activated_at, suspended_at, COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''), COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''), COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''), COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''),
		       COALESCE((SELECT jp.suspension_policy FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'ALLOWED_WITH_WAIVER'),
		       COALESCE((SELECT jp.name FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'PT. GNET BIARO AKSES'),
		       created_at, updated_at 
		FROM registrations WHERE id = ?`, id)
	return scanRegistration(row)
}

func (s *SQLiteStorage) GetRegistrationByNo(ctx context.Context, identifier string) (*domain.Registration, error) {
	clean := strings.TrimSpace(identifier)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, COALESCE(tax_id, ''), address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code, distance_to_odp_meters, status, COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''), COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''), COALESCE(contract_signature_url, ''), contract_signed_at, gigabill_customer_id, gigabill_subscription_id, activated_at, suspended_at, COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''), COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''), COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''), COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''),
		       COALESCE((SELECT jp.suspension_policy FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'ALLOWED_WITH_WAIVER'),
		       COALESCE((SELECT jp.name FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'PT. GNET BIARO AKSES'),
		       created_at, updated_at 
		FROM registrations 
		WHERE registration_no = ? 
		   OR gigabill_customer_id = ?
		   OR (email != '' AND LOWER(email) = LOWER(?)) 
		   OR (phone != '' AND phone = ?)
		ORDER BY created_at DESC LIMIT 1`,
		clean, clean, clean, clean,
	)
	return scanRegistration(row)
}

func (s *SQLiteStorage) GetRegistrationByGigabillCustomerID(ctx context.Context, customerID string) (*domain.Registration, error) {
	clean := strings.TrimSpace(customerID)
	if clean == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, COALESCE(tax_id, ''), address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code, distance_to_odp_meters, status, COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''), COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''), COALESCE(contract_signature_url, ''), contract_signed_at, gigabill_customer_id, gigabill_subscription_id, activated_at, suspended_at, COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''), COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''), COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''), COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''),
		       COALESCE((SELECT jp.suspension_policy FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'ALLOWED_WITH_WAIVER'),
		       COALESCE((SELECT jp.name FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'PT. GNET BIARO AKSES'),
		       created_at, updated_at 
		FROM registrations 
		WHERE gigabill_customer_id = ?
		ORDER BY created_at DESC LIMIT 1`,
		clean,
	)
	return scanRegistration(row)
}

func (s *SQLiteStorage) UpdateRegistrationContact(ctx context.Context, id, email, phone string) error {
	cleanID := strings.TrimSpace(id)
	cleanEmail := strings.TrimSpace(email)
	cleanPhone := strings.TrimSpace(phone)
	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET email = CASE WHEN ? != '' THEN ? ELSE email END,
		    phone = CASE WHEN ? != '' THEN ? ELSE phone END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		cleanEmail, cleanEmail, cleanPhone, cleanPhone, cleanID,
	)
	return err
}

func (s *SQLiteStorage) ListRegistrationsByCustomer(ctx context.Context, identifier string) ([]domain.Registration, error) {
	clean := strings.TrimSpace(identifier)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, COALESCE(tax_id, ''), address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code, distance_to_odp_meters, status, COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''), COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''), COALESCE(contract_signature_url, ''), contract_signed_at, gigabill_customer_id, gigabill_subscription_id, activated_at, suspended_at, COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''), COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''), COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''), COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''),
		       COALESCE((SELECT jp.suspension_policy FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'ALLOWED_WITH_WAIVER'),
		       COALESCE((SELECT jp.name FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'PT. GNET BIARO AKSES'),
		       created_at, updated_at 
		FROM registrations 
		WHERE registration_no = ? 
		   OR gigabill_customer_id = ?
		   OR (id_card_number != '' AND id_card_number = ?)
		   OR (email != '' AND LOWER(email) = LOWER(?)) 
		   OR (phone != '' AND phone = ?)
		ORDER BY created_at DESC`,
		clean, clean, clean, clean, clean,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Registration
	for rows.Next() {
		var r domain.Registration
		var contractSig string
		var contractSignedAt, actAt, suspAt sql.NullTime
		var suspReason, dispNotes, customNotes, otcNotes string
		var otcFee, monthlyPrice int64
		err := rows.Scan(
			&r.ID, &r.RegistrationNo, &r.PartnerID, &r.PartnerCode, &r.FullName, &r.Email, &r.Phone, &r.IDCardNumber, &r.TaxID,
			&r.Address, &r.Latitude, &r.Longitude, &r.SelectedPlanID, &r.SelectedPlanName, &r.NearestODPID, &r.NearestODPCode,
			&r.DistanceToODPMeters, &r.Status, &r.KTPPhotoURL, &r.HousePhotoURL, &r.SitePICName, &r.SitePICPhone, &contractSig, &contractSignedAt, &r.GigabillCustomerID, &r.GigabillSubscriptionID,
			&actAt, &suspAt, &suspReason, &dispNotes, &customNotes, &otcFee, &monthlyPrice, &otcNotes, &r.PPPoEUsername, &r.PPPoEPassword, &r.UpstreamPPPoEUsername, &r.UpstreamPPPoEPassword,
			&r.PartnerSuspensionPolicy, &r.PartnerName,
			&r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		r.ContractSignatureURL = contractSig
		if contractSignedAt.Valid {
			r.ContractSignedAt = &contractSignedAt.Time
		}
		if actAt.Valid {
			r.ActivatedAt = &actAt.Time
		}
		if suspAt.Valid {
			r.SuspendedAt = &suspAt.Time
		}
		r.SuspensionReason = suspReason
		r.DispatchNotes = dispNotes
		r.CustomNotes = customNotes
		r.OTCFee = otcFee
		r.MonthlyPrice = monthlyPrice
		r.OTCNotes = otcNotes
		list = append(list, r)
	}
	return list, nil
}

func scanRegistration(row *sql.Row) (*domain.Registration, error) {
	var r domain.Registration
	var contractSig string
	var contractSignedAt, actAt, suspAt sql.NullTime
	var suspReason, dispNotes, customNotes, otcNotes string
	var otcFee, monthlyPrice int64
	err := row.Scan(
		&r.ID, &r.RegistrationNo, &r.PartnerID, &r.PartnerCode, &r.FullName, &r.Email, &r.Phone, &r.IDCardNumber, &r.TaxID,
		&r.Address, &r.Latitude, &r.Longitude, &r.SelectedPlanID, &r.SelectedPlanName, &r.NearestODPID, &r.NearestODPCode,
		&r.DistanceToODPMeters, &r.Status, &r.KTPPhotoURL, &r.HousePhotoURL, &r.SitePICName, &r.SitePICPhone, &contractSig, &contractSignedAt, &r.GigabillCustomerID, &r.GigabillSubscriptionID,
		&actAt, &suspAt, &suspReason, &dispNotes, &customNotes, &otcFee, &monthlyPrice, &otcNotes, &r.PPPoEUsername, &r.PPPoEPassword, &r.UpstreamPPPoEUsername, &r.UpstreamPPPoEPassword,
		&r.PartnerSuspensionPolicy, &r.PartnerName,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.ContractSignatureURL = contractSig
	if contractSignedAt.Valid {
		r.ContractSignedAt = &contractSignedAt.Time
	}
	if actAt.Valid {
		r.ActivatedAt = &actAt.Time
	}
	if suspAt.Valid {
		r.SuspendedAt = &suspAt.Time
	}
	r.SuspensionReason = suspReason
	r.DispatchNotes = dispNotes
	r.CustomNotes = customNotes
	r.OTCFee = otcFee
	r.MonthlyPrice = monthlyPrice
	r.OTCNotes = otcNotes
	return &r, nil
}

func (s *SQLiteStorage) ListRegistrations(ctx context.Context, partnerID *string, status *string, branchCode *string) ([]domain.Registration, error) {
	query := `SELECT id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, COALESCE(tax_id, ''), address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code, distance_to_odp_meters, status, COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''), COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''), COALESCE(contract_signature_url, ''), contract_signed_at, gigabill_customer_id, gigabill_subscription_id, activated_at, suspended_at, COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''), COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''), COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''), COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''),
	       COALESCE((SELECT jp.suspension_policy FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'ALLOWED_WITH_WAIVER'),
	       COALESCE((SELECT jp.name FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'PT. GNET BIARO AKSES'),
	       created_at, updated_at FROM registrations WHERE 1=1`
	var args []interface{}

	if partnerID != nil && *partnerID != "" {
		query += ` AND (partner_id = ? OR partner_code = (SELECT code FROM partners WHERE id = ?))`
		args = append(args, *partnerID, *partnerID)
	}
	if status != nil && *status != "" {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Registration
	for rows.Next() {
		var r domain.Registration
		var contractSig string
		var contractSignedAt, actAt, suspAt sql.NullTime
		var suspReason, dispNotes, customNotes, otcNotes string
		var otcFee, monthlyPrice int64
		if err := rows.Scan(
			&r.ID, &r.RegistrationNo, &r.PartnerID, &r.PartnerCode, &r.FullName, &r.Email, &r.Phone, &r.IDCardNumber, &r.TaxID,
			&r.Address, &r.Latitude, &r.Longitude, &r.SelectedPlanID, &r.SelectedPlanName, &r.NearestODPID, &r.NearestODPCode,
			&r.DistanceToODPMeters, &r.Status, &r.KTPPhotoURL, &r.HousePhotoURL, &r.SitePICName, &r.SitePICPhone, &contractSig, &contractSignedAt, &r.GigabillCustomerID, &r.GigabillSubscriptionID,
			&actAt, &suspAt, &suspReason, &dispNotes, &customNotes, &otcFee, &monthlyPrice, &otcNotes, &r.PPPoEUsername, &r.PPPoEPassword, &r.UpstreamPPPoEUsername, &r.UpstreamPPPoEPassword,
			&r.PartnerSuspensionPolicy, &r.PartnerName,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		r.ContractSignatureURL = contractSig
		if contractSignedAt.Valid {
			r.ContractSignedAt = &contractSignedAt.Time
		}
		if actAt.Valid {
			r.ActivatedAt = &actAt.Time
		}
		if suspAt.Valid {
			r.SuspendedAt = &suspAt.Time
		}
		r.SuspensionReason = suspReason
		r.DispatchNotes = dispNotes
		r.CustomNotes = customNotes
		r.OTCFee = otcFee
		r.MonthlyPrice = monthlyPrice
		r.OTCNotes = otcNotes
		list = append(list, r)
	}
	return list, nil
}

func (s *SQLiteStorage) ListRegistrationsForPartner(ctx context.Context, partnerID string) ([]domain.Registration, error) {
	partner, err := s.GetPartnerByID(ctx, partnerID)
	if err != nil || partner == nil {
		return nil, fmt.Errorf("data partner tidak ditemukan: %w", err)
	}

	query := `SELECT id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, COALESCE(tax_id, ''), address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code, distance_to_odp_meters, status, COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''), COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''), COALESCE(contract_signature_url, ''), contract_signed_at, gigabill_customer_id, gigabill_subscription_id, activated_at, suspended_at, COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''), COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''), COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''), COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''),
	       COALESCE((SELECT jp.suspension_policy FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'ALLOWED_WITH_WAIVER'),
	       COALESCE((SELECT jp.name FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'PT. GNET BIARO AKSES'),
	       created_at, updated_at FROM registrations 
	WHERE (partner_id = ? OR partner_code = ?)
	   OR ((partner_id IS NULL OR partner_id = '') AND (
	        selected_plan_id = 'paket-custom-enterprise' 
	        OR (custom_notes IS NOT NULL AND custom_notes != '') 
	        OR selected_plan_name LIKE '%Custom%' 
	        OR selected_plan_name LIKE '%Corporate%' 
	        OR selected_plan_name LIKE '%Dedicated%'
	   ))
	ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, partner.ID, partner.Code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Registration
	for rows.Next() {
		var r domain.Registration
		var contractSig string
		var contractSignedAt, actAt, suspAt sql.NullTime
		var suspReason, dispNotes, customNotes, otcNotes string
		var otcFee, monthlyPrice int64
		if err := rows.Scan(
			&r.ID, &r.RegistrationNo, &r.PartnerID, &r.PartnerCode, &r.FullName, &r.Email, &r.Phone, &r.IDCardNumber, &r.TaxID,
			&r.Address, &r.Latitude, &r.Longitude, &r.SelectedPlanID, &r.SelectedPlanName, &r.NearestODPID, &r.NearestODPCode,
			&r.DistanceToODPMeters, &r.Status, &r.KTPPhotoURL, &r.HousePhotoURL, &r.SitePICName, &r.SitePICPhone, &contractSig, &contractSignedAt, &r.GigabillCustomerID, &r.GigabillSubscriptionID,
			&actAt, &suspAt, &suspReason, &dispNotes, &customNotes, &otcFee, &monthlyPrice, &otcNotes, &r.PPPoEUsername, &r.PPPoEPassword, &r.UpstreamPPPoEUsername, &r.UpstreamPPPoEPassword,
			&r.PartnerSuspensionPolicy, &r.PartnerName,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		r.ContractSignatureURL = contractSig
		if contractSignedAt.Valid {
			r.ContractSignedAt = &contractSignedAt.Time
		}
		if actAt.Valid {
			r.ActivatedAt = &actAt.Time
		}
		if suspAt.Valid {
			r.SuspendedAt = &suspAt.Time
		}
		r.SuspensionReason = suspReason
		r.DispatchNotes = dispNotes
		r.CustomNotes = customNotes
		r.OTCFee = otcFee
		r.MonthlyPrice = monthlyPrice
		r.OTCNotes = otcNotes
		list = append(list, r)
	}
	return list, nil
}

func (s *SQLiteStorage) ClaimRegistration(ctx context.Context, regNo string, partnerID string) (*domain.Registration, error) {
	partner, err := s.GetPartnerByID(ctx, partnerID)
	if err != nil || partner == nil {
		return nil, fmt.Errorf("partner tidak ditemukan")
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET partner_id = ?, partner_code = ?, updated_at = CURRENT_TIMESTAMP
		WHERE (registration_no = ? OR id = ?) AND (partner_id IS NULL OR partner_id = '')
	`, partner.ID, partner.Code, regNo, regNo)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return nil, fmt.Errorf("registrasi tidak dapat diklaim (mungkin sudah diklaim oleh rekanan sales lain atau telah terikat referral)")
	}

	return s.GetRegistrationByNo(ctx, regNo)
}

func (s *SQLiteStorage) UpdateRegistrationPricing(ctx context.Context, idOrNo string, req domain.UpdateRegistrationPricingRequest) (*domain.Registration, error) {
	clean := strings.TrimSpace(idOrNo)
	taxID := strings.TrimSpace(req.TaxID)
	planID := strings.TrimSpace(req.SelectedPlanID)
	planName := strings.TrimSpace(req.SelectedPlanName)
	odpCode := strings.TrimSpace(req.ODPCode)

	newStatusExpr := "status"
	if req.RequestNOCApproval {
		newStatusExpr = "'WAITING_APPROVAL_NOC'"
	} else if req.PromoteToInstall {
		newStatusExpr = "'INSTALLATION_SCHEDULED'"
	}

	query := fmt.Sprintf(`
		UPDATE registrations 
		SET otc_fee = ?, monthly_price = ?, otc_notes = ?, 
		    tax_id = CASE WHEN ? != '' THEN ? ELSE tax_id END,
		    selected_plan_id = CASE WHEN ? != '' THEN ? ELSE selected_plan_id END,
		    selected_plan_name = CASE WHEN ? != '' THEN ? ELSE selected_plan_name END,
		    nearest_odp_code = CASE WHEN ? != '' THEN ? ELSE nearest_odp_code END,
		    status = %s,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? OR registration_no = ?
	`, newStatusExpr)

	res, err := s.db.ExecContext(ctx, query,
		req.OTCFee, req.MonthlyPrice, strings.TrimSpace(req.OTCNotes), 
		taxID, taxID, planID, planID, planName, planName, odpCode, odpCode,
		clean, clean)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return nil, fmt.Errorf("registrasi %s tidak ditemukan", clean)
	}

	if req.PromoteToInstall && !req.RequestNOCApproval {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE work_orders
			SET type = 'INSTALLATION', status = 'ASSIGNED', updated_at = CURRENT_TIMESTAMP
			WHERE registration_id = (SELECT id FROM registrations WHERE id = ? OR registration_no = ? LIMIT 1)
		`, clean, clean)
	}

	if strings.HasPrefix(clean, "REG-") {
		return s.GetRegistrationByNo(ctx, clean)
	}
	return s.GetRegistrationByID(ctx, clean)
}

func (s *SQLiteStorage) NOCApprovalRegistration(ctx context.Context, idOrNo string, action string, approver string, notes string) (*domain.Registration, error) {
	clean := strings.TrimSpace(idOrNo)
	isApprove := strings.ToUpper(strings.TrimSpace(action)) == "APPROVE"
	targetStatus := "SURVEY_REVISED"
	if isApprove {
		targetStatus = "INSTALLATION_SCHEDULED"
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations
		SET status = ?,
		    dispatch_notes = CASE WHEN ? != '' THEN dispatch_notes || ' | NOC Approval: ' || ? ELSE dispatch_notes END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? OR registration_no = ?
	`, targetStatus, notes, notes, clean, clean)
	if err != nil {
		return nil, err
	}

	if isApprove {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE work_orders
			SET type = 'INSTALLATION',
			    status = 'ASSIGNED',
			    notes = notes || ' | Disetujui NOC: Siap Instalasi',
			    updated_at = CURRENT_TIMESTAMP
			WHERE registration_id = (SELECT id FROM registrations WHERE id = ? OR registration_no = ? LIMIT 1)
		`, clean, clean)
	} else {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE work_orders
			SET status = 'FAILED',
			    notes = notes || ' | Ditolak NOC: ' || ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE registration_id = (SELECT id FROM registrations WHERE id = ? OR registration_no = ? LIMIT 1)
		`, notes, clean, clean)
	}

	if strings.HasPrefix(clean, "REG-") {
		return s.GetRegistrationByNo(ctx, clean)
	}
	return s.GetRegistrationByID(ctx, clean)
}

func (s *SQLiteStorage) UpgradeBandwidth(ctx context.Context, idOrNo string, req domain.UpgradeBandwidthRequest) (*domain.Registration, error) {
	clean := strings.TrimSpace(idOrNo)
	newPlanID := strings.TrimSpace(req.NewPlanID)
	newPlanName := strings.TrimSpace(req.NewPlanName)
	if newPlanName == "" {
		return nil, fmt.Errorf("nama paket baru wajib diisi")
	}

	var current *domain.Registration
	var err error
	if strings.HasPrefix(clean, "REG-") {
		current, err = s.GetRegistrationByNo(ctx, clean)
	} else {
		current, err = s.GetRegistrationByID(ctx, clean)
	}
	if err != nil || current == nil {
		return nil, fmt.Errorf("registrasi %s tidak ditemukan", clean)
	}

	effDate := strings.TrimSpace(req.EffectiveDate)
	if effDate == "" {
		effDate = "SEGERA"
	}

	logNote := fmt.Sprintf("\n[Upgrade Paket %s]: %s (Rp %d) -> %s (Rp %d) [Efektif: %s]",
		time.Now().Format("2006-01-02 15:04"),
		current.SelectedPlanName, current.MonthlyPrice,
		newPlanName, req.NewMonthlyPrice,
		effDate,
	)
	if req.Notes != "" {
		logNote += " | Catatan: " + strings.TrimSpace(req.Notes)
	}

	var updatedDispatch string
	if current.DispatchNotes != "" {
		updatedDispatch = strings.TrimSpace(current.DispatchNotes) + "\n" + strings.TrimSpace(logNote)
	} else {
		updatedDispatch = strings.TrimSpace(logNote)
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET selected_plan_id = CASE WHEN ? != '' THEN ? ELSE selected_plan_id END,
		    selected_plan_name = ?,
		    monthly_price = ?,
		    dispatch_notes = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? OR registration_no = ?
	`, newPlanID, newPlanID, newPlanName, req.NewMonthlyPrice, updatedDispatch, clean, clean)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return nil, fmt.Errorf("registrasi %s tidak ditemukan", clean)
	}

	if strings.HasPrefix(clean, "REG-") {
		return s.GetRegistrationByNo(ctx, clean)
	}
	return s.GetRegistrationByID(ctx, clean)
}


func (s *SQLiteStorage) UpdateRegistrationStatus(ctx context.Context, id string, status string, gigabillCustID, gigabillSubID *string) error {
	var err error
	if status == "ACTIVE" {
		_, err = s.db.ExecContext(ctx,
			`UPDATE registrations
			 SET status = ?, gigabill_customer_id = COALESCE(?, gigabill_customer_id), gigabill_subscription_id = COALESCE(?, gigabill_subscription_id),
			     activated_at = COALESCE(activated_at, CURRENT_TIMESTAMP), suspended_at = NULL, suspension_reason = NULL, updated_at = CURRENT_TIMESTAMP
			 WHERE id = ?`,
			status, gigabillCustID, gigabillSubID, id,
		)
	} else {
		_, err = s.db.ExecContext(ctx,
			`UPDATE registrations
			 SET status = ?, gigabill_customer_id = COALESCE(?, gigabill_customer_id), gigabill_subscription_id = COALESCE(?, gigabill_subscription_id), updated_at = CURRENT_TIMESTAMP
			 WHERE id = ?`,
			status, gigabillCustID, gigabillSubID, id,
		)
	}
	return err
}

func (s *SQLiteStorage) SuspendRegistration(ctx context.Context, idOrNo string, reason string) error {
	now := time.Now()
	if strings.TrimSpace(reason) == "" {
		reason = "Permintaan jeda layanan sementara"
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE registrations
		 SET status = 'SUSPENDED', suspended_at = ?, suspension_reason = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? OR registration_no = ?`,
		now, reason, idOrNo, idOrNo,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("registrasi %s tidak ditemukan", idOrNo)
	}
	return nil
}

func (s *SQLiteStorage) ResumeRegistration(ctx context.Context, idOrNo string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE registrations
		 SET status = 'ACTIVE', suspended_at = NULL, suspension_reason = NULL, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? OR registration_no = ?`,
		idOrNo, idOrNo,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("registrasi %s tidak ditemukan", idOrNo)
	}
	return nil
}

func (s *SQLiteStorage) AttachRegistrationToPartnerODP(ctx context.Context, idOrNo string, req domain.AttachPartnerODPRequest) (*domain.Registration, error) {
	var reg *domain.Registration
	var err error
	if strings.HasPrefix(idOrNo, "REG-") {
		reg, err = s.GetRegistrationByNo(ctx, idOrNo)
	} else {
		reg, err = s.GetRegistrationByID(ctx, idOrNo)
	}
	if err != nil || reg == nil {
		return nil, fmt.Errorf("pendaftaran tidak ditemukan: %w", err)
	}

	partnerCode := strings.TrimSpace(req.PartnerCode)
	if partnerCode == "" {
		partnerCode = "TELKO-PYK"
	}
	partner, err := s.GetJartaplokPartnerByCode(ctx, partnerCode)
	if err != nil || partner == nil {
		return nil, fmt.Errorf("mitra wholesale dengan kode '%s' tidak ditemukan", partnerCode)
	}

	odpCode := strings.ToUpper(strings.TrimSpace(req.ODPCode))
	if odpCode == "" {
		return nil, fmt.Errorf("kode ODP tidak boleh kosong")
	}

	var odpID string
	var existingCode string
	err = s.db.QueryRowContext(ctx, `SELECT id, code FROM odp_nodes WHERE code = ?`, odpCode).Scan(&odpID, &existingCode)
	if err == sql.ErrNoRows {
		odpID = uuid.NewString()
		odpName := strings.TrimSpace(req.ODPName)
		if odpName == "" {
			odpName = fmt.Sprintf("Tiang ODP %s (%s)", odpCode, partner.Name)
		}
		clusterArea := partner.CoverageArea
		if clusterArea == "" {
			clusterArea = reg.Address
		}
		lat := reg.Latitude
		lng := reg.Longitude

		_, err = s.db.ExecContext(ctx, `
			INSERT INTO odp_nodes (id, code, name, latitude, longitude, total_ports, used_ports, status, cluster_area, provider_id, provider_name, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 8, 1, 'AVAILABLE', ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			odpID, odpCode, odpName, lat, lng, clusterArea, partner.Code, partner.Name,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal membuat titik ODP baru untuk %s: %w", partner.Name, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("error memeriksa ODP: %w", err)
	} else {
		_, _ = s.db.ExecContext(ctx, `UPDATE odp_nodes SET provider_id = ?, provider_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, partner.Code, partner.Name, odpID)
	}

	newStatus := reg.Status
	if reg.Status == "PENDING_SURVEY_OVERDISTANCE" || reg.Status == "SUBMITTED" || reg.Status == "UNCOVERED_WISHLIST" || reg.Status == "SURVEY_SCHEDULED" {
		newStatus = "INSTALLATION_SCHEDULED"
	}

	notes := strings.TrimSpace(req.Notes)
	if notes == "" {
		notes = fmt.Sprintf("Dialihkan/di-attach manual ke infrastruktur %s (ODP: %s)", partner.Name, odpCode)
	}

	dist := req.DistanceMeters
	if dist <= 0 {
		dist = 50.0
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE registrations
		SET nearest_odp_id = ?, nearest_odp_code = ?, distance_to_odp_meters = ?, status = ?, dispatch_notes = ?,
		    upstream_pppoe_username = CASE WHEN ? != '' THEN ? ELSE upstream_pppoe_username END,
		    upstream_pppoe_password = CASE WHEN ? != '' THEN ? ELSE upstream_pppoe_password END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		odpID, odpCode, dist, newStatus, notes,
		strings.TrimSpace(req.UpstreamPPPoEUsername), strings.TrimSpace(req.UpstreamPPPoEUsername),
		strings.TrimSpace(req.UpstreamPPPoEPassword), strings.TrimSpace(req.UpstreamPPPoEPassword),
		reg.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("gagal memperbarui pendaftaran: %w", err)
	}

	_, _ = s.db.ExecContext(ctx, `
		UPDATE work_orders
		SET type = 'INSTALLATION', status = 'ASSIGNED', updated_at = CURRENT_TIMESTAMP
		WHERE registration_id = ? AND type = 'SURVEY' AND status != 'COMPLETED'`,
		reg.ID,
	)

	return s.GetRegistrationByID(ctx, reg.ID)
}

func (s *SQLiteStorage) MarkRegistrationUncovered(ctx context.Context, idOrNo string, action string, reason string) (*domain.Registration, error) {
	var reg *domain.Registration
	var err error
	if strings.HasPrefix(idOrNo, "REG-") {
		reg, err = s.GetRegistrationByNo(ctx, idOrNo)
	} else {
		reg, err = s.GetRegistrationByID(ctx, idOrNo)
	}
	if err != nil || reg == nil {
		return nil, fmt.Errorf("pendaftaran tidak ditemukan: %w", err)
	}

	newStatus := "UNCOVERED_WISHLIST"
	if strings.ToUpper(action) == "CANCEL" {
		newStatus = "CANCELLED_NO_COVERAGE"
	}

	cleanReason := strings.TrimSpace(reason)
	if cleanReason == "" {
		if newStatus == "UNCOVERED_WISHLIST" {
			cleanReason = "Lokasi belum tercover jalur FO. Disimpan ke dalam antrean prioritas perluasan jaringan."
		} else {
			cleanReason = "Dibatalkan resmi karena lokasi di luar jangkauan infrastruktur fiber optik."
		}
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE registrations
		SET status = ?, dispatch_notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		newStatus, cleanReason, reg.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("gagal memperbarui status: %w", err)
	}

	_, _ = s.db.ExecContext(ctx, `
		UPDATE work_orders
		SET status = 'FAILED', notes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE registration_id = ? AND status != 'COMPLETED'`,
		cleanReason, reg.ID,
	)

	return s.GetRegistrationByID(ctx, reg.ID)
}

func (s *SQLiteStorage) UpdateRegistrationODP(ctx context.Context, regID string, odpID string, odpCode string, distanceMeters float64, notes string) error {
	var query string
	var args []interface{}
	cleanNotes := strings.TrimSpace(notes)
	if cleanNotes != "" {
		query = `UPDATE registrations 
		         SET nearest_odp_id = ?, nearest_odp_code = ?, distance_to_odp_meters = ?, 
		             dispatch_notes = CASE WHEN dispatch_notes IS NULL OR dispatch_notes = '' THEN ? ELSE dispatch_notes || ' | ' || ? END,
		             updated_at = CURRENT_TIMESTAMP 
		         WHERE id = ? OR registration_no = ?`
		args = []interface{}{odpID, odpCode, distanceMeters, cleanNotes, cleanNotes, regID, regID}
	} else {
		query = `UPDATE registrations 
		         SET nearest_odp_id = ?, nearest_odp_code = ?, distance_to_odp_meters = ?, updated_at = CURRENT_TIMESTAMP 
		         WHERE id = ? OR registration_no = ?`
		args = []interface{}{odpID, odpCode, distanceMeters, regID, regID}
	}
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func (s *SQLiteStorage) UpdateRegistrationPPPoE(ctx context.Context, regNo string, pppoeUser string, pppoePass string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET pppoe_username = ?, pppoe_password = ?, updated_at = CURRENT_TIMESTAMP
		WHERE registration_no = ? OR id = ?
	`, pppoeUser, pppoePass, regNo, regNo)
	return err
}

func (s *SQLiteStorage) UpdateRegistrationUpstreamPPPoE(ctx context.Context, regNo string, upstreamUser string, upstreamPass string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET upstream_pppoe_username = ?, upstream_pppoe_password = ?, updated_at = CURRENT_TIMESTAMP
		WHERE registration_no = ? OR id = ?
	`, upstreamUser, upstreamPass, regNo, regNo)
	return err
}

func (s *SQLiteStorage) GetNextPPPoESequence(ctx context.Context, clusterCode string) (int, error) {
	var maxSeq sql.NullInt64
	// Match: branchPrefix (2 digit) + clusterCode (3 digit) + urut (5 digit) + @gogiga.net.id
	query := `SELECT MAX(CAST(SUBSTR(pppoe_username, 6, 5) AS INTEGER)) FROM registrations WHERE pppoe_username LIKE '%' || ? || '%@gogiga.net.id'`
	err := s.db.QueryRowContext(ctx, query, clusterCode).Scan(&maxSeq)
	if err != nil && err != sql.ErrNoRows {
		return 1, err
	}
	if !maxSeq.Valid || maxSeq.Int64 <= 0 {
		return 1, nil
	}
	return int(maxSeq.Int64) + 1, nil
}

func (s *SQLiteStorage) UpdateRegistrationKTP(ctx context.Context, regNo string, ktpURL string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE registrations
		 SET ktp_photo_url = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE registration_no = ?`,
		ktpURL, regNo,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("registration %s not found", regNo)
	}
	return nil
}

func (s *SQLiteStorage) UpdateRegistrationContract(ctx context.Context, regNo string, signatureURL string) error {
	now := time.Now()
	res, err := s.db.ExecContext(ctx,
		`UPDATE registrations
		 SET contract_signature_url = ?, contract_signed_at = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE registration_no = ?`,
		signatureURL, now, regNo,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("registration %s not found", regNo)
	}
	return nil
}

func (s *SQLiteStorage) UpdateRegistrationHousePhoto(ctx context.Context, regNo string, housePhotoURL string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE registrations
		 SET house_photo_url = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE registration_no = ?`,
		housePhotoURL, regNo,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("registration %s not found", regNo)
	}
	return nil
}

func (s *SQLiteStorage) UpdateRegistrationSitePIC(ctx context.Context, regNo string, picName string, picPhone string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE registrations
		 SET site_pic_name = ?, site_pic_phone = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE registration_no = ?`,
		picName, picPhone, regNo,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("registration %s not found", regNo)
	}
	return nil
}

func (s *SQLiteStorage) CreateWorkOrder(ctx context.Context, wo *domain.WorkOrder) error {
	if wo.ID == "" {
		wo.ID = uuid.NewString()
	}
	wo.CreatedAt = time.Now()
	wo.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO work_orders (id, order_no, registration_id, type, technician_name, scheduled_at, status, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		wo.ID, wo.OrderNo, wo.RegistrationID, wo.Type, wo.TechnicianName, wo.ScheduledAt, wo.Status, wo.Notes, wo.CreatedAt, wo.UpdatedAt,
	)
	return err
}

func (s *SQLiteStorage) GetWorkOrderByID(ctx context.Context, id string) (*domain.WorkOrder, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, order_no, registration_id, type, technician_name, scheduled_at, status, notes, created_at, updated_at FROM work_orders WHERE id = ?`,
		id,
	)
	var wo domain.WorkOrder
	if err := row.Scan(&wo.ID, &wo.OrderNo, &wo.RegistrationID, &wo.Type, &wo.TechnicianName, &wo.ScheduledAt, &wo.Status, &wo.Notes, &wo.CreatedAt, &wo.UpdatedAt); err != nil {
		return nil, err
	}

	// Try scan BAST if exists
	bastRow := s.db.QueryRowContext(ctx,
		`SELECT id, work_order_id, optical_power_dbm, ont_serial_number, ont_mac_address, dropcore_length_meters, customer_signature_url, proof_photo_url, house_photo_url, speedtest_down_mbps, speedtest_up_mbps, notes, COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''), created_at
		 FROM bast_reports WHERE work_order_id = ?`,
		wo.ID,
	)
	var b domain.BASTReport
	var housePhoto sql.NullString
	if err := bastRow.Scan(&b.ID, &b.WorkOrderID, &b.OpticalPowerDBM, &b.ONTSerialNumber, &b.ONTMACAddress, &b.DropcoreLengthMeters, &b.CustomerSignatureURL, &b.ProofPhotoURL, &housePhoto, &b.SpeedtestDownMbps, &b.SpeedtestUpMbps, &b.Notes, &b.UpstreamPPPoEUsername, &b.UpstreamPPPoEPassword, &b.CreatedAt); err == nil {
		if housePhoto.Valid {
			b.HousePhotoURL = housePhoto.String
		}
		wo.BAST = &b
	}

	return &wo, nil
}

func (s *SQLiteStorage) GetWorkOrderByRegistrationID(ctx context.Context, regID string) (*domain.WorkOrder, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, order_no, registration_id, type, technician_name, scheduled_at, status, notes, created_at, updated_at FROM work_orders WHERE registration_id = ? ORDER BY created_at DESC LIMIT 1`,
		regID,
	)
	var wo domain.WorkOrder
	if err := row.Scan(&wo.ID, &wo.OrderNo, &wo.RegistrationID, &wo.Type, &wo.TechnicianName, &wo.ScheduledAt, &wo.Status, &wo.Notes, &wo.CreatedAt, &wo.UpdatedAt); err != nil {
		return nil, err
	}

	// Try scan BAST if exists
	bastRow := s.db.QueryRowContext(ctx,
		`SELECT id, work_order_id, optical_power_dbm, ont_serial_number, ont_mac_address, dropcore_length_meters, customer_signature_url, proof_photo_url, house_photo_url, speedtest_down_mbps, speedtest_up_mbps, notes, COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''), created_at
		 FROM bast_reports WHERE work_order_id = ?`,
		wo.ID,
	)
	var b domain.BASTReport
	var housePhoto sql.NullString
	if err := bastRow.Scan(&b.ID, &b.WorkOrderID, &b.OpticalPowerDBM, &b.ONTSerialNumber, &b.ONTMACAddress, &b.DropcoreLengthMeters, &b.CustomerSignatureURL, &b.ProofPhotoURL, &housePhoto, &b.SpeedtestDownMbps, &b.SpeedtestUpMbps, &b.Notes, &b.UpstreamPPPoEUsername, &b.UpstreamPPPoEPassword, &b.CreatedAt); err == nil {
		if housePhoto.Valid {
			b.HousePhotoURL = housePhoto.String
		}
		wo.BAST = &b
	}

	return &wo, nil
}

func (s *SQLiteStorage) ListWorkOrders(ctx context.Context, status *string) ([]domain.WorkOrder, error) {
	query := `SELECT id, order_no, registration_id, type, technician_name, scheduled_at, status, notes, created_at, updated_at FROM work_orders WHERE 1=1`
	var args []interface{}
	if status != nil && *status != "" {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.WorkOrder
	for rows.Next() {
		var wo domain.WorkOrder
		if err := rows.Scan(&wo.ID, &wo.OrderNo, &wo.RegistrationID, &wo.Type, &wo.TechnicianName, &wo.ScheduledAt, &wo.Status, &wo.Notes, &wo.CreatedAt, &wo.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, wo)
	}
	return list, nil
}

func (s *SQLiteStorage) AssignWorkOrder(ctx context.Context, idOrRegID string, techName string, notes string) error {
	cleanID := strings.TrimSpace(idOrRegID)
	cleanTech := strings.TrimSpace(techName)
	cleanNotes := strings.TrimSpace(notes)
	query := `UPDATE work_orders 
	          SET technician_name = ?, 
	              notes = CASE WHEN ? != '' THEN ? ELSE notes END,
	              updated_at = CURRENT_TIMESTAMP 
	          WHERE id = ? OR registration_id = ?`
	_, err := s.db.ExecContext(ctx, query, cleanTech, cleanNotes, cleanNotes, cleanID, cleanID)
	return err
}

func (s *SQLiteStorage) SaveBAST(ctx context.Context, bast *domain.BASTReport) error {
	if bast.ID == "" {
		bast.ID = uuid.NewString()
	}
	bast.CreatedAt = time.Now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, _ = tx.ExecContext(ctx, `DELETE FROM bast_reports WHERE work_order_id = ?`, bast.WorkOrderID)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO bast_reports (
			id, work_order_id, optical_power_dbm, ont_serial_number, ont_mac_address, dropcore_length_meters,
			customer_signature_url, proof_photo_url, house_photo_url, speedtest_down_mbps, speedtest_up_mbps, notes,
			upstream_pppoe_username, upstream_pppoe_password, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		bast.ID, bast.WorkOrderID, bast.OpticalPowerDBM, bast.ONTSerialNumber, bast.ONTMACAddress, bast.DropcoreLengthMeters,
		bast.CustomerSignatureURL, bast.ProofPhotoURL, bast.HousePhotoURL, bast.SpeedtestDownMbps, bast.SpeedtestUpMbps, bast.Notes,
		bast.UpstreamPPPoEUsername, bast.UpstreamPPPoEPassword, bast.CreatedAt,
	)
	if err != nil {
		return err
	}

	if bast.UpstreamPPPoEUsername != "" {
		_, _ = tx.ExecContext(ctx, `
			UPDATE registrations
			SET upstream_pppoe_username = ?,
			    upstream_pppoe_password = CASE WHEN ? != '' THEN ? ELSE upstream_pppoe_password END,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = (SELECT registration_id FROM work_orders WHERE id = ?)
		`, bast.UpstreamPPPoEUsername, bast.UpstreamPPPoEPassword, bast.UpstreamPPPoEPassword, bast.WorkOrderID)
	}

	if bast.TechnicianName != "" {
		_, err = tx.ExecContext(ctx,
			`UPDATE work_orders SET status = 'COMPLETED', technician_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			bast.TechnicianName, bast.WorkOrderID,
		)
	} else {
		_, err = tx.ExecContext(ctx,
			`UPDATE work_orders SET status = 'COMPLETED', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			bast.WorkOrderID,
		)
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteStorage) AttachSmartOLTDevice(ctx context.Context, idOrRegNo string, sn string, mac string, opticalPower float64, status string, pppoeUser string) error {
	clean := strings.TrimSpace(idOrRegNo)
	if clean == "" {
		return fmt.Errorf("id or registration_no cannot be empty")
	}

	reg, err := s.GetRegistrationByNo(ctx, clean)
	if err != nil || reg == nil {
		reg, err = s.GetRegistrationByID(ctx, clean)
	}
	if err != nil || reg == nil {
		list, _ := s.ListRegistrations(ctx, nil, nil, nil)
		for _, r := range list {
			if strings.EqualFold(r.PPPoEUsername, clean) || strings.EqualFold(r.FullName, clean) ||
				(pppoeUser != "" && strings.EqualFold(r.PPPoEUsername, pppoeUser)) ||
				(pppoeUser != "" && strings.EqualFold(r.FullName, pppoeUser)) {
				reg = &r
				break
			}
		}
	}
	if reg == nil {
		return fmt.Errorf("pelanggan %s tidak ditemukan di database", clean)
	}

	newStatus := reg.Status
	if newStatus == "SUBMITTED" || newStatus == "SURVEY" || newStatus == "INSTALLATION" || newStatus == "INSTALLATION_SCHEDULED" {
		newStatus = "ACTIVE"
	}

	upPPPoE := reg.UpstreamPPPoEUsername
	if upPPPoE == "" {
		upPPPoE = pppoeUser
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET status = ?, 
		    upstream_pppoe_username = ?,
		    activated_at = CASE WHEN activated_at IS NULL THEN CURRENT_TIMESTAMP ELSE activated_at END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, newStatus, upPPPoE, reg.ID)
	if err != nil {
		return err
	}

	wo, _ := s.GetWorkOrderByRegistrationID(ctx, reg.ID)
	woID := ""
	if wo != nil {
		woID = wo.ID
	} else {
		woID = uuid.NewString()
		newWO := &domain.WorkOrder{
			ID:             woID,
			OrderNo:        fmt.Sprintf("WO-%s", time.Now().Format("20060102150405")),
			RegistrationID: reg.ID,
			Type:           "INSTALLATION",
			TechnicianName: "SmartOLT Auto-Provisioning",
			ScheduledAt:    time.Now(),
			Status:         "COMPLETED",
		}
		_ = s.CreateWorkOrder(ctx, newWO)
	}

	bast := &domain.BASTReport{
		WorkOrderID:           woID,
		OpticalPowerDBM:       opticalPower,
		ONTSerialNumber:       sn,
		ONTMACAddress:         mac,
		DropcoreLengthMeters:  50,
		UpstreamPPPoEUsername: upPPPoE,
		Notes:                 fmt.Sprintf("Tersinkronisasi otomatis via SmartOLT Jartaplok (Status: %s)", status),
		TechnicianName:        "SmartOLT Auto-Provisioning",
	}
	return s.SaveBAST(ctx, bast)
}

func (s *SQLiteStorage) GetPartnerByAPIKey(ctx context.Context, apiKey string) (*domain.Partner, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, code, name, api_key, commission_rate, contact_phone, is_active, created_at FROM partners WHERE api_key = ? AND is_active = 1`,
		apiKey,
	)
	var p domain.Partner
	var active int
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.APIKey, &p.CommissionRate, &p.ContactPhone, &active, &p.CreatedAt); err != nil {
		return nil, err
	}
	p.IsActive = active == 1
	return &p, nil
}

func (s *SQLiteStorage) GetPartnerByID(ctx context.Context, id string) (*domain.Partner, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, code, name, api_key, commission_rate, contact_phone, is_active, created_at FROM partners WHERE id = ?`,
		id,
	)
	var p domain.Partner
	var active int
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.APIKey, &p.CommissionRate, &p.ContactPhone, &active, &p.CreatedAt); err != nil {
		return nil, err
	}
	p.IsActive = active == 1
	return &p, nil
}

func (s *SQLiteStorage) GetPartnerByCode(ctx context.Context, code string) (*domain.Partner, error) {
	cleanCode := strings.ToUpper(strings.TrimSpace(code))
	row := s.db.QueryRowContext(ctx,
		`SELECT id, code, name, api_key, commission_rate, contact_phone, is_active, created_at 
		 FROM partners 
		 WHERE (UPPER(code) = ? 
		    OR UPPER(code) = ? || '-PYK' 
		    OR UPPER(code) = ? || '-TECH' 
		    OR UPPER(code) LIKE ? || '-%') 
		   AND is_active = 1
		 ORDER BY CASE 
		    WHEN UPPER(code) = ? THEN 1 
		    WHEN UPPER(code) = ? || '-PYK' THEN 2 
		    WHEN UPPER(code) LIKE ? || '-%' THEN 3 
		    ELSE 4 
		 END
		 LIMIT 1`,
		cleanCode, cleanCode, cleanCode, cleanCode, cleanCode, cleanCode, cleanCode,
	)
	var p domain.Partner
	var active int
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.APIKey, &p.CommissionRate, &p.ContactPhone, &active, &p.CreatedAt); err != nil {
		return nil, err
	}
	p.IsActive = active == 1
	return &p, nil
}

func (s *SQLiteStorage) GetActiveCustomerByReferralCode(ctx context.Context, code string) (*domain.Registration, error) {
	cleanCode := strings.ToUpper(strings.TrimSpace(code))
	row := s.db.QueryRowContext(ctx, `
		SELECT id, registration_no, full_name, phone, status
		FROM registrations
		WHERE (UPPER(registration_no) = ? OR phone = ?)
		  AND status = 'ACTIVE'
		LIMIT 1
	`, cleanCode, strings.TrimSpace(code))
	var r domain.Registration
	if err := row.Scan(&r.ID, &r.RegistrationNo, &r.FullName, &r.Phone, &r.Status); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *SQLiteStorage) CreatePartner(ctx context.Context, p *domain.Partner) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	p.CreatedAt = time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO partners (id, code, name, api_key, commission_rate, contact_phone, is_active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Code, p.Name, p.APIKey, p.CommissionRate, p.ContactPhone, 1, p.CreatedAt,
	)
	return err
}

func (s *SQLiteStorage) UpdatePartner(ctx context.Context, p *domain.Partner) error {
	activeInt := 0
	if p.IsActive {
		activeInt = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE partners
		 SET name = ?, contact_phone = ?, commission_rate = ?, is_active = ?
		 WHERE id = ?`,
		p.Name, p.ContactPhone, p.CommissionRate, activeInt, p.ID,
	)
	return err
}

func (s *SQLiteStorage) ListPartners(ctx context.Context, branchCode string) ([]domain.Partner, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, code, name, api_key, commission_rate, contact_phone, is_active, created_at FROM partners ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Partner, 0)
	for rows.Next() {
		var p domain.Partner
		var active int
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.APIKey, &p.CommissionRate, &p.ContactPhone, &active, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.IsActive = active == 1
		list = append(list, p)
	}
	return list, nil
}

// ── STAFF USERS & SUPER USER OPERATIONS ──

func (s *SQLiteStorage) seedStaffUsers() error {
	hashOwner, _ := bcrypt.GenerateFromPassword([]byte("Owner@GoGiga2026!"), bcrypt.DefaultCost)
	hashNoc, _ := bcrypt.GenerateFromPassword([]byte("Noc@GoGiga2026!"), bcrypt.DefaultCost)

	hashFinance, _ := bcrypt.GenerateFromPassword([]byte("Finance@GoGiga2026!"), bcrypt.DefaultCost)

	// Idempotently update passwords for existing records if empty
	_, _ = s.db.Exec(`UPDATE staff_users SET full_name = 'Aan Rizal S.Kom (Direktur Utama / Owner)' WHERE username = 'owner'`)
	_, _ = s.db.Exec(`UPDATE staff_users SET password_hash = ? WHERE username = 'owner' AND (password_hash IS NULL OR password_hash = '')`, string(hashOwner))
	_, _ = s.db.Exec(`UPDATE staff_users SET username = 'nando_noc', full_name = 'Nando Azkia Putra S.Kom (Kepala NOC & Core FO)' WHERE username = 'fadhil_noc'`)
	_, _ = s.db.Exec(`UPDATE staff_users SET full_name = 'Nando Azkia Putra S.Kom (Kepala NOC & Core FO)' WHERE username = 'nando_noc'`)
	_, _ = s.db.Exec(`UPDATE staff_users SET password_hash = ? WHERE username = 'nando_noc' AND (password_hash IS NULL OR password_hash = '')`, string(hashNoc))
	_, _ = s.db.Exec(`UPDATE staff_users SET full_name = 'Nando Azkia Putra S.Kom (Koordinator Teknisi & Core FO)' WHERE username = 'nando'`)
	_, _ = s.db.Exec(`UPDATE partners SET name = 'Nando Azkia Putra S.Kom (Koordinator Teknisi & Core FO)' WHERE code = 'NANDO-TECH'`)
	_, _ = s.db.Exec(`UPDATE staff_users SET full_name = 'Fajar Malem Sitepu (Account Executive / Sales)' WHERE username = 'fajar'`)
	_, _ = s.db.Exec(`UPDATE partners SET name = 'Fajar Malem Sitepu (Account Executive / Sales)' WHERE code = 'FAJAR-PYK'`)

	// Idempotently ensure finance staff user exists
	_, _ = s.db.Exec(`INSERT OR IGNORE INTO staff_users (id, username, full_name, role, roles, is_superuser, contact_phone, status, password_hash, created_at, updated_at) 
		VALUES (?, 'finance', 'Staf Keuangan & Billing (Finance)', 'FINANCE', '["FINANCE"]', 0, '081199887755', 'ACTIVE', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, uuid.NewString(), string(hashFinance))

	// Idempotently assign multi-roles for Single Sign-On across all subdomains
	_, _ = s.db.Exec(`UPDATE staff_users SET roles = '["SUPER_ADMIN","ADMIN_NOC","FINANCE","SALES","TECHNICIAN","JARTAPLOK"]', is_superuser = 1 WHERE username = 'owner'`)
	_, _ = s.db.Exec(`UPDATE staff_users SET roles = '["ADMIN_NOC","TECHNICIAN"]' WHERE username = 'nando_noc' OR username = 'nando'`)
	_, _ = s.db.Exec(`UPDATE staff_users SET roles = '["SALES"]' WHERE username = 'fajar'`)
	_, _ = s.db.Exec(`UPDATE staff_users SET roles = '["FINANCE"]' WHERE username = 'finance'`)
	_, _ = s.db.Exec(`UPDATE staff_users SET roles = '["TECHNICIAN"]' WHERE username IN ('ricci', 'zikka', 'egi')`)
	_, _ = s.db.Exec(`UPDATE staff_users SET roles = '["JARTAPLOK"]' WHERE username = 'gnet_biaro'`)

	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM staff_users`).Scan(&count)
	if count > 0 {
		return nil
	}

	initialStaff := []domain.StaffUser{
		{
			ID:           uuid.NewString(),
			Username:     "owner",
			FullName:     "Aan Rizal S.Kom (Direktur Utama / Owner)",
			Role:         "SUPER_ADMIN",
			Roles:        []string{"SUPER_ADMIN", "ADMIN_NOC", "FINANCE", "SALES", "TECHNICIAN", "JARTAPLOK"},
			IsSuperuser:  true,
			ContactPhone: "081199887766",
			Status:       "ACTIVE",
			PasswordHash: string(hashOwner),
		},
		{
			ID:           uuid.NewString(),
			Username:     "nando_noc",
			FullName:     "Nando Azkia Putra S.Kom (Kepala NOC & Core FO)",
			Role:         "ADMIN_NOC",
			Roles:        []string{"ADMIN_NOC", "TECHNICIAN"},
			ContactPhone: "081233445566",
			Status:       "ACTIVE",
			PasswordHash: string(hashNoc),
		},
		{
			ID:           uuid.NewString(),
			Username:     "finance",
			FullName:     "Staf Keuangan & Billing (Finance)",
			Role:         "FINANCE",
			Roles:        []string{"FINANCE"},
			ContactPhone: "081199887755",
			Status:       "ACTIVE",
			PasswordHash: string(hashFinance),
		},
	}

	ctx := context.Background()
	for _, u := range initialStaff {
		_ = s.CreateStaffUser(ctx, &u)
	}
	return nil
}

func (s *SQLiteStorage) ListStaffUsers(ctx context.Context) ([]domain.StaffUser, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, full_name, role, COALESCE(roles, '[]'), COALESCE(is_superuser, 0), contact_phone, status, created_at, updated_at
		FROM staff_users
		ORDER BY CASE role
			WHEN 'SUPER_ADMIN' THEN 1
			WHEN 'ADMIN_NOC' THEN 2
			WHEN 'FINANCE' THEN 3
			WHEN 'TECHNICIAN' THEN 4
			ELSE 5 END, full_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.StaffUser
	for rows.Next() {
		var u domain.StaffUser
		var rolesJSON string
		var isSuper int
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &rolesJSON, &isSuper, &u.ContactPhone, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(rolesJSON), &u.Roles)
		if len(u.Roles) == 0 && u.Role != "" {
			u.Roles = []string{u.Role}
		}
		u.IsSuperuser = (isSuper == 1) || u.Role == "SUPER_ADMIN"
		list = append(list, u)
	}
	return list, nil
}

func (s *SQLiteStorage) GetStaffUserByUsername(ctx context.Context, username string) (*domain.StaffUser, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, full_name, role, COALESCE(roles, '[]'), COALESCE(is_superuser, 0), contact_phone, status, password_hash, created_at, updated_at
		FROM staff_users
		WHERE username = ? COLLATE NOCASE
	`, username)
	var u domain.StaffUser
	var rolesJSON string
	var isSuper int
	err := row.Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &rolesJSON, &isSuper, &u.ContactPhone, &u.Status, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal([]byte(rolesJSON), &u.Roles)
	if len(u.Roles) == 0 && u.Role != "" {
		u.Roles = []string{u.Role}
	}
	u.IsSuperuser = (isSuper == 1) || u.Role == "SUPER_ADMIN"
	return &u, nil
}

func (s *SQLiteStorage) CreateAuthSession(ctx context.Context, sess *domain.AuthSession) error {
	now := time.Now()
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = now
	}
	rolesBytes, _ := json.Marshal(sess.Roles)
	isSuper := 0
	if sess.IsSuperuser {
		isSuper = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_sessions (token, user_id, username, role, roles, is_superuser, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, sess.Token, sess.UserID, sess.Username, sess.Role, string(rolesBytes), isSuper, sess.ExpiresAt, sess.CreatedAt)
	return err
}

func (s *SQLiteStorage) GetAuthSession(ctx context.Context, token string) (*domain.AuthSession, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT token, user_id, username, role, COALESCE(roles, '[]'), COALESCE(is_superuser, 0), expires_at, created_at
		FROM auth_sessions
		WHERE token = ? AND expires_at > CURRENT_TIMESTAMP
	`, token)
	var sess domain.AuthSession
	var rolesJSON string
	var isSuper int
	err := row.Scan(&sess.Token, &sess.UserID, &sess.Username, &sess.Role, &rolesJSON, &isSuper, &sess.ExpiresAt, &sess.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal([]byte(rolesJSON), &sess.Roles)
	if len(sess.Roles) == 0 && sess.Role != "" {
		sess.Roles = []string{sess.Role}
	}
	sess.IsSuperuser = (isSuper == 1) || sess.Role == "SUPER_ADMIN"
	return &sess, nil
}

func (s *SQLiteStorage) DeleteAuthSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE token = ?`, token)
	return err
}

func (s *SQLiteStorage) CreateStaffUser(ctx context.Context, u *domain.StaffUser) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	rolesBytes, _ := json.Marshal(u.Roles)
	isSuper := 0
	if u.IsSuperuser || u.Role == "SUPER_ADMIN" {
		isSuper = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO staff_users (id, username, full_name, role, roles, is_superuser, contact_phone, status, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, u.ID, u.Username, u.FullName, u.Role, string(rolesBytes), isSuper, u.ContactPhone, u.Status, u.PasswordHash, u.CreatedAt, u.UpdatedAt)
	return err
}

func (s *SQLiteStorage) UpdateStaffUser(ctx context.Context, u *domain.StaffUser) error {
	u.UpdatedAt = time.Now()
	rolesBytes, _ := json.Marshal(u.Roles)
	isSuper := 0
	if u.IsSuperuser || u.Role == "SUPER_ADMIN" {
		isSuper = 1
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE staff_users
		SET full_name = ?, role = ?, roles = ?, is_superuser = ?, contact_phone = ?, status = ?, updated_at = ?
		WHERE id = ?
	`, u.FullName, u.Role, string(rolesBytes), isSuper, u.ContactPhone, u.Status, u.UpdatedAt, u.ID)
	return err
}

func (s *SQLiteStorage) UpdateStaffPassword(ctx context.Context, username, newPasswordHash string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE staff_users
		SET password_hash = ?, updated_at = ?
		WHERE username = ?
	`, newPasswordHash, time.Now(), username)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("user %s tidak ditemukan", username)
	}
	return nil
}


func (s *SQLiteStorage) GetExecutiveOverview(ctx context.Context, branchCode string) (*domain.ExecutiveOverview, error) {
	overview := &domain.ExecutiveOverview{
		BranchCode: branchCode,
	}

	// 1. Total registrations, active, and suspended customers
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations`).Scan(&overview.TotalRegisteredCustomers)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'ACTIVE'`).Scan(&overview.TotalActiveCustomers)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'SUSPENDED'`).Scan(&overview.TotalSuspendedCustomers)

	// 2. MRR & JARTAPLOK / Bitstream Wholesale Cost (dengan Prorata & Penyesuaian Jeda)
	partnersMap := make(map[string]*domain.JartaplokPartner)
	if plist, err := s.ListJartaplokPartners(ctx, ""); err == nil {
		for i := range plist {
			partnersMap[plist[i].Code] = &plist[i]
		}
	}

	now := time.Now()
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.selected_plan_name, COALESCE(o.provider_id, 'GNET-BIARO'), r.status,
		       r.activated_at, r.created_at, r.suspended_at
		FROM registrations r
		LEFT JOIN odp_nodes o ON r.nearest_odp_code = o.code
		WHERE r.status = 'ACTIVE' OR r.status = 'SUSPENDED'
	`)
	if err == nil {
		defer rows.Close()
		var totalMRR int64
		var totalJartaplokCost int64
		for rows.Next() {
			var planName, provCode, status string
			var actAt, createAt, suspAt sql.NullTime
			if err := rows.Scan(&planName, &provCode, &status, &actAt, &createAt, &suspAt); err == nil {
				actTime := time.Now()
				if actAt.Valid {
					actTime = actAt.Time
				} else if createAt.Valid {
					actTime = createAt.Time
				}
				var planPrice int64
				var baseWholesaleFee int64

				partner := partnersMap[provCode]
				if partner == nil {
					partner = partnersMap["GNET-BIARO"]
				}

				// Retail Plan price (Hanya ditagihkan jika ACTIVE)
				if status == "ACTIVE" {
					switch {
					case strings.Contains(planName, "300M"):
						planPrice = 1199000
					case strings.Contains(planName, "200M"):
						planPrice = 899000
					case strings.Contains(planName, "150M"):
						planPrice = 699000
					case strings.Contains(planName, "100M") || strings.Contains(planName, "Glory"):
						planPrice = 599000
					case strings.Contains(planName, "50M") || strings.Contains(planName, "Honor") || strings.Contains(planName, "FAST"):
						planPrice = 389000
					case strings.Contains(planName, "40M") || strings.Contains(planName, "Legend"):
						planPrice = 329000
					case strings.Contains(planName, "30M") || strings.Contains(planName, "Epic"):
						planPrice = 299000
					case strings.Contains(planName, "20M") || strings.Contains(planName, "Diamond"):
						planPrice = 259000
					case strings.Contains(planName, "10M") || strings.Contains(planName, "Gold"):
						planPrice = 183150
					default:
						planPrice = 250000
					}
				}

				// Base Wholesale Fee according to partner's specific rates & service model
				if partner != nil {
					switch {
					case strings.Contains(planName, "300M"):
						baseWholesaleFee = partner.Rate300M
					case strings.Contains(planName, "200M"):
						baseWholesaleFee = partner.Rate200M
					case strings.Contains(planName, "150M"):
						baseWholesaleFee = partner.Rate150M
					case strings.Contains(planName, "100M") || strings.Contains(planName, "Glory"):
						baseWholesaleFee = partner.Rate100M
					case strings.Contains(planName, "50M") || strings.Contains(planName, "Honor") || strings.Contains(planName, "FAST"):
						baseWholesaleFee = partner.Rate50M
					case strings.Contains(planName, "40M") || strings.Contains(planName, "Legend"):
						if partner.Rate40M > 0 {
							baseWholesaleFee = partner.Rate40M
						} else {
							baseWholesaleFee = partner.Rate50M
						}
					case strings.Contains(planName, "30M") || strings.Contains(planName, "Epic"):
						if partner.Rate30M > 0 {
							baseWholesaleFee = partner.Rate30M
						} else {
							baseWholesaleFee = partner.Rate50M
						}
					case strings.Contains(planName, "20M") || strings.Contains(planName, "Diamond"):
						if partner.Rate20M > 0 {
							baseWholesaleFee = partner.Rate20M
						} else {
							baseWholesaleFee = partner.Rate50M
						}
					default:
						baseWholesaleFee = partner.Rate50M
					}
				} else {
					baseWholesaleFee = 50000
				}

				var suspTime *time.Time
				if suspAt.Valid {
					suspTime = &suspAt.Time
				}

				// Hitung Prorata HPP Sewa Port JARTAPLOK
				jartaplokFee, _, _, _ := calculateProrata(baseWholesaleFee, actTime, status, suspTime, now)
				if partner != nil && partner.SuspensionPolicy == domain.SuspensionPolicyAllowedFullBilling && status == "SUSPENDED" {
					jartaplokFee, _, _, _ = calculateProrata(baseWholesaleFee, actTime, "ACTIVE", nil, now)
				}

				totalMRR += planPrice
				totalJartaplokCost += jartaplokFee
			}
		}
		overview.EstimatedMRR = totalMRR
		overview.AnnualRunRate = totalMRR * 12
		if overview.TotalActiveCustomers > 0 {
			overview.ARPU = totalMRR / int64(overview.TotalActiveCustomers)
		} else {
			overview.ARPU = 0
		}
		overview.TotalJartaplokCost = totalJartaplokCost
		overview.NetGrossMargin = totalMRR - totalJartaplokCost
		if totalMRR > 0 {
			overview.JartaplokMarginPct = (float64(overview.NetGrossMargin) / float64(totalMRR)) * 100.0
			overview.CostOfGoodsSoldPct = (float64(totalJartaplokCost) / float64(totalMRR)) * 100.0
		}
	}

	// 3. Sales Commission total
	var activePartnerRegs int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'ACTIVE' AND partner_code IS NOT NULL AND partner_code != ''`).Scan(&activePartnerRegs)
	overview.TotalSalesCommission = int64(activePartnerRegs) * 50000
	overview.NetContributionMargin = overview.NetGrossMargin - overview.TotalSalesCommission

	// 4. ODP metrics
	_ = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(total_ports), 0), COALESCE(SUM(used_ports), 0)
		FROM odp_nodes
	`).Scan(&overview.TotalODPs, &overview.TotalODPPorts, &overview.UsedODPPorts)
	overview.AvailableODPPorts = overview.TotalODPPorts - overview.UsedODPPorts
	targetARPU := overview.ARPU
	if targetARPU <= 0 {
		targetARPU = 250000
	}
	overview.PotentialHeadroomMRR = int64(overview.AvailableODPPorts) * targetARPU

	// 5. Work Orders
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_orders WHERE status = 'ASSIGNED'`).Scan(&overview.PendingWorkOrders)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_orders WHERE status = 'COMPLETED'`).Scan(&overview.CompletedWorkOrders)

	return overview, nil
}

// BatchUpsertODPs mengimpor/memperbarui titik ODP dari file KML ke database
func (s *SQLiteStorage) BatchUpsertODPs(ctx context.Context, providerID, providerName string, odps []domain.ODPNode) (*domain.KMLUploadResult, error) {
	if providerID == "" {
		providerID = "GNET-BIARO"
	}
	if providerName == "" {
		providerName = "PT. GNET BIARO AKSES"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback()

	checkStmt, err := tx.PrepareContext(ctx, `SELECT id FROM odp_nodes WHERE code = ?`)
	if err != nil {
		return nil, err
	}
	defer checkStmt.Close()

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO odp_nodes (id, code, name, latitude, longitude, total_ports, used_ports, status, cluster_area, provider_id, provider_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, 'AVAILABLE', ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return nil, err
	}
	defer insertStmt.Close()

	updateStmt, err := tx.PrepareContext(ctx, `
		UPDATE odp_nodes
		SET name = ?, latitude = ?, longitude = ?, cluster_area = ?, provider_id = ?, provider_name = ?, updated_at = CURRENT_TIMESTAMP
		WHERE code = ?
	`)
	if err != nil {
		return nil, err
	}
	defer updateStmt.Close()

	result := &domain.KMLUploadResult{
		TotalPlacemarks: len(odps),
		ProviderID:      providerID,
		ProviderName:    providerName,
	}

	for _, o := range odps {
		if o.Code == "" {
			result.Skipped++
			continue
		}

		var existingID string
		err := checkStmt.QueryRowContext(ctx, o.Code).Scan(&existingID)
		if err == sql.ErrNoRows {
			// Insert new ODP
			totalPorts := o.TotalPorts
			if totalPorts <= 0 {
				totalPorts = 8
			}
			newID := uuid.NewString()
			_, err = insertStmt.ExecContext(ctx, newID, o.Code, o.Name, o.Latitude, o.Longitude, totalPorts, o.ClusterArea, providerID, providerName)
			if err != nil {
				result.Skipped++
				continue
			}
			result.Inserted++
		} else if err == nil {
			// Update existing ODP without overwriting used_ports
			_, err = updateStmt.ExecContext(ctx, o.Name, o.Latitude, o.Longitude, o.ClusterArea, providerID, providerName, o.Code)
			if err != nil {
				result.Skipped++
				continue
			}
			result.Updated++
		} else {
			result.Skipped++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit tx: %w", err)
	}

	return result, nil
}

// ListJartaplokPartners mengembalikan seluruh daftar rekanan JARTAPLOK / Bitstream beserta utilisasi ODP
func (s *SQLiteStorage) ListJartaplokPartners(ctx context.Context, branchCode string) ([]domain.JartaplokPartner, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.code, p.name, p.api_key, p.contact_phone, p.coverage_area,
		       COALESCE(p.service_type, 'SEWA_PORT_FO'), COALESCE(p.suspension_policy, 'ALLOWED_WITH_WAIVER'), COALESCE(p.pricing_model, 'Standard'),
		       COALESCE(p.rate_20m, 0), COALESCE(p.rate_30m, 0), COALESCE(p.rate_40m, 0),
		       COALESCE(p.rate_50m, 50000), COALESCE(p.rate_100m, 90000), COALESCE(p.rate_150m, 135000),
		       COALESCE(p.rate_200m, 180000), COALESCE(p.rate_300m, 270000),
		       COALESCE(p.otc_fee, 0), COALESCE(p.max_distance_meters, 250.0),
		       p.is_active, p.created_at, p.updated_at,
		       COALESCE((SELECT COUNT(*) FROM odp_nodes o WHERE o.provider_id = p.code), 0) as total_odps,
		       COALESCE((SELECT SUM(o.total_ports) FROM odp_nodes o WHERE o.provider_id = p.code), 0) as total_ports,
		       COALESCE((SELECT COUNT(*) FROM registrations r JOIN odp_nodes o ON r.nearest_odp_code = o.code WHERE o.provider_id = p.code AND r.status = 'ACTIVE'), 0) as active_ports
		FROM jartaplok_partners p
		ORDER BY p.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	partners := make([]domain.JartaplokPartner, 0)
	for rows.Next() {
		var p domain.JartaplokPartner
		var isActiveInt int
		if err := rows.Scan(
			&p.ID, &p.Code, &p.Name, &p.APIKey, &p.ContactPhone, &p.CoverageArea,
			&p.ServiceType, &p.SuspensionPolicy, &p.PricingModel,
			&p.Rate20M, &p.Rate30M, &p.Rate40M,
			&p.Rate50M, &p.Rate100M, &p.Rate150M,
			&p.Rate200M, &p.Rate300M,
			&p.OTCFee, &p.MaxDistanceMeters,
			&isActiveInt, &p.CreatedAt, &p.UpdatedAt,
			&p.TotalODPs, &p.TotalPorts, &p.ActivePorts,
		); err != nil {
			return nil, err
		}
		p.IsActive = isActiveInt == 1
		partners = append(partners, p)
	}
	return partners, nil
}

func (s *SQLiteStorage) GetJartaplokPartnerByAPIKey(ctx context.Context, apiKey string) (*domain.JartaplokPartner, error) {
	var p domain.JartaplokPartner
	var isActiveInt int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, api_key, contact_phone, coverage_area,
		       COALESCE(service_type, 'SEWA_PORT_FO'), COALESCE(suspension_policy, 'ALLOWED_WITH_WAIVER'), COALESCE(pricing_model, 'Standard'),
		       COALESCE(rate_20m, 0), COALESCE(rate_30m, 0), COALESCE(rate_40m, 0),
		       COALESCE(rate_50m, 50000), COALESCE(rate_100m, 90000), COALESCE(rate_150m, 135000),
		       COALESCE(rate_200m, 180000), COALESCE(rate_300m, 270000),
		       COALESCE(otc_fee, 0), COALESCE(max_distance_meters, 250.0),
		       is_active, created_at, updated_at
		FROM jartaplok_partners
		WHERE api_key = ? AND is_active = 1
	`, apiKey).Scan(
		&p.ID, &p.Code, &p.Name, &p.APIKey, &p.ContactPhone, &p.CoverageArea,
		&p.ServiceType, &p.SuspensionPolicy, &p.PricingModel,
		&p.Rate20M, &p.Rate30M, &p.Rate40M,
		&p.Rate50M, &p.Rate100M, &p.Rate150M,
		&p.Rate200M, &p.Rate300M,
		&p.OTCFee, &p.MaxDistanceMeters,
		&isActiveInt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.IsActive = isActiveInt == 1
	return &p, nil
}

func (s *SQLiteStorage) GetJartaplokPartnerByCode(ctx context.Context, code string) (*domain.JartaplokPartner, error) {
	var p domain.JartaplokPartner
	var isActiveInt int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, api_key, contact_phone, coverage_area,
		       COALESCE(service_type, 'SEWA_PORT_FO'), COALESCE(suspension_policy, 'ALLOWED_WITH_WAIVER'), COALESCE(pricing_model, 'Standard'),
		       COALESCE(rate_20m, 0), COALESCE(rate_30m, 0), COALESCE(rate_40m, 0),
		       COALESCE(rate_50m, 50000), COALESCE(rate_100m, 90000), COALESCE(rate_150m, 135000),
		       COALESCE(rate_200m, 180000), COALESCE(rate_300m, 270000),
		       COALESCE(otc_fee, 0), COALESCE(max_distance_meters, 250.0),
		       is_active, created_at, updated_at
		FROM jartaplok_partners
		WHERE code = ? OR id = ?
	`, code, code).Scan(
		&p.ID, &p.Code, &p.Name, &p.APIKey, &p.ContactPhone, &p.CoverageArea,
		&p.ServiceType, &p.SuspensionPolicy, &p.PricingModel,
		&p.Rate20M, &p.Rate30M, &p.Rate40M,
		&p.Rate50M, &p.Rate100M, &p.Rate150M,
		&p.Rate200M, &p.Rate300M,
		&p.OTCFee, &p.MaxDistanceMeters,
		&isActiveInt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.IsActive = isActiveInt == 1
	return &p, nil
}

func (s *SQLiteStorage) CreateJartaplokPartner(ctx context.Context, p *domain.JartaplokPartner) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.ServiceType == "" {
		p.ServiceType = "SEWA_PORT_FO"
	}
	if p.SuspensionPolicy == "" {
		if p.ServiceType == "BITSTREAM" {
			p.SuspensionPolicy = domain.SuspensionPolicyDisallowed
		} else {
			p.SuspensionPolicy = domain.SuspensionPolicyAllowedWithWaiver
		}
	}
	if p.PricingModel == "" {
		if p.ServiceType == "BITSTREAM" {
			p.PricingModel = "Bitstream Intra Standard Symetric 1:1 (Zona-2 Sumatera)"
		} else {
			p.PricingModel = "Sewa Port FO Pasif (Jartaplok)"
		}
	}
	if p.MaxDistanceMeters <= 0 {
		if p.ServiceType == "BITSTREAM" {
			p.MaxDistanceMeters = 150.0
		} else {
			p.MaxDistanceMeters = 250.0
		}
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jartaplok_partners (
			id, code, name, api_key, contact_phone, coverage_area,
			service_type, suspension_policy, pricing_model, rate_20m, rate_30m, rate_40m,
			rate_50m, rate_100m, rate_150m, rate_200m, rate_300m,
			otc_fee, max_distance_meters, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
	`, p.ID, p.Code, p.Name, p.APIKey, p.ContactPhone, p.CoverageArea,
		p.ServiceType, p.SuspensionPolicy, p.PricingModel, p.Rate20M, p.Rate30M, p.Rate40M,
		p.Rate50M, p.Rate100M, p.Rate150M, p.Rate200M, p.Rate300M,
		p.OTCFee, p.MaxDistanceMeters, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *SQLiteStorage) UpdateJartaplokPartner(ctx context.Context, p *domain.JartaplokPartner) error {
	isActiveInt := 0
	if p.IsActive {
		isActiveInt = 1
	}
	if p.ServiceType == "" {
		p.ServiceType = "SEWA_PORT_FO"
	}
	if p.SuspensionPolicy == "" {
		p.SuspensionPolicy = domain.SuspensionPolicyAllowedWithWaiver
	}
	if p.MaxDistanceMeters <= 0 {
		if p.ServiceType == "BITSTREAM" {
			p.MaxDistanceMeters = 150.0
		} else {
			p.MaxDistanceMeters = 250.0
		}
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE jartaplok_partners
		SET name = ?, contact_phone = ?, coverage_area = ?,
		    service_type = ?, suspension_policy = ?, pricing_model = ?,
		    rate_20m = ?, rate_30m = ?, rate_40m = ?,
		    rate_50m = ?, rate_100m = ?, rate_150m = ?, rate_200m = ?, rate_300m = ?,
		    otc_fee = ?, max_distance_meters = ?,
		    is_active = ?,
		    api_key = COALESCE(NULLIF(?, ''), api_key),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? OR code = ?
	`, p.Name, p.ContactPhone, p.CoverageArea,
		p.ServiceType, p.SuspensionPolicy, p.PricingModel,
		p.Rate20M, p.Rate30M, p.Rate40M,
		p.Rate50M, p.Rate100M, p.Rate150M, p.Rate200M, p.Rate300M,
		p.OTCFee, p.MaxDistanceMeters,
		isActiveInt, p.APIKey, p.ID, p.Code,
	)
	if err != nil {
		return err
	}

	// Sinkronkan nama rekanan ke seluruh titik ODP terkait
	_, _ = s.db.ExecContext(ctx, `UPDATE odp_nodes SET provider_name = ? WHERE provider_id = ? OR provider_id = (SELECT code FROM jartaplok_partners WHERE id = ?)`, p.Name, p.Code, p.ID)
	return nil
}

// calculateProrata evaluates billable days and prorated rental for a circuit in a given month/year
func calculateProrata(baseRate int64, actTime time.Time, status string, suspTime *time.Time, refTime time.Time) (fee int64, activeDays int, totalDays int, isProrated bool) {
	// Total days in the reference month
	firstOfNextMonth := time.Date(refTime.Year(), refTime.Month()+1, 1, 0, 0, 0, 0, refTime.Location())
	totalDays = firstOfNextMonth.Add(-24 * time.Hour).Day()

	refYear, refMonth := refTime.Year(), refTime.Month()

	// Default start day in month
	startDay := 1
	if actTime.Year() == refYear && actTime.Month() == refMonth {
		startDay = actTime.Day()
		isProrated = true
	} else if actTime.After(refTime) {
		// Activated in future relative to refTime
		return 0, 0, totalDays, true
	}

	// Default end day in month
	endDay := totalDays
	if status == "SUSPENDED" && suspTime != nil {
		if suspTime.Year() == refYear && suspTime.Month() == refMonth {
			// Suspended during current month
			endDay = suspTime.Day() - 1
			if endDay < 0 {
				endDay = 0
			}
			isProrated = true
		} else if suspTime.Before(time.Date(refYear, refMonth, 1, 0, 0, 0, 0, refTime.Location())) {
			// Suspended prior to this month: 0 active days
			return 0, 0, totalDays, true
		}
	} else if status == "SUSPENDED" && suspTime == nil {
		// Suspended without specific date: 0 active days
		return 0, 0, totalDays, true
	}

	if endDay < startDay {
		activeDays = 0
	} else {
		activeDays = endDay - startDay + 1
	}

	if activeDays >= totalDays {
		activeDays = totalDays
		if !isProrated {
			return baseRate, totalDays, totalDays, false
		}
	}

	if activeDays <= 0 {
		return 0, 0, totalDays, true
	}

	// Prorata fee: round((baseRate * activeDays) / totalDays)
	fee = (baseRate*int64(activeDays) + int64(totalDays/2)) / int64(totalDays)
	return fee, activeDays, totalDays, true
}

func getPlanSpeedAndRate(planName string, r20, r30, r40, r50, r100, r150, r200, r300 int64) (int, string, int64) {
	switch {
	case strings.Contains(planName, "300M"):
		return 300, "300 Mbps", r300
	case strings.Contains(planName, "200M"):
		return 200, "200 Mbps", r200
	case strings.Contains(planName, "150M"):
		return 150, "150 Mbps", r150
	case strings.Contains(planName, "100M") || strings.Contains(planName, "Glory"):
		return 100, "100 Mbps", r100
	case strings.Contains(planName, "50M") || strings.Contains(planName, "Honor") || strings.Contains(planName, "FAST"):
		return 50, "50 Mbps", r50
	case strings.Contains(planName, "40M") || strings.Contains(planName, "Legend"):
		return 40, "40 Mbps", r40
	case strings.Contains(planName, "30M") || strings.Contains(planName, "Epic"):
		return 30, "30 Mbps", r30
	case strings.Contains(planName, "20M") || strings.Contains(planName, "Diamond") || strings.Contains(planName, "Starter"):
		return 20, "20 Mbps", r20
	default:
		return 50, "50 Mbps", r50
	}
}

// GetJartaplokBillingSummary menghasilkan rekap tagihan sewa port wholesale (default ke PT. GNET BIARO AKSES)
func (s *SQLiteStorage) GetJartaplokBillingSummary(ctx context.Context) (*domain.JartaplokBillingSummary, error) {
	return s.GetJartaplokBillingSummaryForPartner(ctx, "GNET-BIARO")
}

// GetJartaplokBillingSummaryForPartner menghasilkan rekap tagihan sewa port wholesale spesifik per rekanan JARTAPLOK / Bitstream
// Menerapkan tarif prorata di bulan pertama aktivasi serta pembebasan sewa port pada periode jeda layanan
func (s *SQLiteStorage) GetJartaplokBillingSummaryForPartner(ctx context.Context, partnerCode string) (*domain.JartaplokBillingSummary, error) {
	if partnerCode == "" {
		partnerCode = "GNET-BIARO"
	}

	partner, err := s.GetJartaplokPartnerByCode(ctx, partnerCode)
	if err != nil {
		partner = &domain.JartaplokPartner{
			Code:              partnerCode,
			Name:              "PT. GNET BIARO AKSES (Penyelenggara JARTAPLOK)",
			ServiceType:       "SEWA_PORT_FO",
			Rate50M:           50000,
			Rate100M:          90000,
			Rate150M:          135000,
			Rate200M:          180000,
			Rate300M:          270000,
			MaxDistanceMeters: 250.0,
		}
	}

	now := time.Now()
	summary := &domain.JartaplokBillingSummary{
		PartnerName:       partner.Name,
		ServiceType:       partner.ServiceType,
		PricingModel:      partner.PricingModel,
		OTCFee:            partner.OTCFee,
		MaxDistanceMeters: partner.MaxDistanceMeters,
		PeriodMonth:       now.Format("January 2006"),
	}

	r20 := int64(154000)
	r30 := int64(169000)
	r40 := int64(184000)
	r50 := int64(50000)
	r100 := int64(90000)
	r150 := int64(135000)
	r200 := int64(180000)
	r300 := int64(270000)
	if partner != nil {
		if partner.Rate20M > 0 {
			r20 = partner.Rate20M
		}
		if partner.Rate30M > 0 {
			r30 = partner.Rate30M
		}
		if partner.Rate40M > 0 {
			r40 = partner.Rate40M
		}
		if partner.Rate50M > 0 {
			r50 = partner.Rate50M
		}
		if partner.Rate100M > 0 {
			r100 = partner.Rate100M
		}
		if partner.Rate150M > 0 {
			r150 = partner.Rate150M
		}
		if partner.Rate200M > 0 {
			r200 = partner.Rate200M
		}
		if partner.Rate300M > 0 {
			r300 = partner.Rate300M
		}
	}

	tierActiveCounts := make(map[int]int)
	tierSuspendedCounts := make(map[int]int)
	tierSubtotals := make(map[int]int64)
	tierFullSubtotals := make(map[int]int64)

	var totalActive int
	var totalSuspended int
	var totalBilling int64
	var totalFullMonthly int64

	// Filter seluruh port (aktif maupun jeda layanan) yang tersambung ke ODP milik partner ini
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.selected_plan_name, r.status, r.activated_at, r.created_at, r.suspended_at
		FROM registrations r
		JOIN odp_nodes o ON r.nearest_odp_code = o.code
		WHERE (r.status = 'ACTIVE' OR r.status = 'SUSPENDED') AND o.provider_id = ?
	`, partnerCode)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var planName, status string
			var actAt, createAt, suspAt sql.NullTime
			if err := rows.Scan(&planName, &status, &actAt, &createAt, &suspAt); err == nil {
				actTime := time.Now()
				if actAt.Valid {
					actTime = actAt.Time
				} else if createAt.Valid {
					actTime = createAt.Time
				}

				speedMbps, _, baseRate := getPlanSpeedAndRate(planName, r20, r30, r40, r50, r100, r150, r200, r300)

				var suspTime *time.Time
				if suspAt.Valid {
					suspTime = &suspAt.Time
				}

				fee, _, _, _ := calculateProrata(baseRate, actTime, status, suspTime, now)
				if partner != nil && partner.SuspensionPolicy == domain.SuspensionPolicyAllowedFullBilling && status == "SUSPENDED" {
					fee, _, _, _ = calculateProrata(baseRate, actTime, "ACTIVE", nil, now)
				}

				if status == "ACTIVE" {
					tierActiveCounts[speedMbps]++
					totalActive++
				} else {
					tierSuspendedCounts[speedMbps]++
					totalSuspended++
				}

				tierSubtotals[speedMbps] += fee
				tierFullSubtotals[speedMbps] += baseRate
				totalBilling += fee
				totalFullMonthly += baseRate
			}
		}
	}

	var tiersMeta []struct {
		SpeedMbps   int
		RatePerPort int64
	}

	if partner.ServiceType == "BITSTREAM" || partner.Rate20M > 0 || partner.Rate30M > 0 || partner.Rate40M > 0 {
		// Bitstream Access (Standard Symetric 1:1, Zona-2 Sumatera)
		tiersMeta = []struct {
			SpeedMbps   int
			RatePerPort int64
		}{
			{SpeedMbps: 20, RatePerPort: r20},
			{SpeedMbps: 30, RatePerPort: r30},
			{SpeedMbps: 40, RatePerPort: r40},
			{SpeedMbps: 50, RatePerPort: r50},
			{SpeedMbps: 100, RatePerPort: r100},
			{SpeedMbps: 200, RatePerPort: r200},
		}
	} else {
		// Standar Sewa Port FO (JARTAPLOK)
		tiersMeta = []struct {
			SpeedMbps   int
			RatePerPort int64
		}{
			{SpeedMbps: 50, RatePerPort: r50},
			{SpeedMbps: 100, RatePerPort: r100},
			{SpeedMbps: 150, RatePerPort: r150},
			{SpeedMbps: 200, RatePerPort: r200},
			{SpeedMbps: 300, RatePerPort: r300},
		}
	}

	var tierSummaries []domain.JartaplokTierSummary
	for _, tm := range tiersMeta {
		actCnt := tierActiveCounts[tm.SpeedMbps]
		suspCnt := tierSuspendedCounts[tm.SpeedMbps]
		actualSub := tierSubtotals[tm.SpeedMbps]
		fullSub := tierFullSubtotals[tm.SpeedMbps]

		tierSummaries = append(tierSummaries, domain.JartaplokTierSummary{
			SpeedMbps:      tm.SpeedMbps,
			RatePerPort:    tm.RatePerPort,
			ActivePorts:    actCnt,
			SuspendedPorts: suspCnt,
			FullSubtotal:   fullSub,
			Subtotal:       actualSub,
		})
	}

	summary.TotalActivePorts = totalActive
	summary.TotalSuspendedPorts = totalSuspended
	summary.TotalBillingAmount = totalBilling
	summary.TotalFullMonthly = totalFullMonthly
	if totalFullMonthly > totalBilling {
		summary.TotalSavingsProrata = totalFullMonthly - totalBilling
	}
	summary.Tiers = tierSummaries

	// ODP metrics khusus partner ini
	var totalODPs, totalPorts, usedPorts int
	_ = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(total_ports), 0), COALESCE(SUM(used_ports), 0)
		FROM odp_nodes
		WHERE provider_id = ?
	`, partnerCode).Scan(&totalODPs, &totalPorts, &usedPorts)

	summary.TotalODPs = totalODPs
	summary.TotalCapacityPorts = totalPorts
	summary.AvailablePorts = totalPorts - usedPorts
	if totalPorts > 0 {
		summary.OccupancyPct = (float64(usedPorts) / float64(totalPorts)) * 100.0
	}

	return summary, nil
}

// GetJartaplokActivePorts menghasilkan daftar sirkuit port yang aktif (default ke PT. GNET BIARO AKSES)
func (s *SQLiteStorage) GetJartaplokActivePorts(ctx context.Context) ([]domain.JartaplokActivePort, error) {
	return s.GetJartaplokActivePortsForPartner(ctx, "GNET-BIARO")
}

// GetJartaplokActivePortsForPartner menghasilkan daftar sirkuit port aktif/jeda dengan rincian hari aktif prorata
func (s *SQLiteStorage) GetJartaplokActivePortsForPartner(ctx context.Context, partnerCode string) ([]domain.JartaplokActivePort, error) {
	if partnerCode == "" {
		partnerCode = "GNET-BIARO"
	}

	partner, _ := s.GetJartaplokPartnerByCode(ctx, partnerCode)
	r20 := int64(154000)
	r30 := int64(169000)
	r40 := int64(184000)
	r50 := int64(50000)
	r100 := int64(90000)
	r150 := int64(135000)
	r200 := int64(180000)
	r300 := int64(270000)
	if partner != nil {
		if partner.Rate20M > 0 {
			r20 = partner.Rate20M
		}
		if partner.Rate30M > 0 {
			r30 = partner.Rate30M
		}
		if partner.Rate40M > 0 {
			r40 = partner.Rate40M
		}
		if partner.Rate50M > 0 {
			r50 = partner.Rate50M
		}
		if partner.Rate100M > 0 {
			r100 = partner.Rate100M
		}
		if partner.Rate150M > 0 {
			r150 = partner.Rate150M
		}
		if partner.Rate200M > 0 {
			r200 = partner.Rate200M
		}
		if partner.Rate300M > 0 {
			r300 = partner.Rate300M
		}
	}

	now := time.Now()
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.registration_no, COALESCE(r.nearest_odp_code, 'ODP-UNASSIGNED'), COALESCE(o.name, 'Tiang Distribusi FO'),
		       r.selected_plan_name, r.activated_at, r.created_at, r.status, r.suspended_at, COALESCE(r.suspension_reason, '')
		FROM registrations r
		JOIN odp_nodes o ON r.nearest_odp_code = o.code
		WHERE (r.status = 'ACTIVE' OR r.status = 'SUSPENDED') AND o.provider_id = ?
		ORDER BY CASE r.status WHEN 'ACTIVE' THEN 1 ELSE 2 END, r.updated_at DESC
	`, partnerCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ports []domain.JartaplokActivePort
	portIdx := 1

	for rows.Next() {
		var regNo, odpCode, odpName, planName, status, suspReason string
		var actAt, createAt, suspAt sql.NullTime

		if err := rows.Scan(&regNo, &odpCode, &odpName, &planName, &actAt, &createAt, &status, &suspAt, &suspReason); err != nil {
			continue
		}

		actTime := time.Now()
		if actAt.Valid {
			actTime = actAt.Time
		} else if createAt.Valid {
			actTime = createAt.Time
		}

		_, speedStr, baseRate := getPlanSpeedAndRate(planName, r20, r30, r40, r50, r100, r150, r200, r300)

		var suspTime *time.Time
		var suspStr string
		if suspAt.Valid {
			suspTime = &suspAt.Time
			suspStr = suspAt.Time.Format("02 Jan 2006")
		}

		fee, activeDays, totalDays, isProrated := calculateProrata(baseRate, actTime, status, suspTime, now)
		if partner != nil && partner.SuspensionPolicy == domain.SuspensionPolicyAllowedFullBilling && status == "SUSPENDED" {
			fee, activeDays, totalDays, isProrated = calculateProrata(baseRate, actTime, "ACTIVE", nil, now)
		}

		ports = append(ports, domain.JartaplokActivePort{
			CircuitID:        fmt.Sprintf("CKT-%s-%04d", strings.ReplaceAll(partnerCode, "-", ""), portIdx),
			ODPCode:          odpCode,
			ODPName:          odpName,
			PortNumber:       ((portIdx - 1) % 8) + 1,
			PackageSpeed:     speedStr,
			MonthlyRental:    baseRate,
			ProratedFee:      fee,
			IsProrated:       isProrated,
			ActiveDays:       activeDays,
			TotalDays:        totalDays,
			ActivatedAt:      actTime.Format("02 Jan 2006"),
			SuspendedAt:      suspStr,
			SuspensionReason: suspReason,
			Status:           status,
		})
		portIdx++
	}

	return ports, nil
}

// ── CUSTOMER AUTH (PASSWORD & OTP) ──────────────────────────

func (s *SQLiteStorage) GetCustomerPasswordHash(ctx context.Context, regID string) (string, error) {
	clean := strings.TrimSpace(regID)
	var hash string
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(customer_password_hash, '') 
		FROM registrations 
		WHERE id = ? OR registration_no = ?
		LIMIT 1
	`, clean, clean).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return hash, nil
}

func (s *SQLiteStorage) UpdateCustomerPassword(ctx context.Context, phoneOrEmailOrID string, passwordHash string) error {
	clean := strings.TrimSpace(phoneOrEmailOrID)
	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET customer_password_hash = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? 
		   OR registration_no = ?
		   OR (phone != '' AND phone = ?)
		   OR (email != '' AND LOWER(email) = LOWER(?))
	`, passwordHash, clean, clean, clean, clean)
	return err
}

func (s *SQLiteStorage) StoreCustomerOTP(ctx context.Context, phone string, otpCode string, expiresAt time.Time) error {
	cleanPhone := strings.TrimSpace(phone)
	id := fmt.Sprintf("otp-%d", time.Now().UnixNano())
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO customer_otps (id, phone, otp_code, expires_at, used, created_at)
		VALUES (?, ?, ?, ?, 0, CURRENT_TIMESTAMP)
	`, id, cleanPhone, otpCode, expiresAt)
	return err
}

func (s *SQLiteStorage) VerifyCustomerOTP(ctx context.Context, phone string, otpCode string) (bool, error) {
	cleanPhone := strings.TrimSpace(phone)
	cleanCode := strings.TrimSpace(otpCode)
	var id string
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM customer_otps 
		WHERE phone = ? AND otp_code = ? AND used = 0 AND expires_at > CURRENT_TIMESTAMP
		ORDER BY created_at DESC LIMIT 1
	`, cleanPhone, cleanCode).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE customer_otps SET used = 1 WHERE id = ?`, id)
	return true, nil
}

func (s *SQLiteStorage) GetStaffKPISummary(ctx context.Context, branchCode string) (*domain.StaffKPISummary, error) {
	now := time.Now()
	summary := &domain.StaffKPISummary{
		MonthYear:   now.Format("January 2006"),
		Technicians: []domain.TechnicianKPI{},
		Sales:       []domain.SalesKPI{},
	}

	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'ACTIVE'`).Scan(&summary.TotalActiveSubs)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_orders WHERE status = 'COMPLETED'`).Scan(&summary.TotalSPKDone)
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(AVG(optical_power_dbm), -18.5) FROM bast_reports`).Scan(&summary.AvgTeamOpticalDBM)

	type techDef struct {
		Name    string
		Role    string
		Pattern string
	}

	techList := []techDef{
		{"Tim Teknisi Payakumbuh", "Tim Bersama / Bergilir", "%payakumbuh%"},
		{"Ricci", "Teknisi Lapangan (RICCI-TECH)", "%ricci%"},
		{"Zikka Sintio Anugrah", "Teknisi Lapangan (ZIKKA-TECH)", "%zikka%"},
		{"Egi", "Teknisi Lapangan (EGI-TECH)", "%egi%"},
		{"Nando Azkia Putra S.Kom", "Koordinator & Core FO", "%nando%"},
		{"Mitra Teknisi Golden", "Mitra Lapangan (PT GNET BIARO AKSES)", "%golden%"},
	}

	// Pemeriksaan nama teknisi dinamis dari tabel work_orders
	rowsTech, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT technician_name 
		FROM work_orders 
		WHERE technician_name IS NOT NULL AND TRIM(technician_name) != ''
	`)
	if err == nil {
		defer rowsTech.Close()
		for rowsTech.Next() {
			var rawName string
			if err := rowsTech.Scan(&rawName); err == nil {
				trimmed := strings.TrimSpace(rawName)
				if trimmed == "" {
					continue
				}
				matched := false
				for _, td := range techList {
					patClean := strings.ReplaceAll(strings.ToLower(td.Pattern), "%", "")
					if strings.Contains(strings.ToLower(trimmed), patClean) {
						matched = true
						break
					}
				}
				if !matched {
					techList = append(techList, techDef{
						Name:    trimmed,
						Role:    "Teknisi Penugasan Lapangan",
						Pattern: "%" + strings.ToLower(trimmed) + "%",
					})
				}
			}
		}
	}

	for _, t := range techList {
		tkpi := domain.TechnicianKPI{
			Name: t.Name,
			Role: t.Role,
		}

		_ = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM work_orders 
			WHERE LOWER(technician_name) LIKE LOWER(?)
		`, t.Pattern).Scan(&tkpi.AssignedOrders)

		_ = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM work_orders 
			WHERE LOWER(technician_name) LIKE LOWER(?) AND status = 'COMPLETED'
		`, t.Pattern).Scan(&tkpi.CompletedOrders)

		tkpi.PendingOrders = tkpi.AssignedOrders - tkpi.CompletedOrders
		if tkpi.PendingOrders < 0 {
			tkpi.PendingOrders = 0
		}

		var avgDBM sql.NullFloat64
		var totalMeters sql.NullInt64
		_ = s.db.QueryRowContext(ctx, `
			SELECT AVG(b.optical_power_dbm), SUM(b.dropcore_length_meters)
			FROM bast_reports b
			JOIN work_orders w ON b.work_order_id = w.id
			WHERE LOWER(w.technician_name) LIKE LOWER(?)
		`, t.Pattern).Scan(&avgDBM, &totalMeters)

		if avgDBM.Valid {
			tkpi.AvgOpticalPowerDBM = avgDBM.Float64
		} else {
			tkpi.AvgOpticalPowerDBM = summary.AvgTeamOpticalDBM
		}
		if totalMeters.Valid {
			tkpi.TotalDropcoreMeters = int(totalMeters.Int64)
		}

		score := 70
		if tkpi.CompletedOrders > 0 {
			score += 15
		}
		if tkpi.AvgOpticalPowerDBM >= -23.0 && tkpi.AvgOpticalPowerDBM < -15.0 {
			score += 15
			tkpi.ComplianceRatePct = 100.0
		} else {
			tkpi.ComplianceRatePct = 85.0
		}
		if score > 100 {
			score = 100
		}
		tkpi.Score = score

		if score >= 90 {
			tkpi.Grade = "A (Sangat Baik)"
		} else if score >= 75 {
			tkpi.Grade = "B (Baik)"
		} else {
			tkpi.Grade = "C (Cukup)"
		}

		summary.Technicians = append(summary.Technicians, tkpi)
	}

	// Urutkan teknisi: CompletedOrders DESC, AssignedOrders DESC, Score DESC, Name ASC
	sort.SliceStable(summary.Technicians, func(i, j int) bool {
		if summary.Technicians[i].CompletedOrders != summary.Technicians[j].CompletedOrders {
			return summary.Technicians[i].CompletedOrders > summary.Technicians[j].CompletedOrders
		}
		if summary.Technicians[i].AssignedOrders != summary.Technicians[j].AssignedOrders {
			return summary.Technicians[i].AssignedOrders > summary.Technicians[j].AssignedOrders
		}
		return summary.Technicians[i].Score > summary.Technicians[j].Score
	})

	// Agregasi Sales dinamis dari tabel partners
	rowsSales, err := s.db.QueryContext(ctx, `
		SELECT 
			p.code,
			p.name,
			COALESCE(p.commission_rate, 50000),
			COUNT(r.id) AS total_leads,
			COUNT(CASE WHEN r.status = 'ACTIVE' THEN 1 END) AS active_customers
		FROM partners p
		LEFT JOIN registrations r ON (UPPER(TRIM(r.partner_code)) = UPPER(TRIM(p.code)))
		WHERE p.is_active = 1 AND p.code NOT LIKE 'TEST-%'
		GROUP BY p.code, p.name, p.commission_rate
		ORDER BY active_customers DESC, total_leads DESC, p.code ASC
	`)
	if err == nil {
		defer rowsSales.Close()
		for rowsSales.Next() {
			var skpi domain.SalesKPI
			var commRate float64
			if err := rowsSales.Scan(&skpi.PartnerCode, &skpi.Name, &commRate, &skpi.TotalLeads, &skpi.ActiveCustomers); err == nil {
				if skpi.TotalLeads > 0 {
					skpi.ConversionRatePct = (float64(skpi.ActiveCustomers) / float64(skpi.TotalLeads)) * 100.0
				}
				if commRate <= 0 {
					commRate = 50000
				}
				skpi.TotalCommission = int64(skpi.ActiveCustomers) * int64(commRate)

				score := 70
				if skpi.TotalLeads > 0 {
					score += 10
				}
				if skpi.ActiveCustomers > 0 {
					score += 15
				}
				if skpi.ConversionRatePct >= 50.0 {
					score += 5
				}
				if score > 100 {
					score = 100
				}
				skpi.Score = score

				if score >= 90 {
					skpi.Grade = "A (Sangat Baik)"
				} else if score >= 75 {
					skpi.Grade = "B (Baik)"
				} else {
					skpi.Grade = "C (Cukup)"
				}

				summary.Sales = append(summary.Sales, skpi)
			}
		}
	}

	return summary, nil
}

// ── BRANCH OPERATIONS ──────────────────────────────────────────

func (s *SQLiteStorage) ListBranches(ctx context.Context) ([]domain.Branch, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, code, name, type, COALESCE(city, ''), COALESCE(province, ''),
		       COALESCE(revenue_share_type, 'PERCENTAGE'), COALESCE(share_percent_partner, 70.0),
		       COALESCE(flat_fee_per_sub, 35000), is_active, created_at, updated_at
		FROM branches
		ORDER BY CASE WHEN code = 'PYK' THEN 0 ELSE 1 END, name ASC
	`)
	if err != nil {
		// Fallback static branches if table does not exist in SQLite
		return []domain.Branch{
			{ID: "e049ceae-b219-451e-bbce-fc78301ccc8f", Code: "PYK", Name: "Kantor Cabang Payakumbuh & 50 Kota", Type: "OWNED_BRANCH", City: "Payakumbuh", Province: "Sumatera Barat", IsActive: true},
			{ID: "bf052084-11de-4d66-9b39-34179ba00764", Code: "GNET-BIARO", Name: "PT GNET BIARO AKSES", Type: "RESELLER", City: "Agam / Bukittinggi", Province: "Sumatera Barat", IsActive: true},
			{ID: "8d6ddeff-f639-4dfb-828e-b72a144b458b", Code: "PAPUA", Name: "Kantor Cabang Papua", Type: "OWNED_BRANCH", City: "Jayapura", Province: "Papua", IsActive: true},
		}, nil
	}
	defer rows.Close()

	var branches []domain.Branch
	for rows.Next() {
		var b domain.Branch
		if err := rows.Scan(
			&b.ID, &b.Code, &b.Name, &b.Type, &b.City, &b.Province,
			&b.RevenueShareType, &b.SharePercentPartner, &b.FlatFeePerSub,
			&b.IsActive, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan branch: %w", err)
		}
		branches = append(branches, b)
	}
	return branches, nil
}

func (s *SQLiteStorage) GetBranchByID(ctx context.Context, id string) (*domain.Branch, error) {
	branches, _ := s.ListBranches(ctx)
	for _, b := range branches {
		if b.ID == id {
			return &b, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *SQLiteStorage) GetBranchByCode(ctx context.Context, code string) (*domain.Branch, error) {
	branches, _ := s.ListBranches(ctx)
	for _, b := range branches {
		if strings.EqualFold(b.Code, strings.TrimSpace(code)) {
			return &b, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *SQLiteStorage) ListClusterSmartOLTConfigs(ctx context.Context) ([]domain.ClusterSmartOLTConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, cluster_name, provider_id, integration_type, smartolt_url, smartolt_api_key, olt_id, zone_id, zone_name, is_active, created_at, updated_at
		FROM cluster_smartolt_configs
		ORDER BY cluster_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.ClusterSmartOLTConfig
	for rows.Next() {
		var c domain.ClusterSmartOLTConfig
		var isActiveInt int
		if err := rows.Scan(&c.ID, &c.ClusterName, &c.ProviderID, &c.IntegrationType, &c.SmartOLTURL, &c.SmartOLTKey, &c.OLTID, &c.ZoneID, &c.ZoneName, &isActiveInt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.IsActive = isActiveInt == 1
		list = append(list, c)
	}
	return list, nil
}

func (s *SQLiteStorage) GetClusterSmartOLTConfig(ctx context.Context, clusterName string) (*domain.ClusterSmartOLTConfig, error) {
	var c domain.ClusterSmartOLTConfig
	var isActiveInt int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, cluster_name, provider_id, integration_type, smartolt_url, smartolt_api_key, olt_id, zone_id, zone_name, is_active, created_at, updated_at
		FROM cluster_smartolt_configs
		WHERE LOWER(cluster_name) = LOWER(?)
		LIMIT 1
	`, strings.TrimSpace(clusterName)).Scan(
		&c.ID, &c.ClusterName, &c.ProviderID, &c.IntegrationType, &c.SmartOLTURL, &c.SmartOLTKey, &c.OLTID, &c.ZoneID, &c.ZoneName, &isActiveInt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.IsActive = isActiveInt == 1
	return &c, nil
}

func (s *SQLiteStorage) SaveClusterSmartOLTConfig(ctx context.Context, cfg *domain.ClusterSmartOLTConfig) error {
	if cfg.ID == "" {
		cfg.ID = uuid.NewString()
	}
	activeInt := 0
	if cfg.IsActive {
		activeInt = 1
	}
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cluster_smartolt_configs (id, cluster_name, provider_id, integration_type, smartolt_url, smartolt_api_key, olt_id, zone_id, zone_name, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cluster_name) DO UPDATE SET
			provider_id = excluded.provider_id,
			integration_type = excluded.integration_type,
			smartolt_url = excluded.smartolt_url,
			smartolt_api_key = excluded.smartolt_api_key,
			olt_id = excluded.olt_id,
			zone_id = excluded.zone_id,
			zone_name = excluded.zone_name,
			is_active = excluded.is_active,
			updated_at = excluded.updated_at
	`, cfg.ID, strings.TrimSpace(cfg.ClusterName), strings.TrimSpace(cfg.ProviderID), cfg.IntegrationType,
		strings.TrimSpace(cfg.SmartOLTURL), strings.TrimSpace(cfg.SmartOLTKey), strings.TrimSpace(cfg.OLTID),
		strings.TrimSpace(cfg.ZoneID), strings.TrimSpace(cfg.ZoneName), activeInt, now, now)
	return err
}

func (s *SQLiteStorage) DeleteClusterSmartOLTConfig(ctx context.Context, clusterName string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cluster_smartolt_configs WHERE LOWER(cluster_name) = LOWER(?)`, strings.TrimSpace(clusterName))
	return err
}

