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
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite write safety
	db.SetMaxIdleConns(1)

	s := &SQLiteStorage{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate db: %w", err)
	}

	if err := s.seedDefaultTenant(); err != nil {
		return nil, fmt.Errorf("failed to seed tenant: %w", err)
	}

	return s, nil
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

func (s *SQLiteStorage) migrate() error {
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
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		username TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		full_name TEXT NOT NULL,
		email TEXT DEFAULT '',
		phone TEXT DEFAULT '',
		role TEXT NOT NULL,
		status TEXT DEFAULT 'ACTIVE',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
		UNIQUE(tenant_id, username)
	);

	CREATE TABLE IF NOT EXISTS plans (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		speed_down_mbps INTEGER DEFAULT 20,
		speed_up_mbps INTEGER DEFAULT 20,
		monthly_price REAL DEFAULT 0,
		description TEXT DEFAULT '',
		is_active INTEGER DEFAULT 1,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
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
		last_seen DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS odps (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		latitude REAL NOT NULL,
		longitude REAL NOT NULL,
		total_ports INTEGER DEFAULT 8,
		used_ports INTEGER DEFAULT 0,
		status TEXT DEFAULT 'ACTIVE',
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
		UNIQUE(tenant_id, code)
	);

	CREATE TABLE IF NOT EXISTS olts (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
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
		total_pons INTEGER DEFAULT 1,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS subscribers (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		subscriber_no TEXT NOT NULL,
		full_name TEXT NOT NULL,
		identity_number TEXT DEFAULT '',
		email TEXT DEFAULT '',
		phone TEXT DEFAULT '',
		address TEXT DEFAULT '',
		latitude REAL DEFAULT 0,
		longitude REAL DEFAULT 0,
		distance_to_odp REAL DEFAULT 0,
		selected_plan_id TEXT,
		selected_plan_name TEXT,
		nearest_odp_id TEXT,
		nearest_odp_code TEXT,
		olt_id TEXT,
		pon_port TEXT,
		onu_id INTEGER,
		serial_number TEXT,
		mac_address TEXT,
		rx_optical_power REAL,
		pppoe_username TEXT,
		pppoe_password TEXT,
		vlan_id INTEGER,
		ip_address TEXT,
		status TEXT DEFAULT 'REGISTERED',
		activated_at DATETIME,
		suspended_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
		UNIQUE(tenant_id, subscriber_no)
	);

	CREATE TABLE IF NOT EXISTS work_orders (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		order_no TEXT NOT NULL,
		subscriber_id TEXT NOT NULL,
		subscriber_no TEXT NOT NULL,
		customer_name TEXT NOT NULL,
		customer_phone TEXT DEFAULT '',
		customer_address TEXT DEFAULT '',
		customer_lat REAL DEFAULT 0,
		customer_lng REAL DEFAULT 0,
		odp_code TEXT DEFAULT '',
		plan_name TEXT DEFAULT '',
		order_type TEXT DEFAULT 'INSTALLATION',
		technician_id TEXT,
		technician_name TEXT DEFAULT '',
		status TEXT DEFAULT 'PENDING',
		rx_power_dbm REAL,
		serial_number TEXT DEFAULT '',
		mac_address TEXT DEFAULT '',
		notes TEXT DEFAULT '',
		bast_completed_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
		FOREIGN KEY (subscriber_id) REFERENCES subscribers(id) ON DELETE CASCADE,
		UNIQUE(tenant_id, order_no)
	);

	CREATE TABLE IF NOT EXISTS invoices (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		invoice_no TEXT NOT NULL,
		subscriber_id TEXT NOT NULL,
		subscriber_no TEXT NOT NULL,
		customer_name TEXT NOT NULL,
		plan_name TEXT NOT NULL,
		amount REAL NOT NULL,
		status TEXT DEFAULT 'UNPAID',
		due_date DATETIME NOT NULL,
		paid_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
		UNIQUE(tenant_id, invoice_no)
	);

	CREATE TABLE IF NOT EXISTS voucher_batches (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		batch_no TEXT NOT NULL,
		profile_name TEXT NOT NULL,
		speed_down_mbps INTEGER DEFAULT 10,
		speed_up_mbps INTEGER DEFAULT 10,
		price REAL DEFAULT 5000,
		total_vouchers INTEGER DEFAULT 50,
		used_vouchers INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
		UNIQUE(tenant_id, batch_no)
	);

	CREATE TABLE IF NOT EXISTS vouchers (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		batch_id TEXT NOT NULL,
		code TEXT NOT NULL,
		password TEXT NOT NULL,
		price REAL DEFAULT 5000,
		profile_name TEXT NOT NULL,
		status TEXT DEFAULT 'AVAILABLE',
		used_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
		FOREIGN KEY (batch_id) REFERENCES voucher_batches(id) ON DELETE CASCADE,
		UNIQUE(tenant_id, code)
	);

	CREATE INDEX IF NOT EXISTS idx_subscribers_tenant ON subscribers(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_odps_tenant ON odps(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_workorders_tenant ON work_orders(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_invoices_tenant ON invoices(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_vouchers_tenant ON vouchers(tenant_id);

	CREATE TABLE IF NOT EXISTS tenant_capabilities (
		tenant_id TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
		uses_fibergrid INTEGER NOT NULL DEFAULT 0,
		own_infrastructure INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS jartaplok_agreements (
		id TEXT PRIMARY KEY,
		agreement_no TEXT NOT NULL,
		provider_tenant_id TEXT NOT NULL,
		client_tenant_id TEXT NOT NULL,
		scope_area TEXT NOT NULL,
		total_shared_odps INTEGER DEFAULT 0,
		allocated_ports INTEGER DEFAULT 0,
		used_ports INTEGER DEFAULT 0,
		settlement_rate_per_port REAL DEFAULT 25000,
		status TEXT DEFAULT 'ACTIVE',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (provider_tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
		FOREIGN KEY (client_tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
		UNIQUE(agreement_no)
	);

	CREATE TABLE IF NOT EXISTS jartaplok_shared_odps (
		id TEXT PRIMARY KEY,
		agreement_id TEXT NOT NULL,
		odp_id TEXT NOT NULL,
		allocated_ports INTEGER DEFAULT 8,
		used_ports INTEGER DEFAULT 0,
		FOREIGN KEY (agreement_id) REFERENCES jartaplok_agreements(id) ON DELETE CASCADE,
		FOREIGN KEY (odp_id) REFERENCES odps(id) ON DELETE CASCADE,
		UNIQUE(agreement_id, odp_id)
	);

	CREATE TABLE IF NOT EXISTS tenant_addons (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		addon_code TEXT NOT NULL,
		addon_type TEXT NOT NULL,
		name TEXT NOT NULL DEFAULT '',
		quantity INTEGER DEFAULT 5,
		monthly_price REAL DEFAULT 0,
		status TEXT DEFAULT 'ACTIVE',
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_tenant_addons_tenant ON tenant_addons(tenant_id, status);
	`
	_, err := s.db.Exec(schema)
	_ = s.db.QueryRow("SELECT base_staff_quota FROM tenants LIMIT 1").Scan(new(interface{}))
	_, _ = s.db.Exec("ALTER TABLE tenants ADD COLUMN base_staff_quota INTEGER DEFAULT 3")
	return err
}


func (s *SQLiteStorage) seedDefaultTenant() error {
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
			VALUES (?, 'ispku', 'PT. ISP Kita Nusantara', 'ISPKU', 'ISPKU', '', '#2563eb', '081288889999', 'info@ispku.ispsync.id', 'Jl. Jenderal Sudirman No. 125, Kota Pekanbaru, Riau', 'ACTIVE', ?, ?)
		`, ispkuID, now, now)
		if err != nil {
			return err
		}

		// Staff
		users := []struct{ u, n, r string }{
			{"owner", "Pimpinan ISP Kita", "OWNER"},
			{"noc", "Engineer NOC Core", "NOC"},
			{"sales", "Account Executive", "SALES"},
			{"teknisi", "Teknisi Lapangan", "TECHNICIAN"},
		}
		for _, u := range users {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO users (id, tenant_id, username, password_hash, full_name, email, role, status, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, 'ACTIVE', ?)
			`, uuid.New().String(), ispkuID, u.u, string(pwHash), u.n, u.u+"@ispku.ispsync.id", u.r, now)
		}

		// Plans
		plans := []struct {
			c, n string
			d, u int
			p    float64
			desc string
		}{
			{"HOME-20M", "Home Starter 20 Mbps", 20, 20, 165000, "Internet cepat tanpa batas FUP untuk kebutuhan rumah tangga"},
			{"HOME-50M", "Home Family 50 Mbps", 50, 50, 250000, "Streaming 4K, video conference, dan gaming lancar sekeluarga"},
			{"HOME-100M", "Home Ultra 100 Mbps", 100, 100, 375000, "Kecepatan maksimal untuk power users dan smart home"},
			{"BIZ-200M", "Business Pro 200 Mbps", 200, 200, 950000, "Koneksi dedicated prioritas tinggi untuk perkantoran dan bisnis"},
		}
		for _, p := range plans {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO plans (id, tenant_id, code, name, speed_down_mbps, speed_up_mbps, monthly_price, description, is_active)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
			`, uuid.New().String(), ispkuID, p.c, p.n, p.d, p.u, p.p, p.desc)
		}

		// ODPs
		odps := []struct {
			c, n string
			lat, lng float64
			ports int
		}{
			{"ODP-PYK-001", "ODP Simpang Benteng", -0.2235, 100.6310, 8},
			{"ODP-PYK-002", "ODP Pasar Ibuh Barat", -0.2250, 100.6340, 8},
			{"ODP-PYK-003", "ODP Tan Malaka 01", -0.2210, 100.6280, 16},
			{"ODP-PYK-004", "ODP Koto Nan Ampek", -0.2310, 100.6250, 8},
			{"ODP-PYK-005", "ODP Harau Raya", -0.2180, 100.6400, 16},
		}
		for _, o := range odps {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO odps (id, tenant_id, code, name, latitude, longitude, total_ports, used_ports, status)
				VALUES (?, ?, ?, ?, ?, ?, ?, 0, 'ACTIVE')
			`, uuid.New().String(), ispkuID, o.c, o.n, o.lat, o.lng, o.ports)
		}

		// OLT
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO olts (id, tenant_id, name, vendor, host_ip, port, username, password, status, total_pons)
			VALUES (?, ?, 'OLT-VSOL-JOLINK-01', 'JOLINK', '192.168.10.2', 23, 'admin', 'admin', 'ONLINE', 1)
		`, uuid.New().String(), ispkuID)
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO olts (id, tenant_id, name, vendor, host_ip, port, username, password, status, total_pons)
			VALUES (?, ?, 'OLT-CORE-HUAWEI-5608', 'HUAWEI', '192.168.10.5', 23, 'admin', 'admin', 'ONLINE', 8)
		`, uuid.New().String(), ispkuID)

		// Subscriber
		subID := uuid.New().String()
		subNo := "ISPKU-2026-0001"
		sn := "ZTEGC892AB11"
		mac := "E4:8D:8C:11:22:33"
		rx := -19.45
		vlan := 211
		pppoeU := "sub001@ispku"
		pppoeP := "secretpass"
		ip := "10.20.10.15"
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO subscribers (id, tenant_id, subscriber_no, full_name, identity_number, email, phone, address, latitude, longitude, distance_to_odp, selected_plan_name, nearest_odp_code, pon_port, onu_id, serial_number, mac_address, rx_optical_power, pppoe_username, pppoe_password, vlan_id, ip_address, status, activated_at, created_at, updated_at)
			VALUES (?, ?, ?, 'Rian Pratama', '1376012345670001', 'rian@example.com', '081299887766', 'Jl. Sudirman No. 45 Payakumbuh', -0.2238, 100.6312, 38.5, 'Home Family 50 Mbps', 'ODP-PYK-001', 'EPON0/1', 1, ?, ?, ?, ?, ?, ?, ?, 'ACTIVE', ?, ?, ?)
		`, subID, ispkuID, subNo, sn, mac, rx, pppoeU, pppoeP, vlan, ip, now, now, now)

		// Invoices & Vouchers for ispku
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO invoices (id, tenant_id, invoice_no, subscriber_id, subscriber_no, customer_name, plan_name, amount, status, due_date, paid_at, created_at, updated_at)
			VALUES (?, ?, 'INV-202609-0001', ?, ?, 'Rian Pratama', 'Home Family 50 Mbps', 250000, 'PAID', ?, ?, ?, ?)
		`, uuid.New().String(), ispkuID, subID, subNo, now.AddDate(0, 0, -5), now.AddDate(0, 0, -2), now.AddDate(0, 0, -10), now)
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO invoices (id, tenant_id, invoice_no, subscriber_id, subscriber_no, customer_name, plan_name, amount, status, due_date, paid_at, created_at, updated_at)
			VALUES (?, ?, 'INV-202610-0002', ?, ?, 'Rian Pratama', 'Home Family 50 Mbps', 250000, 'UNPAID', ?, NULL, ?, ?)
		`, uuid.New().String(), ispkuID, subID, subNo, now.AddDate(0, 0, 15), now, now)

		_, _ = s.GenerateVouchers(ctx, ispkuID, "Paket Warkop 24 Jam", 10, 5, 5000, 12)
	}

	// ── 2. Tenant "ispmu" (PT. ISP Mitra Utama) ─────────────────────────────────
	var ispmuID string
	_ = s.db.QueryRowContext(ctx, "SELECT id FROM tenants WHERE slug = 'ispmu'").Scan(&ispmuID)
	if ispmuID == "" {
		ispmuID = uuid.New().String()
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO tenants (id, slug, name, short_name, prefix_id, logo_url, brand_color, contact_phone, contact_email, address, status, created_at, updated_at)
			VALUES (?, 'ispmu', 'PT. ISP Mitra Utama', 'ISPMU', 'ISPMU', '', '#7c3aed', '081377778888', 'info@ispmu.ispsync.id', 'Jl. Khatib Sulaiman No. 45, Kota Padang, Sumatera Barat', 'ACTIVE', ?, ?)
		`, ispmuID, now, now)
		if err != nil {
			return err
		}

		// Staff
		users := []struct{ u, n, r string }{
			{"owner", "Direktur Utama ISPMU", "OWNER"},
			{"noc", "Lead NOC Bukittinggi", "NOC"},
			{"sales", "Account Manager", "SALES"},
			{"teknisi", "Teknisi Lapangan", "TECHNICIAN"},
		}
		for _, u := range users {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO users (id, tenant_id, username, password_hash, full_name, email, role, status, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, 'ACTIVE', ?)
			`, uuid.New().String(), ispmuID, u.u, string(pwHash), u.n, u.u+"@ispmu.ispsync.id", u.r, now)
		}

		// Plans
		plans := []struct {
			c, n string
			d, u int
			p    float64
			desc string
		}{
			{"ISPMU-30M", "Mitra Starter 30 Mbps", 30, 30, 185000, "Akses internet andal untuk keluarga dan usaha kecil"},
			{"ISPMU-75M", "Mitra Family 75 Mbps", 75, 75, 290000, "Streaming 4K multi-device & gaming latensi rendah"},
			{"ISPMU-150M", "Mitra Bisnis 150 Mbps", 150, 150, 550000, "Bandwidth simetris prioritas tinggi untuk kantor & cafe"},
		}
		for _, p := range plans {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO plans (id, tenant_id, code, name, speed_down_mbps, speed_up_mbps, monthly_price, description, is_active)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
			`, uuid.New().String(), ispmuID, p.c, p.n, p.d, p.u, p.p, p.desc)
		}

		// Native ODPs in Bukittinggi
		odps := []struct {
			c, n string
			lat, lng float64
			ports int
		}{
			{"ODP-BKT-001", "ODP Jam Gadang Heritage", -0.3050, 100.3690, 8},
			{"ODP-BKT-002", "ODP Ngarai Sianok Panorama", -0.3080, 100.3640, 16},
			{"ODP-BKT-003", "ODP Pasar Atas Sentral", -0.3040, 100.3705, 8},
		}
		for _, o := range odps {
			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO odps (id, tenant_id, code, name, latitude, longitude, total_ports, used_ports, status)
				VALUES (?, ?, ?, ?, ?, ?, ?, 0, 'ACTIVE')
			`, uuid.New().String(), ispmuID, o.c, o.n, o.lat, o.lng, o.ports)
		}

		// OLT
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO olts (id, tenant_id, name, vendor, host_ip, port, username, password, status, total_pons)
			VALUES (?, ?, 'OLT-ISPMU-ZTE-C320', 'ZTE', '192.168.20.10', 23, 'admin', 'admin', 'ONLINE', 4)
		`, uuid.New().String(), ispmuID)

		// Subscriber
		subID := uuid.New().String()
		subNo := "ISPMU-2026-0001"
		sn := "ZTEGC9112233"
		mac := "E4:8D:8C:33:44:55"
		rx := -18.75
		vlan := 301
		pppoeU := "sub001@ispmu"
		pppoeP := "mitrapass"
		ip := "10.30.10.25"
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO subscribers (id, tenant_id, subscriber_no, full_name, identity_number, email, phone, address, latitude, longitude, distance_to_odp, selected_plan_name, nearest_odp_code, pon_port, onu_id, serial_number, mac_address, rx_optical_power, pppoe_username, pppoe_password, vlan_id, ip_address, status, activated_at, created_at, updated_at)
			VALUES (?, ?, ?, 'Dedi Kurniawan', '1375012345670002', 'dedi@example.com', '081366554433', 'Jl. Panorama No. 12 Bukittinggi', -0.3082, 100.3642, 28.0, 'Mitra Family 75 Mbps', 'ODP-BKT-002', 'GPON0/1', 1, ?, ?, ?, ?, ?, ?, ?, 'ACTIVE', ?, ?, ?)
		`, subID, ispmuID, subNo, sn, mac, rx, pppoeU, pppoeP, vlan, ip, now, now, now)

		// Invoices & Vouchers for ispmu
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO invoices (id, tenant_id, invoice_no, subscriber_id, subscriber_no, customer_name, plan_name, amount, status, due_date, paid_at, created_at, updated_at)
			VALUES (?, ?, 'INV-ISPMU-202609-0001', ?, ?, 'Dedi Kurniawan', 'Mitra Family 75 Mbps', 290000, 'PAID', ?, ?, ?, ?)
		`, uuid.New().String(), ispmuID, subID, subNo, now.AddDate(0, 0, -3), now.AddDate(0, 0, -1), now.AddDate(0, 0, -8), now)
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO invoices (id, tenant_id, invoice_no, subscriber_id, subscriber_no, customer_name, plan_name, amount, status, due_date, paid_at, created_at, updated_at)
			VALUES (?, ?, 'INV-ISPMU-202610-0002', ?, ?, 'Dedi Kurniawan', 'Mitra Family 75 Mbps', 290000, 'UNPAID', ?, NULL, ?, ?)
		`, uuid.New().String(), ispmuID, subID, subNo, now.AddDate(0, 0, 15), now, now)

		_, _ = s.GenerateVouchers(ctx, ispmuID, "Paket Cafe Jam Gadang 12 Jam", 15, 10, 5000, 10)
	}

	// ── 3. Kerjasama Jartaplok Bilateral (ISPKU Provider -> ISPMU Client) ───────
	var countJartaplok int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jartaplok_agreements WHERE agreement_no = 'JARTAPLOK-PYK-2026-01'").Scan(&countJartaplok)
	if countJartaplok == 0 && ispkuID != "" && ispmuID != "" {
		agID := uuid.New().String()
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO jartaplok_agreements (id, agreement_no, provider_tenant_id, client_tenant_id, scope_area, total_shared_odps, allocated_ports, used_ports, settlement_rate_per_port, status, created_at)
			VALUES (?, 'JARTAPLOK-PYK-2026-01', ?, ?, 'Koridor Payakumbuh Metro & Simpang Benteng', 2, 16, 2, 25000, 'ACTIVE', ?)
		`, agID, ispkuID, ispmuID, now)

		// Link ODP-PYK-001 & ODP-PYK-002 from ispku into shared ODPs
		var odp1, odp2 string
		_ = s.db.QueryRowContext(ctx, "SELECT id FROM odps WHERE tenant_id = ? AND code = 'ODP-PYK-001'", ispkuID).Scan(&odp1)
		_ = s.db.QueryRowContext(ctx, "SELECT id FROM odps WHERE tenant_id = ? AND code = 'ODP-PYK-002'", ispkuID).Scan(&odp2)
		if odp1 != "" {
			_, _ = s.db.ExecContext(ctx, "INSERT OR IGNORE INTO jartaplok_shared_odps (id, agreement_id, odp_id, allocated_ports, used_ports) VALUES (?, ?, ?, 8, 1)", uuid.New().String(), agID, odp1)
		}
		if odp2 != "" {
			_, _ = s.db.ExecContext(ctx, "INSERT OR IGNORE INTO jartaplok_shared_odps (id, agreement_id, odp_id, allocated_ports, used_ports) VALUES (?, ?, ?, 8, 1)", uuid.New().String(), agID, odp2)
		}
	}

	return nil
}

// ── Tenant Methods ─────────────────────────────────────────────────────────────

func (s *SQLiteStorage) GetTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, short_name, prefix_id, logo_url, brand_color, contact_phone, contact_email, address, custom_domain, COALESCE(base_staff_quota, 3), status, created_at, updated_at
		FROM tenants WHERE slug = ?
	`, strings.ToLower(slug))

	var t domain.Tenant
	err := row.Scan(&t.ID, &t.Slug, &t.Name, &t.ShortName, &t.PrefixID, &t.LogoURL, &t.BrandColor, &t.ContactPhone, &t.ContactEmail, &t.Address, &t.CustomDomain, &t.BaseStaffQuota, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *SQLiteStorage) GetTenantByCustomDomain(ctx context.Context, domainName string) (*domain.Tenant, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, short_name, prefix_id, logo_url, brand_color, contact_phone, contact_email, address, custom_domain, COALESCE(base_staff_quota, 3), status, created_at, updated_at
		FROM tenants WHERE custom_domain = ?
	`, strings.ToLower(domainName))

	var t domain.Tenant
	err := row.Scan(&t.ID, &t.Slug, &t.Name, &t.ShortName, &t.PrefixID, &t.LogoURL, &t.BrandColor, &t.ContactPhone, &t.ContactEmail, &t.Address, &t.CustomDomain, &t.BaseStaffQuota, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *SQLiteStorage) ListTenants(ctx context.Context) ([]domain.Tenant, error) {
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

func (s *SQLiteStorage) CreateTenant(ctx context.Context, tenant *domain.Tenant) error {
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, tenant.ID, strings.ToLower(tenant.Slug), tenant.Name, tenant.ShortName, tenant.PrefixID, tenant.LogoURL, tenant.BrandColor, tenant.ContactPhone, tenant.ContactEmail, tenant.Address, tenant.CustomDomain, tenant.BaseStaffQuota, tenant.Status, tenant.CreatedAt, tenant.UpdatedAt)
	return err
}

// ── User Methods ───────────────────────────────────────────────────────────────

func (s *SQLiteStorage) GetUserByUsername(ctx context.Context, tenantID, username string) (*domain.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, username, password_hash, full_name, email, phone, role, status, created_at
		FROM users WHERE tenant_id = ? AND username = ?
	`, tenantID, username)

	var u domain.User
	err := row.Scan(&u.ID, &u.TenantID, &u.Username, &u.PasswordHash, &u.FullName, &u.Email, &u.Phone, &u.Role, &u.Status, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *SQLiteStorage) ListUsersByTenant(ctx context.Context, tenantID string) ([]domain.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, username, password_hash, full_name, email, phone, role, status, created_at
		FROM users WHERE tenant_id = ? ORDER BY created_at ASC
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

func (s *SQLiteStorage) CreateUser(ctx context.Context, user *domain.User, rawPassword string) error {
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.TenantID, user.Username, user.PasswordHash, user.FullName, user.Email, user.Phone, strings.ToUpper(user.Role), user.Status, user.CreatedAt)
	return err
}

// ── Add-ons & Staff Quota ───────────────────────────────────────────────────────

func (s *SQLiteStorage) GetStaffQuotaStatus(ctx context.Context, tenantID string) (*domain.StaffQuotaStatus, error) {
	baseQuota := 3
	var dbBase sql.NullInt32
	_ = s.db.QueryRowContext(ctx, "SELECT base_staff_quota FROM tenants WHERE id = ?", tenantID).Scan(&dbBase)
	if dbBase.Valid && dbBase.Int32 > 0 {
		baseQuota = int(dbBase.Int32)
	}

	var addonQuota sql.NullInt32
	_ = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(quantity), 0) FROM tenant_addons 
		WHERE tenant_id = ? AND status = 'ACTIVE' AND addon_type = 'STAFF_SEAT' 
		AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
	`, tenantID).Scan(&addonQuota)

	var usedStaff sql.NullInt32
	_ = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users 
		WHERE tenant_id = ? AND status = 'ACTIVE' AND UPPER(role) != 'OWNER'
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

func (s *SQLiteStorage) ListTenantAddons(ctx context.Context, tenantID string) ([]domain.TenantAddon, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, addon_code, addon_type, name, quantity, monthly_price, status, expires_at, created_at, updated_at
		FROM tenant_addons WHERE tenant_id = ? ORDER BY created_at DESC
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

func (s *SQLiteStorage) CreateTenantAddon(ctx context.Context, addon *domain.TenantAddon) error {
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, addon.ID, addon.TenantID, addon.AddonCode, addon.AddonType, addon.Name, addon.Quantity, addon.MonthlyPrice, addon.Status, addon.ExpiresAt, addon.CreatedAt, addon.UpdatedAt)
	return err
}


// ── Plan Methods ───────────────────────────────────────────────────────────────

func (s *SQLiteStorage) ListPlans(ctx context.Context, tenantID string) ([]domain.Plan, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, code, name, speed_down_mbps, speed_up_mbps, monthly_price, description, is_active
		FROM plans WHERE tenant_id = ? AND is_active = 1 ORDER BY monthly_price ASC
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

func (s *SQLiteStorage) GetPlanByID(ctx context.Context, tenantID, id string) (*domain.Plan, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, code, name, speed_down_mbps, speed_up_mbps, monthly_price, description, is_active
		FROM plans WHERE tenant_id = ? AND id = ?
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

func (s *SQLiteStorage) ListODPs(ctx context.Context, tenantID string) ([]domain.ODP, error) {
	query := `
		SELECT o.id, o.tenant_id, o.code, o.name, o.latitude, o.longitude, o.total_ports, o.used_ports, o.status, 0 as is_shared, '' as owner_slug, '' as owner_name
		FROM odps o WHERE o.tenant_id = ?
		UNION ALL
		SELECT o.id, o.tenant_id, o.code, '[JARTAPLOK ' || UPPER(tp.slug) || '] ' || o.name as name, o.latitude, o.longitude, s.allocated_ports as total_ports, s.used_ports as used_ports, o.status, 1 as is_shared, tp.slug as owner_slug, tp.name as owner_name
		FROM jartaplok_shared_odps s
		JOIN jartaplok_agreements a ON s.agreement_id = a.id
		JOIN odps o ON s.odp_id = o.id
		JOIN tenants tp ON a.provider_tenant_id = tp.id
		WHERE a.client_tenant_id = ? AND a.status = 'ACTIVE'
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

func (s *SQLiteStorage) ListJartaplokAgreements(ctx context.Context, tenantID string) ([]domain.JartaplokAgreement, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.agreement_no, a.provider_tenant_id, tp.slug, tp.name,
		       a.client_tenant_id, tc.slug, tc.name, a.scope_area,
		       a.total_shared_odps, a.allocated_ports, a.used_ports, a.settlement_rate_per_port, a.status, a.created_at
		FROM jartaplok_agreements a
		JOIN tenants tp ON a.provider_tenant_id = tp.id
		JOIN tenants tc ON a.client_tenant_id = tc.id
		WHERE a.provider_tenant_id = ? OR a.client_tenant_id = ?
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

func (s *SQLiteStorage) GetNearestODP(ctx context.Context, tenantID string, lat, lng float64) (*domain.ODP, float64, error) {
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

func (s *SQLiteStorage) UpsertODP(ctx context.Context, odp *domain.ODP) error {
	if odp.ID == "" {
		odp.ID = uuid.New().String()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO odps (id, tenant_id, code, name, latitude, longitude, total_ports, used_ports, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, code) DO UPDATE SET
			name = excluded.name,
			latitude = excluded.latitude,
			longitude = excluded.longitude,
			total_ports = excluded.total_ports,
			used_ports = excluded.used_ports,
			status = excluded.status
	`, odp.ID, odp.TenantID, odp.Code, odp.Name, odp.Latitude, odp.Longitude, odp.TotalPorts, odp.UsedPorts, odp.Status)
	return err
}

// ── OLT Methods ────────────────────────────────────────────────────────────────

func (s *SQLiteStorage) ListOLTs(ctx context.Context, tenantID string) ([]domain.OLT, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, name, vendor, host_ip, port, username, status, total_pons
		FROM olts WHERE tenant_id = ? ORDER BY name ASC
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

func (s *SQLiteStorage) GetOLTByID(ctx context.Context, tenantID, id string) (*domain.OLT, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, name, vendor, host_ip, port, username, password, status, total_pons
		FROM olts WHERE tenant_id = ? AND id = ?
	`, tenantID, id)

	var o domain.OLT
	if err := row.Scan(&o.ID, &o.TenantID, &o.Name, &o.Vendor, &o.HostIP, &o.Port, &o.Username, &o.Password, &o.Status, &o.TotalPONs); err != nil {
		return nil, err
	}
	return &o, nil
}

// ── Subscriber Methods ─────────────────────────────────────────────────────────

func (s *SQLiteStorage) ListSubscribers(ctx context.Context, tenantID string, status string) ([]domain.Subscriber, error) {
	query := `
		SELECT id, tenant_id, subscriber_no, full_name, identity_number, email, phone, address,
		       latitude, longitude, distance_to_odp, selected_plan_id, selected_plan_name,
		       nearest_odp_id, nearest_odp_code, olt_id, pon_port, onu_id, serial_number, mac_address,
		       rx_optical_power, pppoe_username, pppoe_password, vlan_id, ip_address, status,
		       activated_at, suspended_at, created_at, updated_at
		FROM subscribers WHERE tenant_id = ?
	`
	args := []interface{}{tenantID}
	if status != "" {
		query += " AND status = ?"
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

func (s *SQLiteStorage) GetSubscriberByID(ctx context.Context, tenantID, id string) (*domain.Subscriber, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, subscriber_no, full_name, identity_number, email, phone, address,
		       latitude, longitude, distance_to_odp, selected_plan_id, selected_plan_name,
		       nearest_odp_id, nearest_odp_code, olt_id, pon_port, onu_id, serial_number, mac_address,
		       rx_optical_power, pppoe_username, pppoe_password, vlan_id, ip_address, status,
		       activated_at, suspended_at, created_at, updated_at
		FROM subscribers WHERE tenant_id = ? AND (id = ? OR subscriber_no = ?)
	`, tenantID, id, id)

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

func (s *SQLiteStorage) GetSubscriberByNo(ctx context.Context, tenantID, subNo string) (*domain.Subscriber, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, subscriber_no, full_name, identity_number, email, phone, address,
		       latitude, longitude, distance_to_odp, selected_plan_id, selected_plan_name,
		       nearest_odp_id, nearest_odp_code, olt_id, pon_port, onu_id, serial_number, mac_address,
		       rx_optical_power, pppoe_username, pppoe_password, vlan_id, ip_address, status,
		       activated_at, suspended_at, created_at, updated_at
		FROM subscribers WHERE tenant_id = ? AND subscriber_no = ?
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

func (s *SQLiteStorage) CreateSubscriber(ctx context.Context, sub *domain.Subscriber) error {
	now := time.Now()
	if sub.ID == "" {
		sub.ID = uuid.New().String()
	}
	sub.CreatedAt = now
	sub.UpdatedAt = now

	// Auto generate SubscriberNo jika kosong
	if sub.SubscriberNo == "" {
		var count int
		_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM subscribers WHERE tenant_id = ?", sub.TenantID).Scan(&count)
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
			?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?
		)
	`, sub.ID, sub.TenantID, sub.SubscriberNo, sub.FullName, sub.IdentityNumber, sub.Email, sub.Phone, sub.Address,
		sub.Latitude, sub.Longitude, sub.DistanceToODP, sub.SelectedPlanID, sub.SelectedPlanName,
		sub.NearestODPID, sub.NearestODPCode, sub.OLTID, sub.PONPort, sub.ONUID, sub.SerialNumber, sub.MACAddress,
		sub.RxOpticalPower, sub.PPPoEUsername, sub.PPPoEPassword, sub.VLANID, sub.IPAddress, sub.Status,
		sub.CreatedAt, sub.UpdatedAt)
	return err
}

func (s *SQLiteStorage) UpdateSubscriberStatus(ctx context.Context, tenantID, id, status string) error {
	now := time.Now()
	var actTime, suspTime *time.Time
	if status == "ACTIVE" {
		actTime = &now
	} else if status == "RESTRICTED" || status == "SUSPENDED" {
		suspTime = &now
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE subscribers SET status = ?, activated_at = COALESCE(?, activated_at), suspended_at = COALESCE(?, suspended_at), updated_at = ?
		WHERE tenant_id = ? AND (id = ? OR subscriber_no = ?)
	`, status, actTime, suspTime, now, tenantID, id, id)
	return err
}

func (s *SQLiteStorage) DeleteSubscriber(ctx context.Context, tenantID, idOrNo string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM subscribers
		WHERE tenant_id = ? AND (id = ? OR subscriber_no = ?)
	`, tenantID, idOrNo, idOrNo)
	return err
}

func (s *SQLiteStorage) UpdateSubscriberProvisioning(ctx context.Context, tenantID, id string, oltID *string, ponPort *string, onuID *int, sn, mac *string, rxPower *float64, pppoeUser, pppoePass *string, vlan *int, ip *string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE subscribers SET
			olt_id = COALESCE(?, olt_id),
			pon_port = COALESCE(?, pon_port),
			onu_id = COALESCE(?, onu_id),
			serial_number = COALESCE(?, serial_number),
			mac_address = COALESCE(?, mac_address),
			rx_optical_power = COALESCE(?, rx_optical_power),
			pppoe_username = COALESCE(?, pppoe_username),
			pppoe_password = COALESCE(?, pppoe_password),
			vlan_id = COALESCE(?, vlan_id),
			ip_address = COALESCE(?, ip_address),
			updated_at = ?
		WHERE tenant_id = ? AND id = ?
	`, oltID, ponPort, onuID, sn, mac, rxPower, pppoeUser, pppoePass, vlan, ip, now, tenantID, id)
	return err
}

// ── Work Order Methods ─────────────────────────────────────────────────────────

func (s *SQLiteStorage) ListWorkOrders(ctx context.Context, tenantID string, status string) ([]domain.WorkOrder, error) {
	query := `
		SELECT id, tenant_id, order_no, subscriber_id, subscriber_no, customer_name, customer_phone, customer_address,
		       customer_lat, customer_lng, odp_code, plan_name, order_type, technician_id, technician_name, status,
		       rx_power_dbm, serial_number, mac_address, notes, bast_completed_at, created_at, updated_at
		FROM work_orders WHERE tenant_id = ?
	`
	args := []interface{}{tenantID}
	if status != "" {
		query += " AND status = ?"
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

func (s *SQLiteStorage) GetWorkOrderByID(ctx context.Context, tenantID, id string) (*domain.WorkOrder, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, order_no, subscriber_id, subscriber_no, customer_name, customer_phone, customer_address,
		       customer_lat, customer_lng, odp_code, plan_name, order_type, technician_id, technician_name, status,
		       rx_power_dbm, serial_number, mac_address, notes, bast_completed_at, created_at, updated_at
		FROM work_orders WHERE tenant_id = ? AND id = ?
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

func (s *SQLiteStorage) CreateWorkOrder(ctx context.Context, wo *domain.WorkOrder) error {
	now := time.Now()
	if wo.ID == "" {
		wo.ID = uuid.New().String()
	}
	wo.CreatedAt = now
	wo.UpdatedAt = now

	if wo.OrderNo == "" {
		var count int
		_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM work_orders WHERE tenant_id = ?", wo.TenantID).Scan(&count)
		wo.OrderNo = fmt.Sprintf("SPK-%s-%04d", time.Now().Format("2006"), count+1)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO work_orders (
			id, tenant_id, order_no, subscriber_id, subscriber_no, customer_name, customer_phone, customer_address,
			customer_lat, customer_lng, odp_code, plan_name, order_type, technician_id, technician_name, status,
			rx_power_dbm, serial_number, mac_address, notes, created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?
		)
	`, wo.ID, wo.TenantID, wo.OrderNo, wo.SubscriberID, wo.SubscriberNo, wo.CustomerName, wo.CustomerPhone, wo.CustomerAddress,
		wo.CustomerLat, wo.CustomerLng, wo.ODPCode, wo.PlanName, wo.OrderType, wo.TechnicianID, wo.TechnicianName, wo.Status,
		wo.RxPowerDBM, wo.SerialNumber, wo.MACAddress, wo.Notes, wo.CreatedAt, wo.UpdatedAt)
	return err
}

func (s *SQLiteStorage) CompleteWorkOrderBAST(ctx context.Context, tenantID, id string, rxPower float64, sn, mac, notes string) error {
	now := time.Now()
	// Update WO
	res, err := s.db.ExecContext(ctx, `
		UPDATE work_orders SET
			status = 'COMPLETED',
			rx_power_dbm = ?,
			serial_number = ?,
			mac_address = ?,
			notes = ?,
			bast_completed_at = ?,
			updated_at = ?
		WHERE tenant_id = ? AND id = ?
	`, rxPower, sn, mac, notes, now, now, tenantID, id)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("work order not found or unauthorized")
	}

	// Ambil Subscriber ID dari WO untuk auto-promote ke ACTIVE
	var subID string
	_ = s.db.QueryRowContext(ctx, "SELECT subscriber_id FROM work_orders WHERE tenant_id = ? AND id = ?", tenantID, id).Scan(&subID)
	if subID != "" {
		_ = s.UpdateSubscriberProvisioning(ctx, tenantID, subID, nil, nil, nil, &sn, &mac, &rxPower, nil, nil, nil, nil)
		_ = s.UpdateSubscriberStatus(ctx, tenantID, subID, "ACTIVE")
	}

	return nil
}

// ── Haversine Distance Helper ──────────────────────────────────────────────────

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // Earth radius in meters
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaPhi := (lat2 - lat1) * math.Pi / 180
	deltaLambda := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// ── Custom Domain & TLS Check ──────────────────────────────────────────────────

func (s *SQLiteStorage) UpdateTenantCustomDomain(ctx context.Context, tenantID, customDomain string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE tenants SET custom_domain = ?, updated_at = ? WHERE id = ?", strings.ToLower(strings.TrimSpace(customDomain)), time.Now(), tenantID)
	return err
}

func (s *SQLiteStorage) UpdateTenantProfile(ctx context.Context, tenantID, logoUrl, brandColor, contactPhone, contactEmail string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE tenants SET logo_url = ?, brand_color = ?, contact_phone = ?, contact_email = ?, updated_at = ? WHERE id = ?", strings.TrimSpace(logoUrl), strings.TrimSpace(brandColor), strings.TrimSpace(contactPhone), strings.TrimSpace(contactEmail), time.Now(), tenantID)
	return err
}

func (s *SQLiteStorage) CreateMikrotikRouter(ctx context.Context, r *domain.MikrotikRouter) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mikrotik_routers (id, tenant_id, name, wg_pubkey, wg_ip, api_port, api_user, api_password, status, last_seen, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id) DO UPDATE SET 
		wg_pubkey=excluded.wg_pubkey, wg_ip=excluded.wg_ip, api_password=excluded.api_password, updated_at=excluded.updated_at
	`, r.ID, r.TenantID, r.Name, r.WgPubkey, r.WgIP, r.APIPort, r.APIUser, r.APIPassword, r.Status, r.LastSeen, r.CreatedAt, r.UpdatedAt)
	return err
}

func (s *SQLiteStorage) GetMikrotikRouter(ctx context.Context, tenantID string) (*domain.MikrotikRouter, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id, tenant_id, name, wg_pubkey, wg_ip, api_port, api_user, api_password, status, last_seen, created_at, updated_at FROM mikrotik_routers WHERE tenant_id = ?", tenantID)
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

func (s *SQLiteStorage) ValidateDomainForTLS(ctx context.Context, domainName string) bool {
	d := strings.ToLower(strings.TrimSpace(domainName))
	if d == "" {
		return false
	}
	// Reject non-tenant subdomains that have been deactivated
	if d == "cms.ispsync.id" {
		return false
	}
	if d == "ispsync.id" || strings.HasSuffix(d, ".ispsync.id") {
		return true
	}
	var count int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenants WHERE custom_domain = ? AND status = 'ACTIVE'", d).Scan(&count)
	return count > 0
}

// ── Invoicing Methods ──────────────────────────────────────────────────────────

func (s *SQLiteStorage) ListInvoices(ctx context.Context, tenantID string, status string) ([]domain.Invoice, error) {
	query := `SELECT id, tenant_id, invoice_no, subscriber_id, subscriber_no, customer_name, plan_name, amount, status, due_date, paid_at, created_at, updated_at
	          FROM invoices WHERE tenant_id = ?`
	args := []interface{}{tenantID}
	if status != "" {
		query += " AND status = ?"
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

func (s *SQLiteStorage) CreateInvoice(ctx context.Context, inv *domain.Invoice) error {
	now := time.Now()
	if inv.ID == "" {
		inv.ID = uuid.New().String()
	}
	if inv.InvoiceNo == "" {
		var count int
		_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM invoices WHERE tenant_id = ?", inv.TenantID).Scan(&count)
		inv.InvoiceNo = fmt.Sprintf("INV-%s-%04d", time.Now().Format("200601"), count+1)
	}
	inv.CreatedAt = now
	inv.UpdatedAt = now

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO invoices (id, tenant_id, invoice_no, subscriber_id, subscriber_no, customer_name, plan_name, amount, status, due_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, inv.ID, inv.TenantID, inv.InvoiceNo, inv.SubscriberID, inv.SubscriberNo, inv.CustomerName, inv.PlanName, inv.Amount, inv.Status, inv.DueDate, inv.CreatedAt, inv.UpdatedAt)
	return err
}

func (s *SQLiteStorage) MarkInvoicePaid(ctx context.Context, tenantID, invoiceID string) error {
	now := time.Now()
	var subID string
	err := s.db.QueryRowContext(ctx, "SELECT subscriber_id FROM invoices WHERE tenant_id = ? AND id = ?", tenantID, invoiceID).Scan(&subID)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE invoices SET status = 'PAID', paid_at = ?, updated_at = ? WHERE tenant_id = ? AND id = ?
	`, now, now, tenantID, invoiceID)
	if err != nil {
		return err
	}

	_ = s.UpdateSubscriberStatus(ctx, tenantID, subID, "ACTIVE")
	return nil
}

// ── Voucher Methods ────────────────────────────────────────────────────────────

func (s *SQLiteStorage) ListVouchers(ctx context.Context, tenantID string) ([]domain.Voucher, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, batch_id, code, password, price, profile_name, status, used_at, created_at
		FROM vouchers WHERE tenant_id = ? ORDER BY created_at DESC LIMIT 100
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

func (s *SQLiteStorage) GenerateVouchers(ctx context.Context, tenantID string, profileName string, speedDown, speedUp int, price float64, count int) (*domain.VoucherBatch, error) {
	now := time.Now()
	batchID := uuid.New().String()
	var bCount int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM voucher_batches WHERE tenant_id = ?", tenantID).Scan(&bCount)
	batchNo := fmt.Sprintf("BATCH-%s-%03d", time.Now().Format("200601"), bCount+1)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO voucher_batches (id, tenant_id, batch_no, profile_name, speed_down_mbps, speed_up_mbps, price, total_vouchers, used_vouchers, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?)
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
			VALUES (?, ?, ?, ?, ?, ?, ?, 'AVAILABLE', ?)
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

func (s *SQLiteStorage) GetTenantCapabilities(ctx context.Context, tenantID string) (domain.TenantCapabilities, error) {
	caps := domain.TenantCapabilities{TenantID: tenantID}
	err := s.db.QueryRowContext(ctx, "SELECT uses_fibergrid, own_infrastructure FROM tenant_capabilities WHERE tenant_id = ?", tenantID).Scan(&caps.UsesFiberGrid, &caps.OwnInfrastructure)
	if err == sql.ErrNoRows {
		return caps, nil
	}
	return caps, err
}

func (s *SQLiteStorage) SetTenantCapabilities(ctx context.Context, caps domain.TenantCapabilities) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO tenant_capabilities (tenant_id, uses_fibergrid, own_infrastructure, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT (tenant_id) DO UPDATE SET uses_fibergrid = excluded.uses_fibergrid, own_infrastructure = excluded.own_infrastructure, updated_at = excluded.updated_at", caps.TenantID, caps.UsesFiberGrid, caps.OwnInfrastructure)
	return err
}

func (s *SQLiteStorage) UpdateUserStatus(ctx context.Context, tenantID, userID, status string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET status = ? WHERE tenant_id = ? AND id = ?", strings.ToUpper(status), tenantID, userID)
	return err
}

func (s *SQLiteStorage) ResetUserPassword(ctx context.Context, tenantID, username, newPassword string) error {
	pwHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE tenant_id = ? AND LOWER(username) = LOWER(?)", string(pwHash), tenantID, username)
	return err
}
