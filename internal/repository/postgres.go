package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"ispsync/internal/domain"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(connStr string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	s := &PostgresStorage{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("postgres migration error: %w", err)
	}

	if err := s.seedDefaultTenant(); err != nil {
		fmt.Printf("[PostgresStorage] Seed note: %v\n", err)
	}

	return s, nil
}

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

func (s *PostgresStorage) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tenants (
		id TEXT PRIMARY KEY,
		slug TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		short_name TEXT NOT NULL,
		prefix_id TEXT NOT NULL,
		logo_url TEXT DEFAULT '',
		brand_color TEXT DEFAULT '#2563eb',
		contact_phone TEXT DEFAULT '',
		contact_email TEXT DEFAULT '',
		address TEXT DEFAULT '',
		custom_domain TEXT DEFAULT '',
		status TEXT DEFAULT 'ACTIVE',
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		username TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		full_name TEXT NOT NULL,
		email TEXT DEFAULT '',
		phone TEXT DEFAULT '',
		role TEXT NOT NULL,
		status TEXT DEFAULT 'ACTIVE',
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tenant_id, username)
	);

	CREATE TABLE IF NOT EXISTS plans (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		speed_down_mbps INTEGER DEFAULT 20,
		speed_up_mbps INTEGER DEFAULT 20,
		monthly_price DOUBLE PRECISION DEFAULT 0,
		description TEXT DEFAULT '',
		is_active INTEGER DEFAULT 1,
		UNIQUE(tenant_id, code)
	);

	CREATE TABLE IF NOT EXISTS mikrotik_routers (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		wg_pubkey TEXT NOT NULL,
		wg_ip TEXT NOT NULL UNIQUE,
		api_port INTEGER DEFAULT 8728,
		api_user TEXT NOT NULL,
		api_password TEXT NOT NULL,
		status TEXT DEFAULT 'offline',
		last_seen TIMESTAMP,
		created_at TIMESTAMP,
		updated_at TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS odps (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		latitude DOUBLE PRECISION NOT NULL,
		longitude DOUBLE PRECISION NOT NULL,
		total_ports INTEGER DEFAULT 8,
		used_ports INTEGER DEFAULT 0,
		status TEXT DEFAULT 'ACTIVE',
		UNIQUE(tenant_id, code)
	);

	CREATE TABLE IF NOT EXISTS olts (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		vendor TEXT NOT NULL,
		host_ip TEXT NOT NULL,
		port INTEGER DEFAULT 23,
		snmp_port INTEGER DEFAULT 161,
		snmp_community TEXT DEFAULT 'public',
		router_id TEXT REFERENCES mikrotik_routers(id) ON DELETE SET NULL,
		username TEXT DEFAULT 'admin',
		password TEXT DEFAULT 'admin',
		status TEXT DEFAULT 'ONLINE',
		total_pons INTEGER DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS subscribers (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		subscriber_no TEXT NOT NULL,
		full_name TEXT NOT NULL,
		identity_number TEXT DEFAULT '',
		email TEXT DEFAULT '',
		phone TEXT DEFAULT '',
		address TEXT DEFAULT '',
		latitude DOUBLE PRECISION DEFAULT 0,
		longitude DOUBLE PRECISION DEFAULT 0,
		distance_to_odp DOUBLE PRECISION DEFAULT 0,
		selected_plan_id TEXT,
		selected_plan_name TEXT,
		nearest_odp_id TEXT,
		nearest_odp_code TEXT,
		olt_id TEXT,
		pon_port TEXT,
		onu_id INTEGER,
		serial_number TEXT,
		mac_address TEXT,
		rx_optical_power DOUBLE PRECISION,
		pppoe_username TEXT,
		pppoe_password TEXT,
		vlan_id INTEGER,
		ip_address TEXT,
		status TEXT DEFAULT 'REGISTERED',
		activated_at TIMESTAMPTZ,
		suspended_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tenant_id, subscriber_no)
	);

	CREATE TABLE IF NOT EXISTS work_orders (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		order_no TEXT NOT NULL,
		subscriber_id TEXT NOT NULL REFERENCES subscribers(id) ON DELETE CASCADE,
		subscriber_no TEXT NOT NULL,
		customer_name TEXT NOT NULL,
		customer_phone TEXT DEFAULT '',
		customer_address TEXT DEFAULT '',
		customer_lat DOUBLE PRECISION DEFAULT 0,
		customer_lng DOUBLE PRECISION DEFAULT 0,
		odp_code TEXT DEFAULT '',
		plan_name TEXT DEFAULT '',
		order_type TEXT DEFAULT 'INSTALLATION',
		technician_id TEXT,
		technician_name TEXT DEFAULT '',
		status TEXT DEFAULT 'PENDING',
		rx_power_dbm DOUBLE PRECISION,
		serial_number TEXT DEFAULT '',
		mac_address TEXT DEFAULT '',
		notes TEXT DEFAULT '',
		bast_completed_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tenant_id, order_no)
	);

	CREATE TABLE IF NOT EXISTS invoices (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		invoice_no TEXT NOT NULL,
		subscriber_id TEXT NOT NULL,
		subscriber_no TEXT NOT NULL,
		customer_name TEXT NOT NULL,
		plan_name TEXT NOT NULL,
		amount DOUBLE PRECISION NOT NULL,
		status TEXT DEFAULT 'UNPAID',
		due_date TIMESTAMPTZ NOT NULL,
		paid_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tenant_id, invoice_no)
	);

	CREATE TABLE IF NOT EXISTS voucher_batches (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		batch_no TEXT NOT NULL,
		profile_name TEXT NOT NULL,
		speed_down_mbps INTEGER DEFAULT 10,
		speed_up_mbps INTEGER DEFAULT 10,
		price DOUBLE PRECISION DEFAULT 5000,
		total_vouchers INTEGER DEFAULT 50,
		used_vouchers INTEGER DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tenant_id, batch_no)
	);

	CREATE TABLE IF NOT EXISTS vouchers (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		batch_id TEXT NOT NULL REFERENCES voucher_batches(id) ON DELETE CASCADE,
		code TEXT NOT NULL,
		password TEXT NOT NULL,
		price DOUBLE PRECISION DEFAULT 5000,
		profile_name TEXT NOT NULL,
		status TEXT DEFAULT 'AVAILABLE',
		used_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tenant_id, code)
	);

	CREATE TABLE IF NOT EXISTS tenant_capabilities (
		tenant_id TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
		uses_fibergrid BOOLEAN NOT NULL DEFAULT FALSE,
		own_infrastructure BOOLEAN NOT NULL DEFAULT FALSE,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS jartaplok_agreements (
		id TEXT PRIMARY KEY,
		agreement_no TEXT NOT NULL UNIQUE,
		provider_tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		client_tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		scope_area TEXT NOT NULL,
		total_shared_odps INTEGER DEFAULT 0,
		allocated_ports INTEGER DEFAULT 0,
		used_ports INTEGER DEFAULT 0,
		settlement_rate_per_port DOUBLE PRECISION DEFAULT 25000,
		status TEXT DEFAULT 'ACTIVE',
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS jartaplok_shared_odps (
		id TEXT PRIMARY KEY,
		agreement_id TEXT NOT NULL REFERENCES jartaplok_agreements(id) ON DELETE CASCADE,
		odp_id TEXT NOT NULL REFERENCES odps(id) ON DELETE CASCADE,
		allocated_ports INTEGER DEFAULT 8,
		used_ports INTEGER DEFAULT 0,
		UNIQUE(agreement_id, odp_id)
	);

	CREATE INDEX IF NOT EXISTS idx_subscribers_tenant ON subscribers(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_odps_tenant ON odps(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_workorders_tenant ON work_orders(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_invoices_tenant ON invoices(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_vouchers_tenant ON vouchers(tenant_id);

	CREATE TABLE IF NOT EXISTS tenant_addons (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		addon_code TEXT NOT NULL,
		addon_type TEXT NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		quantity INTEGER DEFAULT 5,
		monthly_price DOUBLE PRECISION DEFAULT 0,
		status TEXT DEFAULT 'ACTIVE',
		expires_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_tenant_addons_tenant ON tenant_addons(tenant_id, status);
	ALTER TABLE tenants ADD COLUMN IF NOT EXISTS base_staff_quota INTEGER DEFAULT 3;
	`
	_, err := s.db.Exec(schema)
	return err
}


func (s *PostgresStorage) seedDefaultTenant() error {
	ctx := context.Background()
	now := time.Now()
	pwHash, _ := bcrypt.GenerateFromPassword([]byte("Password@123"), bcrypt.DefaultCost)

	// ── 1. Tenant "ispku" (PT. ISP Kita Nusantara) ─────────────────────────────
	var ispkuID string
	_ = s.db.QueryRowContext(ctx, "SELECT id FROM tenants WHERE slug = 'ispku'").Scan(&ispkuID)
	if ispkuID == "" {
		ispkuID = uuid.New().String()
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO tenants (id, slug, name, short_name, prefix_id, logo_url, brand_color, contact_phone, contact_email, address, status, created_at, updated_at)
			VALUES ($1, 'ispku', 'PT. ISP Kita Nusantara', 'ISPKU', 'ISPKU', '', '#2563eb', '081288889999', 'info@ispku.ispsync.id', 'Jl. Jenderal Sudirman No. 125, Kota Pekanbaru, Riau', 'ACTIVE', $2, $3)
		`, ispkuID, now, now)
		if err != nil {
			return err
		}

		users := []struct{ u, n, r string }{
			{"owner", "Pimpinan ISP Kita", "OWNER"},
			{"noc", "Engineer NOC Core", "NOC"},
			{"sales", "Account Executive", "SALES"},
			{"teknisi", "Teknisi Lapangan", "TECHNICIAN"},
		}
		for _, u := range users {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO users (id, tenant_id, username, password_hash, full_name, email, phone, role, status, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'ACTIVE', $9)
				ON CONFLICT (tenant_id, username) DO NOTHING
			`, uuid.New().String(), ispkuID, u.u, string(pwHash), u.n, u.u+"@ispku.ispsync.id", "081234567890", u.r, now)
		}

		plans := []struct {
			c, n string
			d, u int
			p    float64
		}{
			{"HOME-20", "Paket Home 20 Mbps", 20, 20, 175000},
			{"HOME-50", "Paket Gamer 50 Mbps", 50, 50, 275000},
			{"BIZ-100", "Paket Kantor 100 Mbps", 100, 100, 550000},
		}
		for _, pl := range plans {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO plans (id, tenant_id, code, name, speed_down_mbps, speed_up_mbps, monthly_price, description, is_active)
				VALUES ($1, $2, $3, $4, $5, $6, $7, 'Paket internet fiber optik unlimited tanpa FUP', 1)
				ON CONFLICT (tenant_id, code) DO NOTHING
			`, uuid.New().String(), ispkuID, pl.c, pl.n, pl.d, pl.u, pl.p)
		}

		odps := []struct {
			c, n     string
			lat, lng float64
			tp, up   int
		}{
			{"ODP-PDG-001", "ODP Simpang Haru 01", -0.9423, 100.3752, 8, 4},
			{"ODP-PDG-002", "ODP Jati Baru 02", -0.9385, 100.3698, 8, 7},
			{"ODP-PDG-003", "ODP Khatib Sulaiman 03", -0.9254, 100.3601, 8, 2},
			{"ODP-PDG-004", "ODP Ulak Karang 04", -0.9125, 100.3542, 8, 8},
		}
		for _, o := range odps {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO odps (id, tenant_id, code, name, latitude, longitude, total_ports, used_ports, status)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'ACTIVE')
				ON CONFLICT (tenant_id, code) DO NOTHING
			`, uuid.New().String(), ispkuID, o.c, o.n, o.lat, o.lng, o.tp, o.up)
		}

		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO olts (id, tenant_id, name, vendor, host_ip, port, username, password, status, total_pons)
			VALUES ($1, $2, 'OLT-ZTE-CORE-PDG', 'ZTE C320', '10.10.10.2', 23, 'admin', 'admin', 'ONLINE', 8)
		`, uuid.New().String(), ispkuID)
	}

	// ── 2. Tenant "ispmu" (PT. Mitra Usaha Data) ──────────────────────────────
	var ispmuID string
	_ = s.db.QueryRowContext(ctx, "SELECT id FROM tenants WHERE slug = 'ispmu'").Scan(&ispmuID)
	if ispmuID == "" {
		ispmuID = uuid.New().String()
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO tenants (id, slug, name, short_name, prefix_id, logo_url, brand_color, contact_phone, contact_email, address, status, created_at, updated_at)
			VALUES ($1, 'ispmu', 'PT. Mitra Usaha Data', 'ISPMU', 'ISPMU', '', '#059669', '081377776666', 'admin@ispmu.ispsync.id', 'Jl. Khatib Sulaiman No. 45, Kota Padang, Sumatera Barat', 'ACTIVE', $2, $3)
		`, ispmuID, now, now)
		if err != nil {
			return err
		}

		users := []struct{ u, n, r string }{
			{"owner", "Direktur Mitra Usaha", "OWNER"},
			{"noc", "NOC Operator", "NOC"},
			{"sales", "Sales Wilayah", "SALES"},
			{"teknisi", "Teknisi Fiber", "TECHNICIAN"},
		}
		for _, u := range users {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO users (id, tenant_id, username, password_hash, full_name, email, phone, role, status, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'ACTIVE', $9)
				ON CONFLICT (tenant_id, username) DO NOTHING
			`, uuid.New().String(), ispmuID, u.u, string(pwHash), u.n, u.u+"@ispmu.ispsync.id", "081377776666", u.r, now)
		}

		plans := []struct {
			c, n string
			d, u int
			p    float64
		}{
			{"BKT-15", "Paket Warga 15 Mbps", 15, 15, 150000},
			{"BKT-30", "Paket Bisnis 30 Mbps", 30, 30, 250000},
		}
		for _, pl := range plans {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO plans (id, tenant_id, code, name, speed_down_mbps, speed_up_mbps, monthly_price, description, is_active)
				VALUES ($1, $2, $3, $4, $5, $6, $7, 'Fiber optik andal kualitas premium', 1)
				ON CONFLICT (tenant_id, code) DO NOTHING
			`, uuid.New().String(), ispmuID, pl.c, pl.n, pl.d, pl.u, pl.p)
		}

		odps := []struct {
			c, n     string
			lat, lng float64
			tp, up   int
		}{
			{"ODP-BKT-001", "ODP Jam Gadang 01", -0.3055, 100.3692, 8, 3},
			{"ODP-BKT-002", "ODP Panorama 02", -0.3112, 100.3645, 8, 5},
		}
		for _, o := range odps {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO odps (id, tenant_id, code, name, latitude, longitude, total_ports, used_ports, status)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'ACTIVE')
				ON CONFLICT (tenant_id, code) DO NOTHING
			`, uuid.New().String(), ispmuID, o.c, o.n, o.lat, o.lng, o.tp, o.up)
		}
	}

	// ── 3. Tenant "dev" (Laboratorium ISPSYNC Telecom) ────────────────────────
	var devID string
	_ = s.db.QueryRowContext(ctx, "SELECT id FROM tenants WHERE slug = 'dev'").Scan(&devID)
	if devID == "" {
		devID = uuid.New().String()
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO tenants (id, slug, name, short_name, prefix_id, logo_url, brand_color, contact_phone, contact_email, address, status, created_at, updated_at)
			VALUES ($1, 'dev', 'Laboratorium ISPSYNC R&D', 'DEVLAB', 'DEV', '', '#7c3aed', '081100002026', 'dev@ispsync.id', 'Gedung Cyber 1 Lt. 3, Jl. Kuningan Barat No. 8, Jakarta Selatan', 'ACTIVE', $2, $3)
		`, devID, now, now)
		if err != nil {
			return err
		}

		devPwHash, _ := bcrypt.GenerateFromPassword([]byte("DevLab2026!"), bcrypt.DefaultCost)
		users := []struct{ u, n, r string }{
			{"admin", "Lead Telecom Architect", "OWNER"},
			{"owner", "Owner Lab", "OWNER"},
			{"noc", "NOC Lab Specialist", "NOC"},
			{"sales", "Sales Lab", "SALES"},
			{"teknisi", "Field Tech Lab", "TECHNICIAN"},
		}
		for _, u := range users {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO users (id, tenant_id, username, password_hash, full_name, email, phone, role, status, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'ACTIVE', $9)
				ON CONFLICT (tenant_id, username) DO NOTHING
			`, uuid.New().String(), devID, u.u, string(devPwHash), u.n, u.u+"@dev.ispsync.id", "081100002026", u.r, now)
		}

		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO plans (id, tenant_id, code, name, speed_down_mbps, speed_up_mbps, monthly_price, description, is_active)
			VALUES ($1, $2, 'DEV-GIGA', 'Dev HyperSpeed 1 Gbps', 1000, 1000, 990000, 'Koneksi R&D Super Cepat', 1)
			ON CONFLICT (tenant_id, code) DO NOTHING
		`, uuid.New().String(), devID)

		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO odps (id, tenant_id, code, name, latitude, longitude, total_ports, used_ports, status)
			VALUES ($1, $2, 'ODP-DEV-001', 'ODP Testbed NOC Core', -0.9400, 100.3700, 16, 2, 'ACTIVE')
			ON CONFLICT (tenant_id, code) DO NOTHING
		`, uuid.New().String(), devID)
	}

	return nil
}

// ── Tenant Methods ─────────────────────────────────────────────────────────────

func (s *PostgresStorage) GetTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, short_name, prefix_id, logo_url, brand_color, contact_phone, contact_email, address, custom_domain, COALESCE(base_staff_quota, 3), status, created_at, updated_at
		FROM tenants WHERE LOWER(slug) = LOWER($1)
	`, strings.ToLower(slug))

	var t domain.Tenant
	err := row.Scan(&t.ID, &t.Slug, &t.Name, &t.ShortName, &t.PrefixID, &t.LogoURL, &t.BrandColor, &t.ContactPhone, &t.ContactEmail, &t.Address, &t.CustomDomain, &t.BaseStaffQuota, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *PostgresStorage) GetTenantByCustomDomain(ctx context.Context, domainName string) (*domain.Tenant, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, short_name, prefix_id, logo_url, brand_color, contact_phone, contact_email, address, custom_domain, COALESCE(base_staff_quota, 3), status, created_at, updated_at
		FROM tenants WHERE LOWER(custom_domain) = LOWER($1)
	`, strings.ToLower(domainName))

	var t domain.Tenant
	err := row.Scan(&t.ID, &t.Slug, &t.Name, &t.ShortName, &t.PrefixID, &t.LogoURL, &t.BrandColor, &t.ContactPhone, &t.ContactEmail, &t.Address, &t.CustomDomain, &t.BaseStaffQuota, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *PostgresStorage) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, slug, name, short_name, prefix_id, logo_url, brand_color, contact_phone, contact_email, address, custom_domain, COALESCE(base_staff_quota, 3), status, created_at, updated_at
		FROM tenants ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Tenant
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.ShortName, &t.PrefixID, &t.LogoURL, &t.BrandColor, &t.ContactPhone, &t.ContactEmail, &t.Address, &t.CustomDomain, &t.BaseStaffQuota, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

func (s *PostgresStorage) CreateTenant(ctx context.Context, tenant *domain.Tenant) error {
	now := time.Now()
	if tenant.ID == "" {
		tenant.ID = uuid.New().String()
	}
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	if tenant.BaseStaffQuota <= 0 {
		tenant.BaseStaffQuota = 3
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tenants (id, slug, name, short_name, prefix_id, logo_url, brand_color, contact_phone, contact_email, address, custom_domain, base_staff_quota, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, tenant.ID, strings.ToLower(tenant.Slug), tenant.Name, tenant.ShortName, tenant.PrefixID, tenant.LogoURL, tenant.BrandColor, tenant.ContactPhone, tenant.ContactEmail, tenant.Address, tenant.CustomDomain, tenant.BaseStaffQuota, tenant.Status, tenant.CreatedAt, tenant.UpdatedAt)
	return err
}

// ── User Methods ───────────────────────────────────────────────────────────────

func (s *PostgresStorage) GetUserByUsername(ctx context.Context, tenantID, username string) (*domain.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, username, password_hash, full_name, email, phone, role, status, created_at
		FROM users WHERE tenant_id = $1 AND username = $2
	`, tenantID, username)

	var u domain.User
	err := row.Scan(&u.ID, &u.TenantID, &u.Username, &u.PasswordHash, &u.FullName, &u.Email, &u.Phone, &u.Role, &u.Status, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *PostgresStorage) ListUsersByTenant(ctx context.Context, tenantID string) ([]domain.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, username, password_hash, full_name, email, phone, role, status, created_at
		FROM users WHERE tenant_id = $1 ORDER BY created_at ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.TenantID, &u.Username, &u.PasswordHash, &u.FullName, &u.Email, &u.Phone, &u.Role, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, nil
}

func (s *PostgresStorage) CreateUser(ctx context.Context, user *domain.User, rawPassword string) error {
	if strings.ToUpper(user.Role) != "OWNER" {
		quota, err := s.GetStaffQuotaStatus(ctx, user.TenantID)
		if err != nil {
			return err
		}
		if !quota.CanAddStaff {
			return fmt.Errorf("batas kuota staf tercapai (%d/%d akun aktif), silakan beli add-on staf", quota.UsedStaffCount, quota.TotalQuota)
		}
	}

	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	now := time.Now()
	user.CreatedAt = now
	pwHash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(pwHash)
	if user.Status == "" {
		user.Status = "ACTIVE"
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (id, tenant_id, username, password_hash, full_name, email, phone, role, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, user.ID, user.TenantID, user.Username, user.PasswordHash, user.FullName, user.Email, user.Phone, strings.ToUpper(user.Role), user.Status, user.CreatedAt)
	return err
}

// ── Add-ons & Staff Quota ───────────────────────────────────────────────────────

func (s *PostgresStorage) GetStaffQuotaStatus(ctx context.Context, tenantID string) (*domain.StaffQuotaStatus, error) {
	baseQuota := 3
	var dbBase sql.NullInt32
	_ = s.db.QueryRowContext(ctx, "SELECT base_staff_quota FROM tenants WHERE id = $1", tenantID).Scan(&dbBase)
	if dbBase.Valid && dbBase.Int32 > 0 {
		baseQuota = int(dbBase.Int32)
	}

	var addonQuota sql.NullInt32
	_ = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(quantity), 0) FROM tenant_addons 
		WHERE tenant_id = $1 AND status = 'ACTIVE' AND addon_type = 'STAFF_SEAT' 
		AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
	`, tenantID).Scan(&addonQuota)

	var usedStaff sql.NullInt32
	_ = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users 
		WHERE tenant_id = $1 AND status = 'ACTIVE' AND UPPER(role) != 'OWNER'
	`, tenantID).Scan(&usedStaff)

	aq := 0
	if addonQuota.Valid {
		aq = int(addonQuota.Int32)
	}
	used := 0
	if usedStaff.Valid {
		used = int(usedStaff.Int32)
	}
	total := baseQuota + aq
	remaining := total - used
	if remaining < 0 {
		remaining = 0
	}

	return &domain.StaffQuotaStatus{
		TenantID:       tenantID,
		BaseQuota:      baseQuota,
		AddonQuota:     aq,
		TotalQuota:     total,
		UsedStaffCount: used,
		RemainingQuota: remaining,
		CanAddStaff:    remaining > 0,
	}, nil
}

func (s *PostgresStorage) ListTenantAddons(ctx context.Context, tenantID string) ([]domain.TenantAddon, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, addon_code, addon_type, name, quantity, monthly_price, status, expires_at, created_at, updated_at
		FROM tenant_addons WHERE tenant_id = $1 ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.TenantAddon
	for rows.Next() {
		var a domain.TenantAddon
		if err := rows.Scan(&a.ID, &a.TenantID, &a.AddonCode, &a.AddonType, &a.Name, &a.Quantity, &a.MonthlyPrice, &a.Status, &a.ExpiresAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, nil
}

func (s *PostgresStorage) CreateTenantAddon(ctx context.Context, addon *domain.TenantAddon) error {
	if addon.ID == "" {
		addon.ID = uuid.New().String()
	}
	now := time.Now()
	addon.CreatedAt = now
	addon.UpdatedAt = now
	if addon.Status == "" {
		addon.Status = "ACTIVE"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tenant_addons (id, tenant_id, addon_code, addon_type, name, quantity, monthly_price, status, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, addon.ID, addon.TenantID, addon.AddonCode, addon.AddonType, addon.Name, addon.Quantity, addon.MonthlyPrice, addon.Status, addon.ExpiresAt, addon.CreatedAt, addon.UpdatedAt)
	return err
}


// ── Plan Methods ───────────────────────────────────────────────────────────────

func (s *PostgresStorage) ListPlans(ctx context.Context, tenantID string) ([]domain.Plan, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, code, name, speed_down_mbps, speed_up_mbps, monthly_price, description, is_active
		FROM plans WHERE tenant_id = $1 AND is_active = 1 ORDER BY monthly_price ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Plan
	for rows.Next() {
		var p domain.Plan
		var activeInt int
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Code, &p.Name, &p.SpeedDownMbps, &p.SpeedUpMbps, &p.MonthlyPrice, &p.Description, &activeInt); err != nil {
			return nil, err
		}
		p.IsActive = activeInt == 1
		list = append(list, p)
	}
	return list, nil
}

func (s *PostgresStorage) GetPlanByID(ctx context.Context, tenantID, id string) (*domain.Plan, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, code, name, speed_down_mbps, speed_up_mbps, monthly_price, description, is_active
		FROM plans WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var p domain.Plan
	var activeInt int
	if err := row.Scan(&p.ID, &p.TenantID, &p.Code, &p.Name, &p.SpeedDownMbps, &p.SpeedUpMbps, &p.MonthlyPrice, &p.Description, &activeInt); err != nil {
		return nil, err
	}
	p.IsActive = activeInt == 1
	return &p, nil
}

// ── ODP Methods ────────────────────────────────────────────────────────────────

func (s *PostgresStorage) ListODPs(ctx context.Context, tenantID string) ([]domain.ODP, error) {
	query := `
		SELECT o.id, o.tenant_id, o.code, o.name, o.latitude, o.longitude, o.total_ports, o.used_ports, o.status, 0 as is_shared, '' as owner_slug, '' as owner_name
		FROM odps o WHERE o.tenant_id = $1
		UNION ALL
		SELECT o.id, o.tenant_id, o.code, '[JARTAPLOK ' || UPPER(tp.slug) || '] ' || o.name as name, o.latitude, o.longitude, s.allocated_ports as total_ports, s.used_ports as used_ports, o.status, 1 as is_shared, tp.slug as owner_slug, tp.name as owner_name
		FROM jartaplok_shared_odps s
		JOIN jartaplok_agreements a ON s.agreement_id = a.id
		JOIN odps o ON s.odp_id = o.id
		JOIN tenants tp ON a.provider_tenant_id = tp.id
		WHERE a.client_tenant_id = $2 AND a.status = 'ACTIVE'
		ORDER BY code ASC
	`
	rows, err := s.db.QueryContext(ctx, query, tenantID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.ODP
	for rows.Next() {
		var o domain.ODP
		var isSharedInt int
		if err := rows.Scan(&o.ID, &o.TenantID, &o.Code, &o.Name, &o.Latitude, &o.Longitude, &o.TotalPorts, &o.UsedPorts, &o.Status, &isSharedInt, &o.OwnerTenantSlug, &o.OwnerTenantName); err != nil {
			return nil, err
		}
		o.IsSharedJartaplok = isSharedInt == 1
		list = append(list, o)
	}
	return list, nil
}

func (s *PostgresStorage) ListJartaplokAgreements(ctx context.Context, tenantID string) ([]domain.JartaplokAgreement, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.agreement_no, a.provider_tenant_id, tp.slug, tp.name,
		       a.client_tenant_id, tc.slug, tc.name, a.scope_area,
		       a.total_shared_odps, a.allocated_ports, a.used_ports, a.settlement_rate_per_port, a.status, a.created_at
		FROM jartaplok_agreements a
		JOIN tenants tp ON a.provider_tenant_id = tp.id
		JOIN tenants tc ON a.client_tenant_id = tc.id
		WHERE a.provider_tenant_id = $1 OR a.client_tenant_id = $2
		ORDER BY a.created_at DESC
	`, tenantID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.JartaplokAgreement
	for rows.Next() {
		var ag domain.JartaplokAgreement
		if err := rows.Scan(
			&ag.ID, &ag.AgreementNo, &ag.ProviderTenantID, &ag.ProviderTenantSlug, &ag.ProviderTenantName,
			&ag.ClientTenantID, &ag.ClientTenantSlug, &ag.ClientTenantName, &ag.ScopeArea,
			&ag.TotalSharedODPs, &ag.AllocatedPorts, &ag.UsedPorts, &ag.SettlementRatePerPort, &ag.Status, &ag.CreatedAt,
		); err != nil {
			return nil, err
		}
		ag.MonthlyBill = float64(ag.AllocatedPorts) * ag.SettlementRatePerPort
		list = append(list, ag)
	}
	return list, nil
}

func (s *PostgresStorage) GetNearestODP(ctx context.Context, tenantID string, lat, lng float64) (*domain.ODP, float64, error) {
	odps, err := s.ListODPs(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	if len(odps) == 0 {
		return nil, 0, fmt.Errorf("no ODP found for tenant")
	}

	var best *domain.ODP
	minDist := math.MaxFloat64

	for i := range odps {
		d := haversineDistance(lat, lng, odps[i].Latitude, odps[i].Longitude)
		if d < minDist {
			minDist = d
			best = &odps[i]
		}
	}

	return best, minDist, nil
}

func (s *PostgresStorage) UpsertODP(ctx context.Context, odp *domain.ODP) error {
	if odp.ID == "" {
		odp.ID = uuid.New().String()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO odps (id, tenant_id, code, name, latitude, longitude, total_ports, used_ports, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, code) DO UPDATE SET
			name = EXCLUDED.name,
			latitude = EXCLUDED.latitude,
			longitude = EXCLUDED.longitude,
			total_ports = EXCLUDED.total_ports,
			used_ports = EXCLUDED.used_ports,
			status = EXCLUDED.status
	`, odp.ID, odp.TenantID, odp.Code, odp.Name, odp.Latitude, odp.Longitude, odp.TotalPorts, odp.UsedPorts, odp.Status)
	return err
}

// ── OLT Methods ────────────────────────────────────────────────────────────────

func (s *PostgresStorage) ListOLTs(ctx context.Context, tenantID string) ([]domain.OLT, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, name, vendor, host_ip, port, username, status, total_pons
		FROM olts WHERE tenant_id = $1 ORDER BY name ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.OLT
	for rows.Next() {
		var o domain.OLT
		if err := rows.Scan(&o.ID, &o.TenantID, &o.Name, &o.Vendor, &o.HostIP, &o.Port, &o.Username, &o.Status, &o.TotalPONs); err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, nil
}

func (s *PostgresStorage) GetOLTByID(ctx context.Context, tenantID, id string) (*domain.OLT, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, name, vendor, host_ip, port, username, password, status, total_pons
		FROM olts WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var o domain.OLT
	if err := row.Scan(&o.ID, &o.TenantID, &o.Name, &o.Vendor, &o.HostIP, &o.Port, &o.Username, &o.Password, &o.Status, &o.TotalPONs); err != nil {
		return nil, err
	}
	return &o, nil
}

// ── Subscriber Methods ─────────────────────────────────────────────────────────

func (s *PostgresStorage) ListSubscribers(ctx context.Context, tenantID string, status string) ([]domain.Subscriber, error) {
	query := `
		SELECT id, tenant_id, subscriber_no, full_name, identity_number, email, phone, address,
		       latitude, longitude, distance_to_odp, selected_plan_id, selected_plan_name,
		       nearest_odp_id, nearest_odp_code, olt_id, pon_port, onu_id, serial_number, mac_address,
		       rx_optical_power, pppoe_username, pppoe_password, vlan_id, ip_address, status,
		       activated_at, suspended_at, created_at, updated_at
		FROM subscribers WHERE tenant_id = $1
	`
	args := []interface{}{tenantID}
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Subscriber
	for rows.Next() {
		var sub domain.Subscriber
		if err := rows.Scan(
			&sub.ID, &sub.TenantID, &sub.SubscriberNo, &sub.FullName, &sub.IdentityNumber, &sub.Email, &sub.Phone, &sub.Address,
			&sub.Latitude, &sub.Longitude, &sub.DistanceToODP, &sub.SelectedPlanID, &sub.SelectedPlanName,
			&sub.NearestODPID, &sub.NearestODPCode, &sub.OLTID, &sub.PONPort, &sub.ONUID, &sub.SerialNumber, &sub.MACAddress,
			&sub.RxOpticalPower, &sub.PPPoEUsername, &sub.PPPoEPassword, &sub.VLANID, &sub.IPAddress, &sub.Status,
			&sub.ActivatedAt, &sub.SuspendedAt, &sub.CreatedAt, &sub.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, sub)
	}
	return list, nil
}

func (s *PostgresStorage) GetSubscriberByID(ctx context.Context, tenantID, id string) (*domain.Subscriber, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, subscriber_no, full_name, identity_number, email, phone, address,
		       latitude, longitude, distance_to_odp, selected_plan_id, selected_plan_name,
		       nearest_odp_id, nearest_odp_code, olt_id, pon_port, onu_id, serial_number, mac_address,
		       rx_optical_power, pppoe_username, pppoe_password, vlan_id, ip_address, status,
		       activated_at, suspended_at, created_at, updated_at
		FROM subscribers WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var sub domain.Subscriber
	if err := row.Scan(
		&sub.ID, &sub.TenantID, &sub.SubscriberNo, &sub.FullName, &sub.IdentityNumber, &sub.Email, &sub.Phone, &sub.Address,
		&sub.Latitude, &sub.Longitude, &sub.DistanceToODP, &sub.SelectedPlanID, &sub.SelectedPlanName,
		&sub.NearestODPID, &sub.NearestODPCode, &sub.OLTID, &sub.PONPort, &sub.ONUID, &sub.SerialNumber, &sub.MACAddress,
		&sub.RxOpticalPower, &sub.PPPoEUsername, &sub.PPPoEPassword, &sub.VLANID, &sub.IPAddress, &sub.Status,
		&sub.ActivatedAt, &sub.SuspendedAt, &sub.CreatedAt, &sub.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *PostgresStorage) GetSubscriberByNo(ctx context.Context, tenantID, subNo string) (*domain.Subscriber, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, subscriber_no, full_name, identity_number, email, phone, address,
		       latitude, longitude, distance_to_odp, selected_plan_id, selected_plan_name,
		       nearest_odp_id, nearest_odp_code, olt_id, pon_port, onu_id, serial_number, mac_address,
		       rx_optical_power, pppoe_username, pppoe_password, vlan_id, ip_address, status,
		       activated_at, suspended_at, created_at, updated_at
		FROM subscribers WHERE tenant_id = $1 AND subscriber_no = $2
	`, tenantID, subNo)

	var sub domain.Subscriber
	if err := row.Scan(
		&sub.ID, &sub.TenantID, &sub.SubscriberNo, &sub.FullName, &sub.IdentityNumber, &sub.Email, &sub.Phone, &sub.Address,
		&sub.Latitude, &sub.Longitude, &sub.DistanceToODP, &sub.SelectedPlanID, &sub.SelectedPlanName,
		&sub.NearestODPID, &sub.NearestODPCode, &sub.OLTID, &sub.PONPort, &sub.ONUID, &sub.SerialNumber, &sub.MACAddress,
		&sub.RxOpticalPower, &sub.PPPoEUsername, &sub.PPPoEPassword, &sub.VLANID, &sub.IPAddress, &sub.Status,
		&sub.ActivatedAt, &sub.SuspendedAt, &sub.CreatedAt, &sub.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *PostgresStorage) CreateSubscriber(ctx context.Context, sub *domain.Subscriber) error {
	now := time.Now()
	if sub.ID == "" {
		sub.ID = uuid.New().String()
	}
	sub.CreatedAt = now
	sub.UpdatedAt = now

	if sub.SubscriberNo == "" {
		var count int
		_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM subscribers WHERE tenant_id = $1", sub.TenantID).Scan(&count)
		sub.SubscriberNo = fmt.Sprintf("SUB-%s-%04d", time.Now().Format("2006"), count+1)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subscribers (
			id, tenant_id, subscriber_no, full_name, identity_number, email, phone, address,
			latitude, longitude, distance_to_odp, selected_plan_id, selected_plan_name,
			nearest_odp_id, nearest_odp_code, olt_id, pon_port, onu_id, serial_number, mac_address,
			rx_optical_power, pppoe_username, pppoe_password, vlan_id, ip_address, status,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26,
			$27, $28
		)
	`, sub.ID, sub.TenantID, sub.SubscriberNo, sub.FullName, sub.IdentityNumber, sub.Email, sub.Phone, sub.Address,
		sub.Latitude, sub.Longitude, sub.DistanceToODP, sub.SelectedPlanID, sub.SelectedPlanName,
		sub.NearestODPID, sub.NearestODPCode, sub.OLTID, sub.PONPort, sub.ONUID, sub.SerialNumber, sub.MACAddress,
		sub.RxOpticalPower, sub.PPPoEUsername, sub.PPPoEPassword, sub.VLANID, sub.IPAddress, sub.Status,
		sub.CreatedAt, sub.UpdatedAt)
	return err
}

func (s *PostgresStorage) UpdateSubscriberStatus(ctx context.Context, tenantID, id, status string) error {
	now := time.Now()
	var actTime, suspTime *time.Time
	if status == "ACTIVE" {
		actTime = &now
	} else if status == "RESTRICTED" || status == "SUSPENDED" {
		suspTime = &now
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE subscribers SET status = $1, activated_at = COALESCE($2, activated_at), suspended_at = COALESCE($3, suspended_at), updated_at = $4
		WHERE tenant_id = $5 AND id = $6
	`, status, actTime, suspTime, now, tenantID, id)
	return err
}

func (s *PostgresStorage) UpdateSubscriberProvisioning(ctx context.Context, tenantID, id string, oltID *string, ponPort *string, onuID *int, sn, mac *string, rxPower *float64, pppoeUser, pppoePass *string, vlan *int, ip *string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE subscribers SET
			olt_id = COALESCE($1, olt_id),
			pon_port = COALESCE($2, pon_port),
			onu_id = COALESCE($3, onu_id),
			serial_number = COALESCE($4, serial_number),
			mac_address = COALESCE($5, mac_address),
			rx_optical_power = COALESCE($6, rx_optical_power),
			pppoe_username = COALESCE($7, pppoe_username),
			pppoe_password = COALESCE($8, pppoe_password),
			vlan_id = COALESCE($9, vlan_id),
			ip_address = COALESCE($10, ip_address),
			updated_at = $11
		WHERE tenant_id = $12 AND id = $13
	`, oltID, ponPort, onuID, sn, mac, rxPower, pppoeUser, pppoePass, vlan, ip, now, tenantID, id)
	return err
}

// ── Work Order Methods ─────────────────────────────────────────────────────────

func (s *PostgresStorage) ListWorkOrders(ctx context.Context, tenantID string, status string) ([]domain.WorkOrder, error) {
	query := `
		SELECT id, tenant_id, order_no, subscriber_id, subscriber_no, customer_name, customer_phone, customer_address,
		       customer_lat, customer_lng, odp_code, plan_name, order_type, technician_id, technician_name, status,
		       rx_power_dbm, serial_number, mac_address, notes, bast_completed_at, created_at, updated_at
		FROM work_orders WHERE tenant_id = $1
	`
	args := []interface{}{tenantID}
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.WorkOrder
	for rows.Next() {
		var wo domain.WorkOrder
		if err := rows.Scan(
			&wo.ID, &wo.TenantID, &wo.OrderNo, &wo.SubscriberID, &wo.SubscriberNo, &wo.CustomerName, &wo.CustomerPhone, &wo.CustomerAddress,
			&wo.CustomerLat, &wo.CustomerLng, &wo.ODPCode, &wo.PlanName, &wo.OrderType, &wo.TechnicianID, &wo.TechnicianName, &wo.Status,
			&wo.RxPowerDBM, &wo.SerialNumber, &wo.MACAddress, &wo.Notes, &wo.BASTCompletedAt, &wo.CreatedAt, &wo.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, wo)
	}
	return list, nil
}

func (s *PostgresStorage) GetWorkOrderByID(ctx context.Context, tenantID, id string) (*domain.WorkOrder, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, order_no, subscriber_id, subscriber_no, customer_name, customer_phone, customer_address,
		       customer_lat, customer_lng, odp_code, plan_name, order_type, technician_id, technician_name, status,
		       rx_power_dbm, serial_number, mac_address, notes, bast_completed_at, created_at, updated_at
		FROM work_orders WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)

	var wo domain.WorkOrder
	if err := row.Scan(
		&wo.ID, &wo.TenantID, &wo.OrderNo, &wo.SubscriberID, &wo.SubscriberNo, &wo.CustomerName, &wo.CustomerPhone, &wo.CustomerAddress,
		&wo.CustomerLat, &wo.CustomerLng, &wo.ODPCode, &wo.PlanName, &wo.OrderType, &wo.TechnicianID, &wo.TechnicianName, &wo.Status,
		&wo.RxPowerDBM, &wo.SerialNumber, &wo.MACAddress, &wo.Notes, &wo.BASTCompletedAt, &wo.CreatedAt, &wo.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &wo, nil
}

func (s *PostgresStorage) CreateWorkOrder(ctx context.Context, wo *domain.WorkOrder) error {
	now := time.Now()
	if wo.ID == "" {
		wo.ID = uuid.New().String()
	}
	wo.CreatedAt = now
	wo.UpdatedAt = now

	if wo.OrderNo == "" {
		var count int
		_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM work_orders WHERE tenant_id = $1", wo.TenantID).Scan(&count)
		wo.OrderNo = fmt.Sprintf("SPK-%s-%04d", time.Now().Format("2006"), count+1)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO work_orders (
			id, tenant_id, order_no, subscriber_id, subscriber_no, customer_name, customer_phone, customer_address,
			customer_lat, customer_lng, odp_code, plan_name, order_type, technician_id, technician_name, status,
			rx_power_dbm, serial_number, mac_address, notes, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22
		)
	`, wo.ID, wo.TenantID, wo.OrderNo, wo.SubscriberID, wo.SubscriberNo, wo.CustomerName, wo.CustomerPhone, wo.CustomerAddress,
		wo.CustomerLat, wo.CustomerLng, wo.ODPCode, wo.PlanName, wo.OrderType, wo.TechnicianID, wo.TechnicianName, wo.Status,
		wo.RxPowerDBM, wo.SerialNumber, wo.MACAddress, wo.Notes, wo.CreatedAt, wo.UpdatedAt)
	return err
}

func (s *PostgresStorage) CompleteWorkOrderBAST(ctx context.Context, tenantID, id string, rxPower float64, sn, mac, notes string) error {
	now := time.Now()
	res, err := s.db.ExecContext(ctx, `
		UPDATE work_orders SET
			status = 'COMPLETED',
			rx_power_dbm = $1,
			serial_number = $2,
			mac_address = $3,
			notes = $4,
			bast_completed_at = $5,
			updated_at = $6
		WHERE tenant_id = $7 AND id = $8
	`, rxPower, sn, mac, notes, now, now, tenantID, id)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("work order not found or unauthorized")
	}

	var subID string
	_ = s.db.QueryRowContext(ctx, "SELECT subscriber_id FROM work_orders WHERE tenant_id = $1 AND id = $2", tenantID, id).Scan(&subID)
	if subID != "" {
		_ = s.UpdateSubscriberProvisioning(ctx, tenantID, subID, nil, nil, nil, &sn, &mac, &rxPower, nil, nil, nil, nil)
		_ = s.UpdateSubscriberStatus(ctx, tenantID, subID, "ACTIVE")
	}

	return nil
}

// ── Custom Domain & TLS Check ──────────────────────────────────────────────────

func (s *PostgresStorage) UpdateTenantCustomDomain(ctx context.Context, tenantID, customDomain string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE tenants SET custom_domain = $1, updated_at = $2 WHERE id = $3", strings.ToLower(strings.TrimSpace(customDomain)), time.Now(), tenantID)
	return err
}

func (s *PostgresStorage) UpdateTenantProfile(ctx context.Context, tenantID, logoUrl, brandColor, contactPhone, contactEmail string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE tenants SET logo_url = $1, brand_color = $2, contact_phone = $3, contact_email = $4, updated_at = $5 WHERE id = $6", strings.TrimSpace(logoUrl), strings.TrimSpace(brandColor), strings.TrimSpace(contactPhone), strings.TrimSpace(contactEmail), time.Now(), tenantID)
	return err
}

func (s *PostgresStorage) CreateMikrotikRouter(ctx context.Context, r *domain.MikrotikRouter) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mikrotik_routers (id, tenant_id, name, wg_pubkey, wg_ip, api_port, api_user, api_password, status, last_seen, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT(id) DO UPDATE SET 
		wg_pubkey=excluded.wg_pubkey, wg_ip=excluded.wg_ip, api_password=excluded.api_password, updated_at=excluded.updated_at
	`, r.ID, r.TenantID, r.Name, r.WgPubkey, r.WgIP, r.APIPort, r.APIUser, r.APIPassword, r.Status, r.LastSeen, r.CreatedAt, r.UpdatedAt)
	return err
}

func (s *PostgresStorage) GetMikrotikRouter(ctx context.Context, tenantID string) (*domain.MikrotikRouter, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id, tenant_id, name, wg_pubkey, wg_ip, api_port, api_user, api_password, status, last_seen, created_at, updated_at FROM mikrotik_routers WHERE tenant_id = $1", tenantID)
	var r domain.MikrotikRouter
	err := row.Scan(&r.ID, &r.TenantID, &r.Name, &r.WgPubkey, &r.WgIP, &r.APIPort, &r.APIUser, &r.APIPassword, &r.Status, &r.LastSeen, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

func (s *PostgresStorage) ValidateDomainForTLS(ctx context.Context, domainName string) bool {
	d := strings.ToLower(strings.TrimSpace(domainName))
	if d == "" {
		return false
	}
	if d == "cms.ispsync.id" {
		return false
	}
	if d == "ispsync.id" || strings.HasSuffix(d, ".ispsync.id") {
		return true
	}
	var count int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenants WHERE LOWER(custom_domain) = $1 AND status = 'ACTIVE'", d).Scan(&count)
	return count > 0
}

// ── Invoicing Methods ──────────────────────────────────────────────────────────

func (s *PostgresStorage) ListInvoices(ctx context.Context, tenantID string, status string) ([]domain.Invoice, error) {
	query := `SELECT id, tenant_id, invoice_no, subscriber_id, subscriber_no, customer_name, plan_name, amount, status, due_date, paid_at, created_at, updated_at
	          FROM invoices WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Invoice
	for rows.Next() {
		var inv domain.Invoice
		if err := rows.Scan(&inv.ID, &inv.TenantID, &inv.InvoiceNo, &inv.SubscriberID, &inv.SubscriberNo, &inv.CustomerName, &inv.PlanName, &inv.Amount, &inv.Status, &inv.DueDate, &inv.PaidAt, &inv.CreatedAt, &inv.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, inv)
	}
	return list, nil
}

func (s *PostgresStorage) CreateInvoice(ctx context.Context, inv *domain.Invoice) error {
	now := time.Now()
	if inv.ID == "" {
		inv.ID = uuid.New().String()
	}
	if inv.InvoiceNo == "" {
		var count int
		_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM invoices WHERE tenant_id = $1", inv.TenantID).Scan(&count)
		inv.InvoiceNo = fmt.Sprintf("INV-%s-%04d", time.Now().Format("200601"), count+1)
	}
	inv.CreatedAt = now
	inv.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO invoices (id, tenant_id, invoice_no, subscriber_id, subscriber_no, customer_name, plan_name, amount, status, due_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, inv.ID, inv.TenantID, inv.InvoiceNo, inv.SubscriberID, inv.SubscriberNo, inv.CustomerName, inv.PlanName, inv.Amount, inv.Status, inv.DueDate, inv.CreatedAt, inv.UpdatedAt)
	return err
}

func (s *PostgresStorage) MarkInvoicePaid(ctx context.Context, tenantID, invoiceID string) error {
	now := time.Now()
	var subID string
	err := s.db.QueryRowContext(ctx, "SELECT subscriber_id FROM invoices WHERE tenant_id = $1 AND id = $2", tenantID, invoiceID).Scan(&subID)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE invoices SET status = 'PAID', paid_at = $1, updated_at = $2 WHERE tenant_id = $3 AND id = $4
	`, now, now, tenantID, invoiceID)
	if err != nil {
		return err
	}

	_ = s.UpdateSubscriberStatus(ctx, tenantID, subID, "ACTIVE")
	return nil
}

// ── Voucher Methods ────────────────────────────────────────────────────────────

func (s *PostgresStorage) ListVouchers(ctx context.Context, tenantID string) ([]domain.Voucher, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, batch_id, code, password, price, profile_name, status, used_at, created_at
		FROM vouchers WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 100
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Voucher
	for rows.Next() {
		var v domain.Voucher
		if err := rows.Scan(&v.ID, &v.TenantID, &v.BatchID, &v.Code, &v.Password, &v.Price, &v.ProfileName, &v.Status, &v.UsedAt, &v.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, nil
}

func (s *PostgresStorage) GenerateVouchers(ctx context.Context, tenantID string, profileName string, speedDown, speedUp int, price float64, count int) (*domain.VoucherBatch, error) {
	now := time.Now()
	batchID := uuid.New().String()
	var bCount int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM voucher_batches WHERE tenant_id = $1", tenantID).Scan(&bCount)
	batchNo := fmt.Sprintf("BATCH-%s-%03d", time.Now().Format("200601"), bCount+1)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO voucher_batches (id, tenant_id, batch_no, profile_name, speed_down_mbps, speed_up_mbps, price, total_vouchers, used_vouchers, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0, $9)
	`, batchID, tenantID, batchNo, profileName, speedDown, speedUp, price, count, now)
	if err != nil {
		return nil, err
	}

	for i := 0; i < count; i++ {
		vID := uuid.New().String()
		code := fmt.Sprintf("V%02d%04d", time.Now().Unix()%100, i+1)
		pass := fmt.Sprintf("%04d", (i*37+19)%9000+1000)

		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO vouchers (id, tenant_id, batch_id, code, password, price, profile_name, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'AVAILABLE', $8)
		`, vID, tenantID, batchID, code, pass, price, profileName, now)
	}

	return &domain.VoucherBatch{
		ID:            batchID,
		TenantID:      tenantID,
		BatchNo:       batchNo,
		ProfileName:   profileName,
		SpeedDownMbps: speedDown,
		SpeedUpMbps:   speedUp,
		Price:         price,
		TotalVouchers: count,
		UsedVouchers:  0,
		CreatedAt:     now,
	}, nil
}

func (s *PostgresStorage) GetTenantCapabilities(ctx context.Context, tenantID string) (domain.TenantCapabilities, error) {
	caps := domain.TenantCapabilities{TenantID: tenantID}
	err := s.db.QueryRowContext(ctx, "SELECT uses_fibergrid, own_infrastructure FROM tenant_capabilities WHERE tenant_id = $1", tenantID).Scan(&caps.UsesFiberGrid, &caps.OwnInfrastructure)
	if err == sql.ErrNoRows {
		return caps, nil
	}
	return caps, err
}

func (s *PostgresStorage) SetTenantCapabilities(ctx context.Context, caps domain.TenantCapabilities) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO tenant_capabilities (tenant_id, uses_fibergrid, own_infrastructure, updated_at) VALUES ($1, $2, $3, CURRENT_TIMESTAMP) ON CONFLICT (tenant_id) DO UPDATE SET uses_fibergrid = excluded.uses_fibergrid, own_infrastructure = excluded.own_infrastructure, updated_at = excluded.updated_at", caps.TenantID, caps.UsesFiberGrid, caps.OwnInfrastructure)
	return err
}

func (s *PostgresStorage) UpdateUserStatus(ctx context.Context, tenantID, userID, status string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET status = $1 WHERE tenant_id = $2 AND id = $3", strings.ToUpper(status), tenantID, userID)
	return err
}

func (s *PostgresStorage) ResetUserPassword(ctx context.Context, tenantID, username, newPassword string) error {
	pwHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE users SET password_hash = $1 WHERE tenant_id = $2 AND LOWER(username) = LOWER($3)", string(pwHash), tenantID, username)
	return err
}
