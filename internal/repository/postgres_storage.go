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
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
	"isp-onboarding/internal/domain"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	s := &PostgresStorage{db: db}
	if err := s.initSchema(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize postgres schema: %w", err)
	}

	return s, nil
}

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

func (s *PostgresStorage) initSchema(ctx context.Context) error {
	queries := []string{
		// Extensions
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`,
		`CREATE EXTENSION IF NOT EXISTS "citext";`,
		`CREATE EXTENSION IF NOT EXISTS "postgis";`,

		// 1. Partners
		`CREATE TABLE IF NOT EXISTS partners (
			id VARCHAR(64) PRIMARY KEY,
			code VARCHAR(64) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			api_key VARCHAR(128) UNIQUE NOT NULL,
			commission_rate DOUBLE PRECISION NOT NULL DEFAULT 0.0,
			contact_phone VARCHAR(64) NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		// 2. Jartaplok Partners
		`CREATE TABLE IF NOT EXISTS jartaplok_partners (
			id VARCHAR(64) PRIMARY KEY,
			code VARCHAR(64) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			api_key VARCHAR(128) UNIQUE NOT NULL,
			contact_phone VARCHAR(64) NOT NULL,
			coverage_area TEXT NOT NULL,
			service_type VARCHAR(64) NOT NULL DEFAULT 'SEWA_PORT_FO',
			suspension_policy VARCHAR(64) NOT NULL DEFAULT 'ALLOWED_WITH_WAIVER',
			pricing_model VARCHAR(128) NOT NULL DEFAULT 'Sewa Port FO Pasif (Jartaplok)',
			rate_20m INTEGER NOT NULL DEFAULT 0,
			rate_30m INTEGER NOT NULL DEFAULT 0,
			rate_40m INTEGER NOT NULL DEFAULT 0,
			rate_50m INTEGER NOT NULL DEFAULT 50000,
			rate_100m INTEGER NOT NULL DEFAULT 90000,
			rate_150m INTEGER NOT NULL DEFAULT 135000,
			rate_200m INTEGER NOT NULL DEFAULT 180000,
			rate_300m INTEGER NOT NULL DEFAULT 270000,
			otc_fee INTEGER NOT NULL DEFAULT 0,
			max_distance_meters DOUBLE PRECISION NOT NULL DEFAULT 250.0,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		// 3. ODP Nodes with GIS PostGIS Point
		`CREATE TABLE IF NOT EXISTS odp_nodes (
			id VARCHAR(64) PRIMARY KEY,
			code VARCHAR(64) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			latitude DOUBLE PRECISION NOT NULL,
			longitude DOUBLE PRECISION NOT NULL,
			geom GEOMETRY(Point, 4326),
			total_ports INTEGER NOT NULL DEFAULT 8,
			used_ports INTEGER NOT NULL DEFAULT 0,
			status VARCHAR(32) NOT NULL DEFAULT 'AVAILABLE',
			cluster_area VARCHAR(100) NOT NULL DEFAULT '',
			provider_id VARCHAR(64) NOT NULL DEFAULT 'GNET-BIARO',
			provider_name VARCHAR(100) NOT NULL DEFAULT 'PT. GNET BIARO AKSES',
			is_cluster_active BOOLEAN NOT NULL DEFAULT TRUE,
			splitter_spec VARCHAR(50) NOT NULL DEFAULT '1:8 PLC',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_odp_nodes_geom ON odp_nodes USING GIST(geom);`,
		`CREATE INDEX IF NOT EXISTS idx_odp_nodes_code ON odp_nodes(code);`,
		`CREATE INDEX IF NOT EXISTS idx_odp_nodes_provider ON odp_nodes(provider_id);`,

		// Trigger for ODP geom
		`CREATE OR REPLACE FUNCTION update_odp_geom() RETURNS TRIGGER AS $$
		BEGIN
			NEW.geom := ST_SetSRID(ST_MakePoint(NEW.longitude, NEW.latitude), 4326);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;`,
		`DROP TRIGGER IF EXISTS trg_odp_geom ON odp_nodes;`,
		`CREATE TRIGGER trg_odp_geom
		BEFORE INSERT OR UPDATE OF latitude, longitude ON odp_nodes
		FOR EACH ROW EXECUTE FUNCTION update_odp_geom();`,

		// 4. Fiber Routes (Kabel FO GIS)
		`CREATE TABLE IF NOT EXISTS fiber_routes (
			id VARCHAR(64) PRIMARY KEY DEFAULT gen_random_uuid()::text,
			route_code VARCHAR(64) UNIQUE NOT NULL,
			route_name VARCHAR(255) NOT NULL,
			cable_type VARCHAR(32) NOT NULL DEFAULT 'DISTRIBUTION',
			core_count INTEGER NOT NULL DEFAULT 12,
			geom GEOMETRY(LineString, 4326),
			start_node_id VARCHAR(64),
			end_node_id VARCHAR(64),
			cluster_area VARCHAR(100) NOT NULL DEFAULT '',
			status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
			color_hex VARCHAR(16) NOT NULL DEFAULT '#10b981',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_fiber_routes_geom ON fiber_routes USING GIST(geom);`,

		// 5. Registrations
		`CREATE TABLE IF NOT EXISTS registrations (
			id VARCHAR(64) PRIMARY KEY,
			registration_no VARCHAR(64) UNIQUE NOT NULL,
			partner_id VARCHAR(64),
			partner_code VARCHAR(64),
			full_name VARCHAR(255) NOT NULL,
			email VARCHAR(255),
			phone VARCHAR(50) NOT NULL,
			id_card_number VARCHAR(64) NOT NULL,
			tax_id VARCHAR(64) DEFAULT '',
			address TEXT NOT NULL,
			latitude DOUBLE PRECISION NOT NULL,
			longitude DOUBLE PRECISION NOT NULL,
			geom GEOMETRY(Point, 4326),
			selected_plan_id VARCHAR(64) NOT NULL,
			selected_plan_name VARCHAR(255) NOT NULL,
			nearest_odp_id VARCHAR(64),
			nearest_odp_code VARCHAR(64),
			distance_to_odp_meters DOUBLE PRECISION NOT NULL,
			status VARCHAR(64) NOT NULL DEFAULT 'SUBMITTED',
			ktp_photo_url TEXT,
			house_photo_url TEXT,
			site_pic_name VARCHAR(255),
			site_pic_phone VARCHAR(50),
			contract_signature_url TEXT,
			contract_signed_at TIMESTAMPTZ,
			activated_at TIMESTAMPTZ,
			suspended_at TIMESTAMPTZ,
			suspension_reason TEXT,
			gigabill_customer_id VARCHAR(64),
			gigabill_subscription_id VARCHAR(64),
			dispatch_notes TEXT,
			custom_notes TEXT,
			otc_fee BIGINT DEFAULT 0,
			monthly_price BIGINT DEFAULT 0,
			otc_notes TEXT,
			pppoe_username VARCHAR(128) DEFAULT '',
			pppoe_password VARCHAR(128) DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_registrations_geom ON registrations USING GIST(geom);`,
		`CREATE INDEX IF NOT EXISTS idx_registrations_regno ON registrations(registration_no);`,
		`CREATE INDEX IF NOT EXISTS idx_registrations_phone ON registrations(phone);`,
		`CREATE INDEX IF NOT EXISTS idx_registrations_pppoe ON registrations(pppoe_username);`,

		// Trigger for registration geom
		`CREATE OR REPLACE FUNCTION update_registration_geom() RETURNS TRIGGER AS $$
		BEGIN
			NEW.geom := ST_SetSRID(ST_MakePoint(NEW.longitude, NEW.latitude), 4326);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;`,
		`DROP TRIGGER IF EXISTS trg_registration_geom ON registrations;`,
		`CREATE TRIGGER trg_registration_geom
		BEFORE INSERT OR UPDATE OF latitude, longitude ON registrations
		FOR EACH ROW EXECUTE FUNCTION update_registration_geom();`,

		// 6. Work Orders
		`CREATE TABLE IF NOT EXISTS work_orders (
			id VARCHAR(64) PRIMARY KEY,
			order_no VARCHAR(64) UNIQUE NOT NULL,
			registration_id VARCHAR(64) NOT NULL REFERENCES registrations(id) ON DELETE CASCADE,
			type VARCHAR(64) NOT NULL,
			technician_name VARCHAR(255) NOT NULL,
			scheduled_at TIMESTAMPTZ NOT NULL,
			status VARCHAR(64) NOT NULL DEFAULT 'ASSIGNED',
			notes TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_work_orders_reg ON work_orders(registration_id);`,

		// 7. BAST Reports
		`CREATE TABLE IF NOT EXISTS bast_reports (
			id VARCHAR(64) PRIMARY KEY,
			work_order_id VARCHAR(64) UNIQUE NOT NULL REFERENCES work_orders(id) ON DELETE CASCADE,
			optical_power_dbm DOUBLE PRECISION NOT NULL,
			ont_serial_number VARCHAR(64) NOT NULL,
			ont_mac_address VARCHAR(64) NOT NULL,
			dropcore_length_meters INTEGER NOT NULL,
			customer_signature_url TEXT,
			proof_photo_url TEXT,
			house_photo_url TEXT,
			speedtest_down_mbps DOUBLE PRECISION NOT NULL,
			speedtest_up_mbps DOUBLE PRECISION NOT NULL,
			notes TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		// 8. Staff Users
		`CREATE TABLE IF NOT EXISTS staff_users (
			id VARCHAR(64) PRIMARY KEY,
			username VARCHAR(64) UNIQUE NOT NULL,
			password_hash TEXT NOT NULL DEFAULT '',
			full_name VARCHAR(255) NOT NULL,
			role VARCHAR(64) NOT NULL,
			contact_phone VARCHAR(50) NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		// 9. Auth Sessions
		`CREATE TABLE IF NOT EXISTS auth_sessions (
			token VARCHAR(128) PRIMARY KEY,
			user_id VARCHAR(64) NOT NULL,
			username VARCHAR(64) NOT NULL,
			role VARCHAR(64) NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`ALTER TABLE registrations ADD COLUMN IF NOT EXISTS upstream_pppoe_username VARCHAR(128) DEFAULT '';`,
		`ALTER TABLE registrations ADD COLUMN IF NOT EXISTS upstream_pppoe_password VARCHAR(128) DEFAULT '';`,
		`ALTER TABLE bast_reports ADD COLUMN IF NOT EXISTS upstream_pppoe_username VARCHAR(128) DEFAULT '';`,
		`ALTER TABLE bast_reports ADD COLUMN IF NOT EXISTS upstream_pppoe_password VARCHAR(128) DEFAULT '';`,
		`ALTER TABLE partners ADD COLUMN IF NOT EXISTS branch_id VARCHAR(64);`,
		`ALTER TABLE partners ADD COLUMN IF NOT EXISTS branch_code VARCHAR(64);`,
		`ALTER TABLE jartaplok_partners ADD COLUMN IF NOT EXISTS branch_id VARCHAR(64);`,
		`ALTER TABLE jartaplok_partners ADD COLUMN IF NOT EXISTS branch_code VARCHAR(64);`,
		`ALTER TABLE staff_users ADD COLUMN IF NOT EXISTS roles TEXT DEFAULT '[]';`,
		`ALTER TABLE staff_users ADD COLUMN IF NOT EXISTS is_superuser INTEGER DEFAULT 0;`,
		`ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS roles TEXT DEFAULT '[]';`,
		`ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS is_superuser INTEGER DEFAULT 0;`,
		`ALTER TABLE registrations ADD COLUMN IF NOT EXISTS ont_serial_number VARCHAR(64) DEFAULT '';`,
		`ALTER TABLE registrations ADD COLUMN IF NOT EXISTS ont_optical_power DOUBLE PRECISION DEFAULT 0;`,
		`ALTER TABLE registrations ADD COLUMN IF NOT EXISTS ont_status VARCHAR(64) DEFAULT '';`,
	}

	for _, q := range queries {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			// Don't fail hard on postgis if permission is restricted, but try best effort
			if strings.Contains(q, "postgis") {
				continue
			}
			return fmt.Errorf("exec query error: %w (query: %s)", err, q)
		}
	}

	return s.seedDefaultData(ctx)
}

func (s *PostgresStorage) seedDefaultData(ctx context.Context) error {
	// Seed default Jartaplok Partner (GNET-BIARO)
	var count int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jartaplok_partners WHERE code = 'GNET-BIARO'`).Scan(&count)
	if count == 0 {
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO jartaplok_partners (
				id, code, name, api_key, contact_phone, coverage_area,
				service_type, suspension_policy, pricing_model, rate_20m, rate_30m, rate_40m,
				rate_50m, rate_100m, rate_150m, rate_200m, rate_300m, otc_fee, max_distance_meters,
				is_active, created_at, updated_at
			) VALUES (
				'gnet-biaro-001', 'GNET-BIARO', 'PT. GNET BIARO AKSES', 'jartaplok2026', '085186866164', 'Bukittinggi, Biaro & Agam',
				'SEWA_PORT_FO', 'ALLOWED_WITH_WAIVER', 'Sewa Port FO Pasif (Jartaplok)', 0, 0, 0,
				50000, 90000, 135000, 180000, 270000, 0, 250.0,
				TRUE, NOW(), NOW()
			) ON CONFLICT (code) DO NOTHING;
		`)
	}

	// Seed PT Telkom Infrastruktur Indonesia
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jartaplok_partners WHERE code = 'TELKO-PYK'`).Scan(&count)
	if count == 0 {
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO jartaplok_partners (
				id, code, name, api_key, contact_phone, coverage_area,
				service_type, suspension_policy, pricing_model, rate_20m, rate_30m, rate_40m,
				rate_50m, rate_100m, rate_150m, rate_200m, rate_300m, otc_fee, max_distance_meters,
				is_active, created_at, updated_at
			) VALUES (
				'tif-pyk-001', 'TELKO-PYK', 'PT Telkom Infrastruktur Indonesia', 'jartap_telko_pyk_260922', '08126789000', 'Payakumbuh, Lima Puluh Kota & Sekitarnya',
				'BITSTREAM', 'DISALLOWED', 'Bitstream Intra Standard Symetric 1:1 (Zona-2 Sumatera)', 154000, 169000, 184000,
				200000, 270000, 360000, 450000, 600000, 500000, 150.0,
				TRUE, NOW(), NOW()
			) ON CONFLICT (code) DO NOTHING;
		`)
	}

	// Seed default superuser and staff multi-role accounts
	hashOwner, _ := bcrypt.GenerateFromPassword([]byte("Owner@GoGiga2026!"), bcrypt.DefaultCost)
	hashNoc, _ := bcrypt.GenerateFromPassword([]byte("Noc@GoGiga2026!"), bcrypt.DefaultCost)
	hashFinance, _ := bcrypt.GenerateFromPassword([]byte("Finance@GoGiga2026!"), bcrypt.DefaultCost)

	// Idempotently update passwords for existing records if empty or if matching old default
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET full_name = 'Aan Rizal S.Kom (Direktur Utama / Owner)' WHERE username = 'owner'`)
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET password_hash = $1 WHERE username = 'owner' AND (password_hash IS NULL OR password_hash = '' OR password_hash LIKE '$2a$10$A3OC%')`, string(hashOwner))
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET username = 'nando_noc', full_name = 'Nando Azkia Putra S.Kom (Kepala NOC & Core FO)' WHERE username = 'fadhil_noc'`)
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET full_name = 'Nando Azkia Putra S.Kom (Kepala NOC & Core FO)' WHERE username = 'nando_noc'`)
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET password_hash = $1 WHERE username = 'nando_noc' AND (password_hash IS NULL OR password_hash = '' OR password_hash LIKE '$2a$10$kElw%')`, string(hashNoc))
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET full_name = 'Nando Azkia Putra S.Kom (Koordinator Teknisi & Core FO)' WHERE username = 'nando'`)
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET password_hash = $1 WHERE username = 'nando' AND (password_hash IS NULL OR password_hash = '' OR password_hash LIKE '$2a$10$U3me%')`, string(hashNoc))
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET full_name = 'Fajar Malem Sitepu (Account Executive / Sales)' WHERE username = 'fajar'`)

	// Idempotently ensure finance staff user exists
	_, _ = s.db.ExecContext(ctx, `INSERT INTO staff_users (id, username, full_name, role, roles, is_superuser, contact_phone, status, password_hash, created_at, updated_at) 
		VALUES ($1, 'finance', 'Staf Keuangan & Billing (Finance)', 'FINANCE', '["FINANCE"]', 0, '081199887755', 'ACTIVE', $2, NOW(), NOW())
		ON CONFLICT (username) DO NOTHING`, uuid.NewString(), string(hashFinance))

	// Idempotently assign multi-roles for Single Sign-On across all subdomains
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET roles = '["SUPER_ADMIN","ADMIN_NOC","FINANCE","SALES","TECHNICIAN","JARTAPLOK"]', is_superuser = 1 WHERE username = 'owner'`)
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET roles = '["ADMIN_NOC","TECHNICIAN"]' WHERE username = 'nando_noc' OR username = 'nando'`)
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET roles = '["SALES"]' WHERE username = 'fajar'`)
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET roles = '["FINANCE"]' WHERE username = 'finance'`)
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET roles = '["TECHNICIAN"]' WHERE username IN ('ricci', 'zikka', 'egi')`)
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET roles = '["JARTAPLOK"]' WHERE username = 'gnet_biaro'`)
	_, _ = s.db.ExecContext(ctx, `UPDATE staff_users SET roles = '["BRANCH_MANAGER","ADMIN_NOC"]' WHERE username LIKE 'kacab_%'`)

	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff_users`).Scan(&count)
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO staff_users (id, username, password_hash, full_name, role, roles, is_superuser, contact_phone, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, '["SUPER_ADMIN","ADMIN_NOC","FINANCE","SALES","TECHNICIAN","JARTAPLOK"]', 1, $6, $7, NOW(), NOW())
			ON CONFLICT (username) DO NOTHING;
		`, "staff-admin-001", "admin", string(hash), "Administrator GoGiga", "SUPER_ADMIN", "081200000000", "ACTIVE")
	}

	// Link partners to appropriate branches
	_, _ = s.db.ExecContext(ctx, `
		UPDATE partners SET branch_code = 'PAPUA', branch_id = (SELECT id FROM branches WHERE code = 'PAPUA' LIMIT 1)
		WHERE code = 'SALES-PAPUA' AND (branch_code IS NULL OR branch_code = '' OR branch_code = 'ALL');

		UPDATE partners SET branch_code = 'PYK', branch_id = (SELECT id FROM branches WHERE code = 'PYK' LIMIT 1)
		WHERE (code IN ('FAJAR-PYK', 'EGI-TECH', 'NANDO-TECH', 'RICCI-TECH', 'ZIKKA-TECH', 'TEST-SU') OR code LIKE '%-PYK' OR code LIKE '%-TECH')
		  AND (branch_code IS NULL OR branch_code = '' OR branch_code = 'ALL');

		UPDATE partners SET branch_code = 'ALL', branch_id = NULL
		WHERE code = 'PARTNER-OFFICIAL';

		UPDATE partners SET branch_code = 'PYK', branch_id = (SELECT id FROM branches WHERE code = 'PYK' LIMIT 1)
		WHERE branch_code IS NULL OR branch_code = '';

		UPDATE jartaplok_partners SET branch_code = 'PYK', branch_id = (SELECT id FROM branches WHERE code = 'PYK' LIMIT 1)
		WHERE branch_code IS NULL OR branch_code = '';
	`)

	return nil
}

// ── ODP OPERATIONS ──────────────────────────────────────────────

func (s *PostgresStorage) ListODPs(ctx context.Context, branchCode string) ([]domain.ODPNode, error) {
	branchCode = strings.TrimSpace(branchCode)
	query := `
		SELECT o.id, o.code, o.name, o.latitude, o.longitude, o.total_ports, o.used_ports, o.status, o.cluster_area, 
		       COALESCE(o.provider_id, 'GNET-BIARO'), 
		       COALESCE(jp.name, o.provider_name, 'PT. GNET BIARO DATA'), 
		       o.created_at, o.updated_at,
		       o.branch_id::text,
		       COALESCE(b.code, 'PYK')
		FROM odp_nodes o 
		LEFT JOIN jartaplok_partners jp ON o.provider_id = jp.code 
		LEFT JOIN branches b ON o.branch_id = b.id
		WHERE o.status = 'AVAILABLE'
	`
	var args []interface{}
	if branchCode != "" && branchCode != "ALL" {
		query += ` AND (b.code = $1 OR o.branch_id::text = $1)`
		args = append(args, branchCode)
	}
	query += ` ORDER BY o.code ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	odps := make([]domain.ODPNode, 0)
	for rows.Next() {
		var o domain.ODPNode
		var branchID, branchCodeStr sql.NullString
		if err := rows.Scan(&o.ID, &o.Code, &o.Name, &o.Latitude, &o.Longitude, &o.TotalPorts, &o.UsedPorts, &o.Status, &o.ClusterArea, &o.ProviderID, &o.ProviderName, &o.CreatedAt, &o.UpdatedAt, &branchID, &branchCodeStr); err != nil {
			return nil, err
		}
		if branchID.Valid {
			o.BranchID = &branchID.String
		}
		if branchCodeStr.Valid {
			o.BranchCode = &branchCodeStr.String
		}
		odps = append(odps, o)
	}
	return odps, nil
}

func (s *PostgresStorage) GetODPByID(ctx context.Context, id string) (*domain.ODPNode, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT o.id, o.code, o.name, o.latitude, o.longitude, o.total_ports, o.used_ports, o.status, o.cluster_area, 
		       COALESCE(o.provider_id, 'GNET-BIARO'), 
		       COALESCE(jp.name, o.provider_name, 'PT. GNET BIARO DATA'), 
		       o.created_at, o.updated_at 
		FROM odp_nodes o 
		LEFT JOIN jartaplok_partners jp ON o.provider_id = jp.code 
		WHERE o.id = $1
	`, id)
	var o domain.ODPNode
	if err := row.Scan(&o.ID, &o.Code, &o.Name, &o.Latitude, &o.Longitude, &o.TotalPorts, &o.UsedPorts, &o.Status, &o.ClusterArea, &o.ProviderID, &o.ProviderName, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *PostgresStorage) GetODPByCode(ctx context.Context, code string) (*domain.ODPNode, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT o.id, o.code, o.name, o.latitude, o.longitude, o.total_ports, o.used_ports, o.status, o.cluster_area, 
		       COALESCE(o.provider_id, 'GNET-BIARO'), 
		       COALESCE(jp.name, o.provider_name, 'PT. GNET BIARO AKSES'), 
		       o.created_at, o.updated_at 
		FROM odp_nodes o 
		LEFT JOIN jartaplok_partners jp ON o.provider_id = jp.code 
		WHERE UPPER(o.code) = UPPER($1) LIMIT 1
	`, strings.TrimSpace(code))
	var o domain.ODPNode
	if err := row.Scan(&o.ID, &o.Code, &o.Name, &o.Latitude, &o.Longitude, &o.TotalPorts, &o.UsedPorts, &o.Status, &o.ClusterArea, &o.ProviderID, &o.ProviderName, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *PostgresStorage) CreateODP(ctx context.Context, odp *domain.ODPNode) error {
	if odp.ID == "" {
		odp.ID = uuid.NewString()
	}
	odp.CreatedAt = time.Now()
	odp.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO odp_nodes (
			id, code, name, latitude, longitude, total_ports, used_ports, status, cluster_area, provider_id, provider_name, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (code) DO UPDATE SET
			name = EXCLUDED.name,
			latitude = EXCLUDED.latitude,
			longitude = EXCLUDED.longitude,
			total_ports = EXCLUDED.total_ports,
			status = EXCLUDED.status,
			cluster_area = EXCLUDED.cluster_area,
			provider_id = EXCLUDED.provider_id,
			provider_name = EXCLUDED.provider_name,
			updated_at = EXCLUDED.updated_at
	`, odp.ID, odp.Code, odp.Name, odp.Latitude, odp.Longitude, odp.TotalPorts, odp.UsedPorts, odp.Status, odp.ClusterArea, odp.ProviderID, odp.ProviderName, odp.CreatedAt, odp.UpdatedAt)
	return err
}

func (s *PostgresStorage) DeleteODP(ctx context.Context, idOrCode string) error {
	var providerID, providerName string
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(provider_id, ''), COALESCE(provider_name, '') FROM odp_nodes WHERE id = $1 OR code = $2`, idOrCode, idOrCode).Scan(&providerID, &providerName)
	if err == nil && (providerID == "GNET2" || strings.Contains(providerName, "(2)")) {
		return fmt.Errorf("tiang ODP ini adalah aset mitra Jartaplok (%s) yang dikelola otomatis via API FTTX. Pengelolaan tiang hanya dapat dilakukan melalui FTTX Management System penyedia", providerName)
	}

	var activeCustCount int
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM registrations 
		WHERE (nearest_odp_id = $1 OR nearest_odp_code = $2) 
		  AND status NOT IN ('CANCELLED_NO_COVERAGE', 'REJECTED')
	`, idOrCode, idOrCode).Scan(&activeCustCount)
	if err != nil {
		return err
	}
	if activeCustCount > 0 {
		return fmt.Errorf("tiang ODP tidak dapat dihapus karena masih melayani %d pelanggan aktif/antrean. Silakan alihkan pelanggan ke tiang lain terlebih dahulu", activeCustCount)
	}

	res, err := s.db.ExecContext(ctx, `DELETE FROM odp_nodes WHERE id = $1 OR code = $2`, idOrCode, idOrCode)
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

// FindNearestAvailableODP uses PostGIS ST_DistanceSphere & ST_DWithin with spatial indexing
func (s *PostgresStorage) FindNearestAvailableODP(ctx context.Context, lat, lng float64, maxDistanceMeters float64) (*domain.ODPNode, float64, error) {
	query := `
		SELECT o.id, o.code, o.name, o.latitude, o.longitude, o.total_ports, o.used_ports, o.status, o.cluster_area, 
		       COALESCE(o.provider_id, 'GNET-BIARO'), 
		       COALESCE(jp.name, o.provider_name, 'PT. GNET BIARO DATA'), 
		       o.created_at, o.updated_at,
		       ST_DistanceSphere(o.geom, ST_SetSRID(ST_MakePoint($1, $2), 4326)) AS distance_meters
		FROM odp_nodes o 
		LEFT JOIN jartaplok_partners jp ON o.provider_id = jp.code 
		WHERE o.status = 'AVAILABLE' 
		  AND o.used_ports < o.total_ports
		  AND o.is_cluster_active = TRUE
		  AND ST_DWithin(o.geom::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3)
		ORDER BY distance_meters ASC
		LIMIT 1`

	var o domain.ODPNode
	var dist float64
	err := s.db.QueryRowContext(ctx, query, lng, lat, maxDistanceMeters).Scan(
		&o.ID, &o.Code, &o.Name, &o.Latitude, &o.Longitude, &o.TotalPorts, &o.UsedPorts, &o.Status, &o.ClusterArea,
		&o.ProviderID, &o.ProviderName, &o.CreatedAt, &o.UpdatedAt, &dist,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, 0, nil
		}
		// Fallback to in-memory Haversine if PostGIS geography functions are unavailable
		return s.findNearestFallback(ctx, lat, lng, maxDistanceMeters)
	}
	return &o, dist, nil
}

func (s *PostgresStorage) findNearestFallback(ctx context.Context, lat, lng float64, maxDistanceMeters float64) (*domain.ODPNode, float64, error) {
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

func (s *PostgresStorage) IncrementODPUsedPort(ctx context.Context, odpID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE odp_nodes 
		SET used_ports = used_ports + 1,
		    status = CASE WHEN used_ports + 1 >= total_ports THEN 'FULL' ELSE status END,
		    updated_at = NOW()
		WHERE id = $1
	`, odpID)
	return err
}

func (s *PostgresStorage) DecrementODPUsedPort(ctx context.Context, odpID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE odp_nodes 
		SET used_ports = GREATEST(0, used_ports - 1),
		    status = CASE WHEN (used_ports - 1) < total_ports AND status = 'FULL' THEN 'AVAILABLE' ELSE status END,
		    updated_at = NOW()
		WHERE id = $1
	`, odpID)
	return err
}

func (s *PostgresStorage) ListClusters(ctx context.Context, branchCode string) ([]domain.ClusterSummary, error) {
	branchCode = strings.TrimSpace(branchCode)
	var query string
	var args []interface{}
	if branchCode != "" && branchCode != "ALL" {
		query = `
			SELECT o.cluster_area, 
			       COUNT(*) as total, 
			       COALESCE(SUM(CASE WHEN o.status = 'AVAILABLE' AND o.is_cluster_active = true THEN 1 ELSE 0 END), 0) as active
			FROM odp_nodes o
			LEFT JOIN branches b ON o.branch_id = b.id
			WHERE b.code = $1 OR o.branch_id::text = $1
			GROUP BY o.cluster_area
			ORDER BY total DESC
		`
		args = append(args, branchCode)
	} else {
		query = `
			SELECT cluster_area, 
			       COUNT(*) as total, 
			       COALESCE(SUM(CASE WHEN status = 'AVAILABLE' AND is_cluster_active = true THEN 1 ELSE 0 END), 0) as active
			FROM odp_nodes
			GROUP BY cluster_area
			ORDER BY total DESC
		`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.ClusterSummary, 0)
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

func (s *PostgresStorage) SetClusterStatus(ctx context.Context, clusterArea string, active bool, branchCode string) error {
	branchCode = strings.TrimSpace(branchCode)
	if branchCode != "" && branchCode != "ALL" {
		var count int
		err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) 
			FROM odp_nodes o
			LEFT JOIN branches b ON o.branch_id = b.id
			WHERE o.cluster_area = $1 AND (b.code = $2 OR o.branch_id::text = $2)
		`, clusterArea, branchCode).Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("akses ditolak: cluster '%s' tidak berada di bawah wewenang cabang Anda", clusterArea)
		}

		_, err = s.db.ExecContext(ctx, `
			UPDATE odp_nodes SET is_cluster_active = $1, updated_at = NOW() 
			WHERE cluster_area = $2 AND (branch_id IN (SELECT id FROM branches WHERE code = $3) OR branch_id::text = $3)
		`, active, clusterArea, branchCode)
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE odp_nodes SET is_cluster_active = $1, updated_at = NOW() WHERE cluster_area = $2
	`, active, clusterArea)
	return err
}

func (s *PostgresStorage) ListODPsByProvider(ctx context.Context, providerID string) ([]domain.ODPNode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.id, o.code, o.name, o.latitude, o.longitude, o.total_ports, o.used_ports, o.status, o.cluster_area,
		       COALESCE(o.provider_id, 'GNET-BIARO'),
		       COALESCE(jp.name, o.provider_name, 'PT. GNET BIARO DATA'),
		       o.created_at, o.updated_at
		FROM odp_nodes o
		LEFT JOIN jartaplok_partners jp ON o.provider_id = jp.code
		WHERE o.provider_id = $1 OR jp.code = $1
		ORDER BY o.code ASC
	`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var odps []domain.ODPNode
	for rows.Next() {
		var o domain.ODPNode
		if err := rows.Scan(&o.ID, &o.Code, &o.Name, &o.Latitude, &o.Longitude, &o.TotalPorts, &o.UsedPorts, &o.Status, &o.ClusterArea, &o.ProviderID, &o.ProviderName, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		odps = append(odps, o)
	}
	return odps, nil
}

func (s *PostgresStorage) BatchUpsertODPs(ctx context.Context, providerID, providerName string, odps []domain.ODPNode) (*domain.KMLUploadResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result := &domain.KMLUploadResult{
		TotalPlacemarks: len(odps),
		ProviderID:      providerID,
		ProviderName:    providerName,
	}

	for _, odp := range odps {
		if odp.ID == "" {
			odp.ID = uuid.NewString()
		}
		var exists bool
		_ = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM odp_nodes WHERE code = $1)`, odp.Code).Scan(&exists)

		_, err := tx.ExecContext(ctx, `
			INSERT INTO odp_nodes (
				id, code, name, latitude, longitude, total_ports, used_ports, status, cluster_area, provider_id, provider_name, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
			ON CONFLICT (code) DO UPDATE SET
				name = EXCLUDED.name,
				latitude = EXCLUDED.latitude,
				longitude = EXCLUDED.longitude,
				total_ports = EXCLUDED.total_ports,
				cluster_area = EXCLUDED.cluster_area,
				provider_id = EXCLUDED.provider_id,
				provider_name = EXCLUDED.provider_name,
				updated_at = NOW()
		`, odp.ID, odp.Code, odp.Name, odp.Latitude, odp.Longitude, odp.TotalPorts, odp.UsedPorts, odp.Status, odp.ClusterArea, providerID, providerName)

		if err != nil {
			result.Skipped++
			result.Details = append(result.Details, fmt.Sprintf("%s: %v", odp.Code, err))
		} else {
			if exists {
				result.Updated++
			} else {
				result.Inserted++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// ── REGISTRATION OPERATIONS ─────────────────────────────────────

func (s *PostgresStorage) CreateRegistration(ctx context.Context, reg *domain.Registration) error {
	if reg.ID == "" {
		reg.ID = uuid.NewString()
	}
	reg.CreatedAt = time.Now()
	reg.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO registrations (
			id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, tax_id,
			address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code,
			distance_to_odp_meters, status, ktp_photo_url, house_photo_url, site_pic_name, site_pic_phone,
			dispatch_notes, custom_notes, otc_fee, monthly_price, otc_notes, pppoe_username, pppoe_password,
			upstream_pppoe_username, upstream_pppoe_password, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22,
			$23, $24, $25, $26, $27, $28, $29,
			$30, $31, $32, $33
		)
	`,
		reg.ID, reg.RegistrationNo, reg.PartnerID, reg.PartnerCode, reg.FullName, reg.Email, reg.Phone, reg.IDCardNumber, reg.TaxID,
		reg.Address, reg.Latitude, reg.Longitude, reg.SelectedPlanID, reg.SelectedPlanName, reg.NearestODPID, reg.NearestODPCode,
		reg.DistanceToODPMeters, reg.Status, reg.KTPPhotoURL, reg.HousePhotoURL, reg.SitePICName, reg.SitePICPhone,
		reg.DispatchNotes, reg.CustomNotes, reg.OTCFee, reg.MonthlyPrice, reg.OTCNotes, reg.PPPoEUsername, reg.PPPoEPassword,
		reg.UpstreamPPPoEUsername, reg.UpstreamPPPoEPassword, reg.CreatedAt, reg.UpdatedAt,
	)
	return err
}

func (s *PostgresStorage) GetRegistrationByID(ctx context.Context, id string) (*domain.Registration, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, COALESCE(tax_id, ''), address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code, distance_to_odp_meters, status, COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''), COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''), COALESCE(contract_signature_url, ''), contract_signed_at, gigabill_customer_id, gigabill_subscription_id, activated_at, suspended_at, COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''), COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''), COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''), COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''),
		       COALESCE((SELECT jp.suspension_policy FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'ALLOWED_WITH_WAIVER'),
		       COALESCE((SELECT jp.name FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'PT. GNET BIARO AKSES'),
		       created_at, updated_at 
		FROM registrations WHERE id = $1
	`, id)
	return scanRegistration(row)
}

func (s *PostgresStorage) GetRegistrationByNo(ctx context.Context, identifier string) (*domain.Registration, error) {
	clean := strings.TrimSpace(identifier)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, COALESCE(tax_id, ''), address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code, distance_to_odp_meters, status, COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''), COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''), COALESCE(contract_signature_url, ''), contract_signed_at, gigabill_customer_id, gigabill_subscription_id, activated_at, suspended_at, COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''), COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''), COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''), COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''),
		       COALESCE((SELECT jp.suspension_policy FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'ALLOWED_WITH_WAIVER'),
		       COALESCE((SELECT jp.name FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'PT. GNET BIARO AKSES'),
		       created_at, updated_at 
		FROM registrations 
		WHERE registration_no = $1 
		   OR gigabill_customer_id = $2
		   OR (email != '' AND LOWER(email) = LOWER($3)) 
		   OR (phone != '' AND phone = $4)
		ORDER BY created_at DESC LIMIT 1
	`, clean, clean, clean, clean)
	return scanRegistration(row)
}

func (s *PostgresStorage) GetRegistrationByGigabillCustomerID(ctx context.Context, customerID string) (*domain.Registration, error) {
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
		WHERE gigabill_customer_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, clean)
	return scanRegistration(row)
}

func (s *PostgresStorage) UpdateRegistrationContact(ctx context.Context, id, email, phone string) error {
	cleanID := strings.TrimSpace(id)
	cleanEmail := strings.TrimSpace(email)
	cleanPhone := strings.TrimSpace(phone)
	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET email = CASE WHEN $1 != '' THEN $1 ELSE email END,
		    phone = CASE WHEN $2 != '' THEN $2 ELSE phone END,
		    updated_at = NOW()
		WHERE id = $3
	`, cleanEmail, cleanPhone, cleanID)
	return err
}

func (s *PostgresStorage) ListRegistrationsByCustomer(ctx context.Context, identifier string) ([]domain.Registration, error) {
	clean := strings.TrimSpace(identifier)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, COALESCE(tax_id, ''), address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code, distance_to_odp_meters, status, COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''), COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''), COALESCE(contract_signature_url, ''), contract_signed_at, gigabill_customer_id, gigabill_subscription_id, activated_at, suspended_at, COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''), COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''), COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''), COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''),
		       COALESCE((SELECT jp.suspension_policy FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'ALLOWED_WITH_WAIVER'),
		       COALESCE((SELECT jp.name FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'PT. GNET BIARO AKSES'),
		       created_at, updated_at 
		FROM registrations 
		WHERE registration_no = $1 
		   OR gigabill_customer_id = $2
		   OR (id_card_number != '' AND id_card_number = $3)
		   OR (email != '' AND LOWER(email) = LOWER($4)) 
		   OR (phone != '' AND phone = $5)
		ORDER BY created_at DESC
	`, clean, clean, clean, clean, clean)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanRegistrationsList(rows)
}

func (s *PostgresStorage) ListRegistrations(ctx context.Context, partnerID *string, status *string, branchCode *string) ([]domain.Registration, error) {
	query := `SELECT id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, COALESCE(tax_id, ''), address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code, distance_to_odp_meters, status, COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''), COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''), COALESCE(contract_signature_url, ''), contract_signed_at, gigabill_customer_id, gigabill_subscription_id, activated_at, suspended_at, COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''), COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''), COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''), COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''),
	       COALESCE((SELECT jp.suspension_policy FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'ALLOWED_WITH_WAIVER'),
	       COALESCE((SELECT jp.name FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'PT. GNET BIARO AKSES'),
	       created_at, updated_at FROM registrations WHERE 1=1`
	var args []interface{}
	idx := 1

	if partnerID != nil && *partnerID != "" {
		query += fmt.Sprintf(` AND (partner_id = $%d OR partner_code = (SELECT code FROM partners WHERE id = $%d))`, idx, idx+1)
		args = append(args, *partnerID, *partnerID)
		idx += 2
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(` AND status = $%d`, idx)
		args = append(args, *status)
		idx++
	}
	if branchCode != nil && *branchCode != "" && *branchCode != "ALL" {
		query += fmt.Sprintf(` AND (registrations.branch_id IN (SELECT id FROM branches WHERE code = $%d) OR registrations.branch_id::text = $%d)`, idx, idx+1)
		args = append(args, *branchCode, *branchCode)
		idx += 2
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanRegistrationsList(rows)
}

func (s *PostgresStorage) ListRegistrationsForPartner(ctx context.Context, partnerID string) ([]domain.Registration, error) {
	query := `SELECT id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, COALESCE(tax_id, ''), address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code, distance_to_odp_meters, status, COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''), COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''), COALESCE(contract_signature_url, ''), contract_signed_at, gigabill_customer_id, gigabill_subscription_id, activated_at, suspended_at, COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''), COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''), COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''), COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''),
	       COALESCE((SELECT jp.suspension_policy FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'ALLOWED_WITH_WAIVER'),
	       COALESCE((SELECT jp.name FROM odp_nodes o JOIN jartaplok_partners jp ON o.provider_id = jp.code WHERE o.code = registrations.nearest_odp_code LIMIT 1), 'PT. GNET BIARO AKSES'),
	       created_at, updated_at 
	FROM registrations 
	WHERE (partner_id = $1 
	       OR partner_code = (SELECT code FROM partners WHERE id = $2)
	       OR partner_code ILIKE (SELECT split_part(code, '-', 1) FROM partners WHERE id = $2)
	       OR partner_code ILIKE (SELECT split_part(code, '-', 1) || '-%' FROM partners WHERE id = $2)
	      )
	   OR (
	       (partner_id IS NULL OR partner_id = '' OR partner_code = 'DIRECT_SALES')
	       AND status NOT IN ('ACTIVE', 'SUSPENDED', 'TERMINATED', 'CANCELLED_NO_COVERAGE')
	       AND (
	           selected_plan_name ILIKE '%corporate%' 
	           OR selected_plan_name ILIKE '%dedicated%' 
	           OR selected_plan_name ILIKE '%bisnis%' 
	           OR selected_plan_name ILIKE '%soho%'
	           OR monthly_price >= 500000
	       )
	   )
	ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, partnerID, partnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanRegistrationsList(rows)
}

func (s *PostgresStorage) scanRegistrationsList(rows *sql.Rows) ([]domain.Registration, error) {
	list := make([]domain.Registration, 0)
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

func (s *PostgresStorage) ClaimRegistration(ctx context.Context, regNo string, partnerID string) (*domain.Registration, error) {
	var partnerCode string
	err := s.db.QueryRowContext(ctx, `SELECT code FROM partners WHERE id = $1`, partnerID).Scan(&partnerCode)
	if err != nil {
		return nil, fmt.Errorf("mitra tidak valid: %w", err)
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE registrations
		SET partner_id = $1, partner_code = $2, updated_at = NOW()
		WHERE registration_no = $3 
		  AND (partner_id IS NULL OR partner_id = '' OR partner_code = 'DIRECT_SALES')
	`, partnerID, partnerCode, regNo)
	if err != nil {
		return nil, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, fmt.Errorf("registrasi tidak dapat diklaim (mungkin sudah menjadi referral sales/mitra lain)")
	}
	return s.GetRegistrationByNo(ctx, regNo)
}

func (s *PostgresStorage) UpdateRegistrationStatus(ctx context.Context, id string, status string, gigabillCustID, gigabillSubID *string) error {
	query := `UPDATE registrations SET status = $1, updated_at = NOW()`
	args := []interface{}{status}
	idx := 2

	if gigabillCustID != nil {
		query += fmt.Sprintf(`, gigabill_customer_id = $%d`, idx)
		args = append(args, *gigabillCustID)
		idx++
	}
	if gigabillSubID != nil {
		query += fmt.Sprintf(`, gigabill_subscription_id = $%d`, idx)
		args = append(args, *gigabillSubID)
		idx++
	}
	if status == "ACTIVE" {
		query += `, activated_at = NOW()`
	}

	query += fmt.Sprintf(` WHERE id = $%d`, idx)
	args = append(args, id)

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func (s *PostgresStorage) UpdateRegistrationKTP(ctx context.Context, regNo string, ktpURL string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE registrations SET ktp_photo_url = $1, updated_at = NOW() WHERE registration_no = $2`, ktpURL, regNo)
	return err
}

func (s *PostgresStorage) UpdateRegistrationContract(ctx context.Context, regNo string, signatureURL string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE registrations SET contract_signature_url = $1, contract_signed_at = NOW(), updated_at = NOW() WHERE registration_no = $2`, signatureURL, regNo)
	return err
}

func (s *PostgresStorage) UpdateRegistrationHousePhoto(ctx context.Context, regNo string, housePhotoURL string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE registrations SET house_photo_url = $1, updated_at = NOW() WHERE registration_no = $2`, housePhotoURL, regNo)
	return err
}

func (s *PostgresStorage) UpdateRegistrationSitePIC(ctx context.Context, regNo string, picName string, picPhone string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE registrations SET site_pic_name = $1, site_pic_phone = $2, updated_at = NOW() WHERE registration_no = $3`, picName, picPhone, regNo)
	return err
}

func (s *PostgresStorage) SuspendRegistration(ctx context.Context, idOrNo string, reason string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET status = 'SUSPENDED', suspended_at = NOW(), suspension_reason = $1, updated_at = NOW()
		WHERE id = $2 OR registration_no = $3
	`, reason, idOrNo, idOrNo)
	return err
}

func (s *PostgresStorage) ResumeRegistration(ctx context.Context, idOrNo string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET status = 'ACTIVE', suspended_at = NULL, suspension_reason = '', updated_at = NOW()
		WHERE id = $1 OR registration_no = $2
	`, idOrNo, idOrNo)
	return err
}

func (s *PostgresStorage) AttachRegistrationToPartnerODP(ctx context.Context, idOrNo string, req domain.AttachPartnerODPRequest) (*domain.Registration, error) {
	cleanCode := strings.TrimSpace(req.ODPCode)
	odp, err := s.GetODPByCode(ctx, cleanCode)
	if err != nil {
		return nil, fmt.Errorf("tiang ODP dengan kode '%s' tidak ditemukan: %w", cleanCode, err)
	}

	dist := req.DistanceMeters
	if dist <= 0 {
		dist = 50.0
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE registrations
		SET nearest_odp_id = $1,
		    nearest_odp_code = $2,
		    distance_to_odp_meters = $3,
		    status = CASE WHEN status IN ('UNCOVERED_LEAD', 'PENDING_SURVEY_OVERDISTANCE', 'SUBMITTED', 'UNCOVERED_WISHLIST', 'SURVEY_SCHEDULED') THEN 'INSTALLATION_SCHEDULED' ELSE status END,
		    dispatch_notes = $4,
		    upstream_pppoe_username = CASE WHEN $5 != '' THEN $5 ELSE upstream_pppoe_username END,
		    upstream_pppoe_password = CASE WHEN $6 != '' THEN $6 ELSE upstream_pppoe_password END,
		    updated_at = NOW()
		WHERE id = $7 OR registration_no = $8
	`, odp.ID, odp.Code, dist, req.Notes, strings.TrimSpace(req.UpstreamPPPoEUsername), strings.TrimSpace(req.UpstreamPPPoEPassword), idOrNo, idOrNo)
	if err != nil {
		return nil, err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("registrasi tidak ditemukan")
	}

	_ = s.IncrementODPUsedPort(ctx, odp.ID)
	return s.GetRegistrationByNo(ctx, idOrNo)
}

func (s *PostgresStorage) MarkRegistrationUncovered(ctx context.Context, idOrNo string, action string, reason string) (*domain.Registration, error) {
	status := "UNCOVERED_LEAD"
	if strings.ToUpper(action) == "CANCEL" {
		status = "CANCELLED_NO_COVERAGE"
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations
		SET status = $1,
		    custom_notes = $2,
		    updated_at = NOW()
		WHERE id = $3 OR registration_no = $4
	`, status, reason, idOrNo, idOrNo)
	if err != nil {
		return nil, err
	}
	return s.GetRegistrationByNo(ctx, idOrNo)
}

func (s *PostgresStorage) UpdateRegistrationPricing(ctx context.Context, idOrNo string, req domain.UpdateRegistrationPricingRequest) (*domain.Registration, error) {
	newStatusExpr := "status"
	if req.RequestNOCApproval {
		newStatusExpr = "'WAITING_APPROVAL_NOC'"
	} else if req.PromoteToInstall {
		newStatusExpr = "'INSTALLATION_SCHEDULED'"
	}

	query := fmt.Sprintf(`
		UPDATE registrations
		SET otc_fee = $1,
		    monthly_price = $2,
		    otc_notes = $3,
		    tax_id = CASE WHEN $4 != '' THEN $4 ELSE tax_id END,
		    selected_plan_id = CASE WHEN $5 != '' THEN $5 ELSE selected_plan_id END,
		    selected_plan_name = CASE WHEN $6 != '' THEN $6 ELSE selected_plan_name END,
		    nearest_odp_code = CASE WHEN $7 != '' THEN $7 ELSE nearest_odp_code END,
		    status = %s,
		    updated_at = NOW()
		WHERE id = $8 OR registration_no = $9
	`, newStatusExpr)

	_, err := s.db.ExecContext(ctx, query,
		req.OTCFee, req.MonthlyPrice, strings.TrimSpace(req.OTCNotes),
		strings.TrimSpace(req.TaxID), strings.TrimSpace(req.SelectedPlanID), strings.TrimSpace(req.SelectedPlanName),
		strings.TrimSpace(req.ODPCode), idOrNo, idOrNo)
	if err != nil {
		return nil, err
	}

	if req.PromoteToInstall && !req.RequestNOCApproval {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE work_orders
			SET type = 'INSTALLATION',
			    status = 'ASSIGNED',
			    notes = notes || ' | Siap Pasang setelah survei disetujui (Kabel Tambahan Ditanggung Pelanggan)',
			    updated_at = NOW()
			WHERE registration_id = (SELECT id FROM registrations WHERE id = $1 OR registration_no = $2 LIMIT 1)
		`, idOrNo, idOrNo)
	}

	return s.GetRegistrationByNo(ctx, idOrNo)
}

func (s *PostgresStorage) NOCApprovalRegistration(ctx context.Context, idOrNo string, action string, approver string, notes string) (*domain.Registration, error) {
	isApprove := strings.ToUpper(strings.TrimSpace(action)) == "APPROVE"
	targetStatus := "SURVEY_REVISED"
	if isApprove {
		targetStatus = "INSTALLATION_SCHEDULED"
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations
		SET status = $1,
		    dispatch_notes = CASE WHEN $2 != '' THEN dispatch_notes || ' | NOC Approval: ' || $2 ELSE dispatch_notes END,
		    updated_at = NOW()
		WHERE id = $3 OR registration_no = $4
	`, targetStatus, notes, idOrNo, idOrNo)
	if err != nil {
		return nil, err
	}

	if isApprove {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE work_orders
			SET type = 'INSTALLATION',
			    status = 'ASSIGNED',
			    notes = notes || ' | Disetujui NOC (' || $1 || '): Siap Pasang Lapangan',
			    updated_at = NOW()
			WHERE registration_id = (SELECT id FROM registrations WHERE id = $2 OR registration_no = $3 LIMIT 1)
		`, approver, idOrNo, idOrNo)
	} else {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE work_orders
			SET status = 'FAILED',
			    notes = notes || ' | Ditolak NOC (' || $1 || '): ' || $2,
			    updated_at = NOW()
			WHERE registration_id = (SELECT id FROM registrations WHERE id = $3 OR registration_no = $4 LIMIT 1)
		`, approver, notes, idOrNo, idOrNo)
	}

	return s.GetRegistrationByNo(ctx, idOrNo)
}

func (s *PostgresStorage) UpgradeBandwidth(ctx context.Context, idOrNo string, req domain.UpgradeBandwidthRequest) (*domain.Registration, error) {
	current, err := s.GetRegistrationByNo(ctx, idOrNo)
	if err != nil || current == nil {
		current, err = s.GetRegistrationByID(ctx, idOrNo)
		if err != nil || current == nil {
			return nil, fmt.Errorf("registrasi %s tidak ditemukan", idOrNo)
		}
	}

	effDate := strings.TrimSpace(req.EffectiveDate)
	if effDate == "" {
		effDate = "SEGERA"
	}

	logNote := fmt.Sprintf("\n[Upgrade Paket %s]: %s (Rp %d) -> %s (Rp %d) [Efektif: %s]",
		time.Now().Format("2006-01-02 15:04"),
		current.SelectedPlanName, current.MonthlyPrice,
		req.NewPlanName, req.NewMonthlyPrice,
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

	_, err = s.db.ExecContext(ctx, `
		UPDATE registrations
		SET selected_plan_id = CASE WHEN $1 != '' THEN $1 ELSE selected_plan_id END,
		    selected_plan_name = $2,
		    monthly_price = $3,
		    dispatch_notes = $4,
		    updated_at = NOW()
		WHERE id = $5 OR registration_no = $6
	`, req.NewPlanID, req.NewPlanName, req.NewMonthlyPrice, updatedDispatch, current.ID, current.RegistrationNo)
	if err != nil {
		return nil, err
	}
	return s.GetRegistrationByNo(ctx, current.RegistrationNo)
}

func (s *PostgresStorage) UpdateRegistrationODP(ctx context.Context, regID string, odpID string, odpCode string, distanceMeters float64, notes string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations
		SET nearest_odp_id = $1,
		    nearest_odp_code = $2,
		    distance_to_odp_meters = $3,
		    dispatch_notes = $4,
		    updated_at = NOW()
		WHERE id = $5 OR registration_no = $6
	`, odpID, odpCode, distanceMeters, notes, regID, regID)
	return err
}

func (s *PostgresStorage) UpdateRegistrationPPPoE(ctx context.Context, regNo string, pppoeUser string, pppoePass string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET pppoe_username = $1, pppoe_password = $2, updated_at = NOW()
		WHERE registration_no = $3 OR id = $4
	`, pppoeUser, pppoePass, regNo, regNo)
	return err
}

func (s *PostgresStorage) UpdateRegistrationUpstreamPPPoE(ctx context.Context, regNo string, upstreamUser string, upstreamPass string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET upstream_pppoe_username = $1, upstream_pppoe_password = $2, updated_at = NOW()
		WHERE registration_no = $3 OR id = $4
	`, upstreamUser, upstreamPass, regNo, regNo)
	return err
}

func (s *PostgresStorage) GetNextPPPoESequence(ctx context.Context, clusterCode string) (int, error) {
	var maxSeq sql.NullInt64
	query := `SELECT MAX(CAST(SUBSTRING(pppoe_username FROM 6 FOR 5) AS INTEGER)) 
	          FROM registrations 
	          WHERE pppoe_username LIKE '%' || $1 || '%@gogiga.net.id'`
	err := s.db.QueryRowContext(ctx, query, clusterCode).Scan(&maxSeq)
	if err != nil && err != sql.ErrNoRows {
		return 1, err
	}

	var maxAccSeq sql.NullInt64
	accQuery := `SELECT MAX(CAST(SUBSTRING(identity FROM 6 FOR 5) AS INTEGER))
	             FROM access_accounts
	             WHERE identity LIKE '%' || $1 || '%@gogiga.net.id'`
	_ = s.db.QueryRowContext(ctx, accQuery, clusterCode).Scan(&maxAccSeq)

	currMax := int64(0)
	if maxSeq.Valid && maxSeq.Int64 > currMax {
		currMax = maxSeq.Int64
	}
	if maxAccSeq.Valid && maxAccSeq.Int64 > currMax {
		currMax = maxAccSeq.Int64
	}
	return int(currMax) + 1, nil
}

func (s *PostgresStorage) DeleteRegistration(ctx context.Context, idOrRegNo string) error {
	var regID, nearestODPID, nearestODPCode string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(nearest_odp_id, ''), COALESCE(nearest_odp_code, '') 
		FROM registrations 
		WHERE id = $1 OR registration_no = $2
	`, idOrRegNo, idOrRegNo).Scan(&regID, &nearestODPID, &nearestODPCode)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("permohonan registrasi tidak ditemukan")
		}
		return err
	}

	_, _ = s.db.ExecContext(ctx, `DELETE FROM bast_reports WHERE work_order_id IN (SELECT id FROM work_orders WHERE registration_id = $1)`, regID)
	_, _ = s.db.ExecContext(ctx, `DELETE FROM work_orders WHERE registration_id = $1`, regID)

	res, err := s.db.ExecContext(ctx, `DELETE FROM registrations WHERE id = $1`, regID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("permohonan registrasi tidak ditemukan")
	}

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
			updated_at = NOW()
			WHERE id = $1 OR code = $2
		`, nearestODPID, nearestODPCode)
	}

	return nil
}

// ── WORK ORDER & BAST ───────────────────────────────────────────

func (s *PostgresStorage) CreateWorkOrder(ctx context.Context, wo *domain.WorkOrder) error {
	if wo.ID == "" {
		wo.ID = uuid.NewString()
	}
	wo.CreatedAt = time.Now()
	wo.UpdatedAt = time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO work_orders (id, order_no, registration_id, type, technician_name, scheduled_at, status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, wo.ID, wo.OrderNo, wo.RegistrationID, wo.Type, wo.TechnicianName, wo.ScheduledAt, wo.Status, wo.Notes, wo.CreatedAt, wo.UpdatedAt)
	return err
}

func (s *PostgresStorage) GetWorkOrderByID(ctx context.Context, id string) (*domain.WorkOrder, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, order_no, registration_id, type, technician_name, scheduled_at, status, COALESCE(notes, ''), created_at, updated_at
		FROM work_orders WHERE id = $1
	`, id)
	var wo domain.WorkOrder
	if err := row.Scan(&wo.ID, &wo.OrderNo, &wo.RegistrationID, &wo.Type, &wo.TechnicianName, &wo.ScheduledAt, &wo.Status, &wo.Notes, &wo.CreatedAt, &wo.UpdatedAt); err != nil {
		return nil, err
	}

	bastRow := s.db.QueryRowContext(ctx, `
		SELECT id, work_order_id, optical_power_dbm, ont_serial_number, ont_mac_address, dropcore_length_meters, customer_signature_url, proof_photo_url, house_photo_url, speedtest_down_mbps, speedtest_up_mbps, notes, COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''), created_at
		FROM bast_reports WHERE work_order_id = $1
	`, wo.ID)
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

func (s *PostgresStorage) GetWorkOrderByRegistrationID(ctx context.Context, regID string) (*domain.WorkOrder, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, order_no, registration_id, type, technician_name, scheduled_at, status, COALESCE(notes, ''), created_at, updated_at
		FROM work_orders WHERE registration_id = $1 ORDER BY created_at DESC LIMIT 1
	`, regID)
	var wo domain.WorkOrder
	if err := row.Scan(&wo.ID, &wo.OrderNo, &wo.RegistrationID, &wo.Type, &wo.TechnicianName, &wo.ScheduledAt, &wo.Status, &wo.Notes, &wo.CreatedAt, &wo.UpdatedAt); err != nil {
		return nil, err
	}

	bastRow := s.db.QueryRowContext(ctx, `
		SELECT id, work_order_id, optical_power_dbm, ont_serial_number, ont_mac_address, dropcore_length_meters, customer_signature_url, proof_photo_url, house_photo_url, speedtest_down_mbps, speedtest_up_mbps, notes, COALESCE(upstream_pppoe_username, ''), COALESCE(upstream_pppoe_password, ''), created_at
		FROM bast_reports WHERE work_order_id = $1
	`, wo.ID)
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

func (s *PostgresStorage) ListWorkOrders(ctx context.Context, status *string) ([]domain.WorkOrder, error) {
	query := `SELECT id, order_no, registration_id, type, technician_name, scheduled_at, status, COALESCE(notes, ''), created_at, updated_at FROM work_orders`
	var args []interface{}
	if status != nil && *status != "" {
		query += ` WHERE status = $1`
		args = append(args, *status)
	}
	query += ` ORDER BY scheduled_at ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.WorkOrder
	for rows.Next() {
		var w domain.WorkOrder
		var notes string
		if err := rows.Scan(&w.ID, &w.OrderNo, &w.RegistrationID, &w.Type, &w.TechnicianName, &w.ScheduledAt, &w.Status, &notes, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		w.Notes = notes
		list = append(list, w)
	}
	return list, nil
}

func (s *PostgresStorage) SaveBAST(ctx context.Context, bast *domain.BASTReport) error {
	if bast.ID == "" {
		bast.ID = uuid.NewString()
	}
	bast.CreatedAt = time.Now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, _ = tx.ExecContext(ctx, `DELETE FROM bast_reports WHERE work_order_id = $1`, bast.WorkOrderID)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO bast_reports (
			id, work_order_id, optical_power_dbm, ont_serial_number, ont_mac_address,
			dropcore_length_meters, customer_signature_url, proof_photo_url, house_photo_url, speedtest_down_mbps, speedtest_up_mbps, notes,
			upstream_pppoe_username, upstream_pppoe_password, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`,
		bast.ID, bast.WorkOrderID, bast.OpticalPowerDBM, bast.ONTSerialNumber, bast.ONTMACAddress,
		bast.DropcoreLengthMeters, bast.CustomerSignatureURL, bast.ProofPhotoURL, bast.HousePhotoURL, bast.SpeedtestDownMbps, bast.SpeedtestUpMbps, bast.Notes,
		bast.UpstreamPPPoEUsername, bast.UpstreamPPPoEPassword, bast.CreatedAt,
	)
	if err != nil {
		return err
	}

	if bast.UpstreamPPPoEUsername != "" {
		_, _ = tx.ExecContext(ctx, `
			UPDATE registrations
			SET upstream_pppoe_username = $1,
			    upstream_pppoe_password = CASE WHEN $2 != '' THEN $2 ELSE upstream_pppoe_password END,
			    updated_at = NOW()
			WHERE id = (SELECT registration_id FROM work_orders WHERE id = $3)
		`, bast.UpstreamPPPoEUsername, bast.UpstreamPPPoEPassword, bast.WorkOrderID)
	}

	if bast.TechnicianName != "" {
		_, err = tx.ExecContext(ctx, `UPDATE work_orders SET status = 'COMPLETED', technician_name = $1, updated_at = NOW() WHERE id = $2`, bast.TechnicianName, bast.WorkOrderID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE work_orders SET status = 'COMPLETED', updated_at = NOW() WHERE id = $1`, bast.WorkOrderID)
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *PostgresStorage) AttachSmartOLTDevice(ctx context.Context, idOrRegNo string, sn string, mac string, opticalPower float64, status string, pppoeUser string) error {
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
		SET status = $1, 
		    upstream_pppoe_username = $2,
		    activated_at = CASE WHEN activated_at IS NULL THEN CURRENT_TIMESTAMP ELSE activated_at END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
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

// ── PARTNER OPERATIONS ──────────────────────────────────────────

func (s *PostgresStorage) GetPartnerByAPIKey(ctx context.Context, apiKey string) (*domain.Partner, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, api_key, commission_rate, contact_phone, is_active, created_at
		FROM partners WHERE api_key = $1 AND is_active = TRUE
	`, apiKey)
	var p domain.Partner
	var isActive bool
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.APIKey, &p.CommissionRate, &p.ContactPhone, &isActive, &p.CreatedAt); err != nil {
		return nil, err
	}
	p.IsActive = isActive
	return &p, nil
}

func (s *PostgresStorage) GetPartnerByID(ctx context.Context, id string) (*domain.Partner, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, api_key, commission_rate, contact_phone, is_active, created_at
		FROM partners WHERE id = $1
	`, id)
	var p domain.Partner
	var isActive bool
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.APIKey, &p.CommissionRate, &p.ContactPhone, &isActive, &p.CreatedAt); err != nil {
		return nil, err
	}
	p.IsActive = isActive
	return &p, nil
}

func (s *PostgresStorage) GetPartnerByCode(ctx context.Context, code string) (*domain.Partner, error) {
	cleanCode := strings.ToUpper(strings.TrimSpace(code))
	row := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, api_key, commission_rate, contact_phone, is_active, created_at
		FROM partners 
		WHERE UPPER(code) = $1
		   OR UPPER(code) = $1 || '-PYK'
		   OR UPPER(code) = $1 || '-TECH'
		ORDER BY CASE 
			WHEN UPPER(code) = $1 THEN 1 
			WHEN UPPER(code) = $1 || '-PYK' THEN 2 
			ELSE 3 
		END
		LIMIT 1
	`, cleanCode)
	var p domain.Partner
	var isActive bool
	if err := row.Scan(&p.ID, &p.Code, &p.Name, &p.APIKey, &p.CommissionRate, &p.ContactPhone, &isActive, &p.CreatedAt); err != nil {
		return nil, err
	}
	p.IsActive = isActive
	return &p, nil
}

func (s *PostgresStorage) GetActiveCustomerByReferralCode(ctx context.Context, code string) (*domain.Registration, error) {
	cleanCode := strings.ToUpper(strings.TrimSpace(code))
	row := s.db.QueryRowContext(ctx, `
		SELECT id, registration_no, full_name, phone, status
		FROM registrations
		WHERE (UPPER(registration_no) = $1 OR phone = $2)
		  AND status = 'ACTIVE'
		LIMIT 1
	`, cleanCode, strings.TrimSpace(code))
	var r domain.Registration
	if err := row.Scan(&r.ID, &r.RegistrationNo, &r.FullName, &r.Phone, &r.Status); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *PostgresStorage) CreatePartner(ctx context.Context, p *domain.Partner) error {
	if _, err := uuid.Parse(p.ID); err != nil {
		p.ID = uuid.NewString()
	}
	p.CreatedAt = time.Now()
	if p.BranchCode != nil && *p.BranchCode != "" && *p.BranchCode != "ALL" && p.BranchID == nil {
		var bID string
		if err := s.db.QueryRowContext(ctx, `SELECT id FROM branches WHERE code = $1 LIMIT 1`, *p.BranchCode).Scan(&bID); err == nil {
			p.BranchID = &bID
		}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO partners (id, code, name, api_key, commission_rate, contact_phone, is_active, status, branch_id, branch_code, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'ACTIVE', $8, $9, $10, $10)
	`, p.ID, p.Code, p.Name, p.APIKey, p.CommissionRate, p.ContactPhone, p.IsActive, p.BranchID, p.BranchCode, p.CreatedAt)
	return err
}

func (s *PostgresStorage) UpdatePartner(ctx context.Context, p *domain.Partner) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE partners SET name = $1, commission_rate = $2, contact_phone = $3, is_active = $4 WHERE id = $5
	`, p.Name, p.CommissionRate, p.ContactPhone, p.IsActive, p.ID)
	return err
}

func (s *PostgresStorage) ListPartners(ctx context.Context, branchCode string) ([]domain.Partner, error) {
	branchCode = strings.TrimSpace(branchCode)
	query := `
		SELECT p.id, p.code, p.name, p.api_key, p.commission_rate, p.contact_phone, p.is_active, p.created_at,
		       p.branch_id::text, COALESCE(b.code, p.branch_code, 'ALL') AS branch_code
		FROM partners p
		LEFT JOIN branches b ON p.branch_id = b.id
	`
	var args []interface{}
	if branchCode != "" && branchCode != "ALL" {
		query += ` WHERE (b.code = $1 OR p.branch_code = $1 OR p.branch_id::text = $1)`
		args = append(args, branchCode)
	}
	query += ` ORDER BY p.name ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]domain.Partner, 0)
	for rows.Next() {
		var p domain.Partner
		var isActive bool
		var bID, bCode *string
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.APIKey, &p.CommissionRate, &p.ContactPhone, &isActive, &p.CreatedAt, &bID, &bCode); err != nil {
			return nil, err
		}
		p.IsActive = isActive
		p.BranchID = bID
		p.BranchCode = bCode
		list = append(list, p)
	}
	return list, nil
}

// ── STAFF USERS & SUPERUSER ─────────────────────────────────────

func (s *PostgresStorage) ListStaffUsers(ctx context.Context) ([]domain.StaffUser, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.username, s.full_name, s.role, COALESCE(s.roles, '[]'), COALESCE(s.is_superuser, 0), s.contact_phone, s.status, s.created_at, s.updated_at,
		       s.branch_id, b.code
		FROM staff_users s
		LEFT JOIN branches b ON s.branch_id = b.id
		ORDER BY s.full_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.StaffUser
	for rows.Next() {
		var u domain.StaffUser
		var branchID, branchCode sql.NullString
		var rolesJSON string
		var isSuper int
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &rolesJSON, &isSuper, &u.ContactPhone, &u.Status, &u.CreatedAt, &u.UpdatedAt, &branchID, &branchCode); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(rolesJSON), &u.Roles)
		if len(u.Roles) == 0 && u.Role != "" {
			u.Roles = []string{u.Role}
		}
		u.IsSuperuser = (isSuper == 1) || u.Role == "SUPER_ADMIN"
		if branchID.Valid {
			u.BranchID = &branchID.String
		}
		if branchCode.Valid {
			u.BranchCode = &branchCode.String
		}
		list = append(list, u)
	}
	return list, nil
}

func (s *PostgresStorage) CreateStaffUser(ctx context.Context, u *domain.StaffUser) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	rolesBytes, _ := json.Marshal(u.Roles)
	isSuper := 0
	if u.IsSuperuser || u.Role == "SUPER_ADMIN" {
		isSuper = 1
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO staff_users (id, username, password_hash, full_name, role, roles, is_superuser, contact_phone, status, created_at, updated_at, branch_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, u.ID, u.Username, u.PasswordHash, u.FullName, u.Role, string(rolesBytes), isSuper, u.ContactPhone, u.Status, u.CreatedAt, u.UpdatedAt, u.BranchID)
	return err
}

func (s *PostgresStorage) UpdateStaffUser(ctx context.Context, u *domain.StaffUser) error {
	rolesBytes, _ := json.Marshal(u.Roles)
	isSuper := 0
	if u.IsSuperuser || u.Role == "SUPER_ADMIN" {
		isSuper = 1
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE staff_users SET full_name = $1, role = $2, roles = $3, is_superuser = $4, contact_phone = $5, status = $6, branch_id = $7, updated_at = NOW() WHERE id = $8
	`, u.FullName, u.Role, string(rolesBytes), isSuper, u.ContactPhone, u.Status, u.BranchID, u.ID)
	return err
}

func (s *PostgresStorage) UpdateStaffPassword(ctx context.Context, username, newPasswordHash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE staff_users SET password_hash = $1, updated_at = NOW() WHERE username = $2`, newPasswordHash, username)
	return err
}

func (s *PostgresStorage) GetStaffUserByUsername(ctx context.Context, username string) (*domain.StaffUser, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT s.id, s.username, s.password_hash, s.full_name, s.role, COALESCE(s.roles, '[]'), COALESCE(s.is_superuser, 0), s.contact_phone, s.status,
		       s.created_at, s.updated_at, s.branch_id, b.code
		FROM staff_users s
		LEFT JOIN branches b ON s.branch_id = b.id
		WHERE LOWER(s.username) = LOWER($1)
	`, strings.TrimSpace(username))
	var u domain.StaffUser
	var branchID, branchCode sql.NullString
	var rolesJSON string
	var isSuper int
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.FullName, &u.Role, &rolesJSON, &isSuper, &u.ContactPhone, &u.Status, &u.CreatedAt, &u.UpdatedAt, &branchID, &branchCode); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(rolesJSON), &u.Roles)
	if len(u.Roles) == 0 && u.Role != "" {
		u.Roles = []string{u.Role}
	}
	u.IsSuperuser = (isSuper == 1) || u.Role == "SUPER_ADMIN"
	if branchID.Valid {
		u.BranchID = &branchID.String
	}
	if branchCode.Valid {
		u.BranchCode = &branchCode.String
	}
	return &u, nil
}

func (s *PostgresStorage) CreateAuthSession(ctx context.Context, sess *domain.AuthSession) error {
	rolesBytes, _ := json.Marshal(sess.Roles)
	isSuper := 0
	if sess.IsSuperuser {
		isSuper = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_sessions (token, user_id, username, role, roles, is_superuser, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, sess.Token, sess.UserID, sess.Username, sess.Role, string(rolesBytes), isSuper, sess.ExpiresAt, sess.CreatedAt)
	return err
}

func (s *PostgresStorage) GetAuthSession(ctx context.Context, token string) (*domain.AuthSession, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT a.token, a.user_id, a.username, a.role, COALESCE(a.roles, '[]'), COALESCE(a.is_superuser, 0), a.expires_at, a.created_at,
		       s.branch_id, b.code
		FROM auth_sessions a
		LEFT JOIN staff_users s ON a.username = s.username
		LEFT JOIN branches b ON s.branch_id = b.id
		WHERE a.token = $1 AND a.expires_at > NOW()
	`, token)
	var sess domain.AuthSession
	var branchID, branchCode sql.NullString
	var rolesJSON string
	var isSuper int
	if err := row.Scan(&sess.Token, &sess.UserID, &sess.Username, &sess.Role, &rolesJSON, &isSuper, &sess.ExpiresAt, &sess.CreatedAt, &branchID, &branchCode); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(rolesJSON), &sess.Roles)
	if len(sess.Roles) == 0 && sess.Role != "" {
		sess.Roles = []string{sess.Role}
	}
	sess.IsSuperuser = (isSuper == 1) || sess.Role == "SUPER_ADMIN"
	if branchID.Valid {
		sess.BranchID = &branchID.String
	}
	if branchCode.Valid {
		sess.BranchCode = &branchCode.String
	}
	return &sess, nil
}

func (s *PostgresStorage) DeleteAuthSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE token = $1`, token)
	return err
}

func (s *PostgresStorage) GetExecutiveOverview(ctx context.Context, branchCode string) (*domain.ExecutiveOverview, error) {
	overview := &domain.ExecutiveOverview{
		BranchCode: branchCode,
	}

	var branchID *string
	if branchCode != "" && branchCode != "ALL" {
		if b, err := s.GetBranchByCode(ctx, branchCode); err == nil && b != nil {
			branchID = &b.ID
			overview.BranchName = b.Name
		}
	}
	if overview.BranchName == "" {
		if branchCode == "" || branchCode == "ALL" {
			overview.BranchName = "Seluruh Wilayah (Nasional)"
		} else {
			overview.BranchName = branchCode
		}
	}

	// 1. Total registrations, active, and suspended customers
	if branchID != nil {
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE branch_id = $1`, *branchID).Scan(&overview.TotalRegisteredCustomers)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'ACTIVE' AND branch_id = $1`, *branchID).Scan(&overview.TotalActiveCustomers)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'SUSPENDED' AND branch_id = $1`, *branchID).Scan(&overview.TotalSuspendedCustomers)
	} else {
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations`).Scan(&overview.TotalRegisteredCustomers)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'ACTIVE'`).Scan(&overview.TotalActiveCustomers)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'SUSPENDED'`).Scan(&overview.TotalSuspendedCustomers)
	}

	// 2. MRR & JARTAPLOK / Bitstream Wholesale Cost
	partnersMap := make(map[string]*domain.JartaplokPartner)
	if plist, err := s.ListJartaplokPartners(ctx, branchCode); err == nil {
		for i := range plist {
			partnersMap[plist[i].Code] = &plist[i]
		}
	}

	now := time.Now()
	var queryMRR string
	var argsMRR []interface{}
	if branchID != nil {
		queryMRR = `
			SELECT r.selected_plan_name, COALESCE(o.provider_id, 'GNET-BIARO'), r.status,
			       r.activated_at, r.created_at, r.suspended_at
			FROM registrations r
			LEFT JOIN odp_nodes o ON r.nearest_odp_code = o.code
			WHERE (r.status = 'ACTIVE' OR r.status = 'SUSPENDED') AND r.branch_id = $1
		`
		argsMRR = append(argsMRR, *branchID)
	} else {
		queryMRR = `
			SELECT r.selected_plan_name, COALESCE(o.provider_id, 'GNET-BIARO'), r.status,
			       r.activated_at, r.created_at, r.suspended_at
			FROM registrations r
			LEFT JOIN odp_nodes o ON r.nearest_odp_code = o.code
			WHERE r.status = 'ACTIVE' OR r.status = 'SUSPENDED'
		`
	}
	rows, err := s.db.QueryContext(ctx, queryMRR, argsMRR...)
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
	if branchID != nil {
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'ACTIVE' AND partner_code IS NOT NULL AND partner_code != '' AND branch_id = $1`, *branchID).Scan(&activePartnerRegs)
	} else {
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'ACTIVE' AND partner_code IS NOT NULL AND partner_code != ''`).Scan(&activePartnerRegs)
	}
	overview.TotalSalesCommission = int64(activePartnerRegs) * 50000
	overview.NetContributionMargin = overview.NetGrossMargin - overview.TotalSalesCommission

	// 4. ODP metrics
	if branchID != nil {
		_ = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*), COALESCE(SUM(total_ports), 0), COALESCE(SUM(used_ports), 0)
			FROM odp_nodes
			WHERE branch_id = $1
		`, *branchID).Scan(&overview.TotalODPs, &overview.TotalODPPorts, &overview.UsedODPPorts)
	} else {
		_ = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*), COALESCE(SUM(total_ports), 0), COALESCE(SUM(used_ports), 0)
			FROM odp_nodes
		`).Scan(&overview.TotalODPs, &overview.TotalODPPorts, &overview.UsedODPPorts)
	}
	overview.AvailableODPPorts = overview.TotalODPPorts - overview.UsedODPPorts
	targetARPU := overview.ARPU
	if targetARPU <= 0 {
		targetARPU = 250000
	}
	overview.PotentialHeadroomMRR = int64(overview.AvailableODPPorts) * targetARPU

	// 5. Work Orders
	if branchID != nil {
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_orders WHERE status = 'ASSIGNED' AND branch_id = $1`, *branchID).Scan(&overview.PendingWorkOrders)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_orders WHERE status = 'COMPLETED' AND branch_id = $1`, *branchID).Scan(&overview.CompletedWorkOrders)
	} else {
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_orders WHERE status = 'ASSIGNED'`).Scan(&overview.PendingWorkOrders)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_orders WHERE status = 'COMPLETED'`).Scan(&overview.CompletedWorkOrders)
	}

	return overview, nil
}

// ── JARTAPLOK PARTNERS B2B ──────────────────────────────────────

func (s *PostgresStorage) ListJartaplokPartners(ctx context.Context, branchCode string) ([]domain.JartaplokPartner, error) {
	branchCode = strings.TrimSpace(branchCode)
	query := `
		SELECT p.id, p.code, p.name, p.api_key, p.contact_phone, p.coverage_area,
		       COALESCE(p.service_type, 'SEWA_PORT_FO'), COALESCE(p.suspension_policy, 'ALLOWED_WITH_WAIVER'), COALESCE(p.pricing_model, 'Standard'),
		       COALESCE(p.rate_20m, 0), COALESCE(p.rate_30m, 0), COALESCE(p.rate_40m, 0),
		       COALESCE(p.rate_50m, 50000), COALESCE(p.rate_100m, 90000), COALESCE(p.rate_150m, 135000),
		       COALESCE(p.rate_200m, 180000), COALESCE(p.rate_300m, 270000),
		       COALESCE(p.otc_fee, 0), COALESCE(p.max_distance_meters, 250.0),
		       p.is_active, p.created_at, p.updated_at,
		       COALESCE((SELECT COUNT(*) FROM odp_nodes o WHERE o.provider_id = p.code), 0) as total_odps,
		       COALESCE((SELECT SUM(o.total_ports) FROM odp_nodes o WHERE o.provider_id = p.code), 0) as total_ports,
		       COALESCE((SELECT COUNT(*) FROM registrations r JOIN odp_nodes o ON r.nearest_odp_code = o.code WHERE o.provider_id = p.code AND r.status = 'ACTIVE'), 0) as active_ports,
		       p.branch_id::text, COALESCE(b.code, p.branch_code, 'ALL') AS branch_code
		FROM jartaplok_partners p
		LEFT JOIN branches b ON p.branch_id = b.id::text
	`
	var args []interface{}
	if branchCode != "" && branchCode != "ALL" {
		query += ` WHERE (p.branch_code = $1 OR b.code = $1 OR p.branch_id = $1)`
		args = append(args, branchCode)
	}
	query += ` ORDER BY p.created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	partners := make([]domain.JartaplokPartner, 0)
	for rows.Next() {
		var p domain.JartaplokPartner
		var isActive bool
		var bID, bCode *string
		if err := rows.Scan(
			&p.ID, &p.Code, &p.Name, &p.APIKey, &p.ContactPhone, &p.CoverageArea,
			&p.ServiceType, &p.SuspensionPolicy, &p.PricingModel,
			&p.Rate20M, &p.Rate30M, &p.Rate40M,
			&p.Rate50M, &p.Rate100M, &p.Rate150M,
			&p.Rate200M, &p.Rate300M,
			&p.OTCFee, &p.MaxDistanceMeters,
			&isActive, &p.CreatedAt, &p.UpdatedAt,
			&p.TotalODPs, &p.TotalPorts, &p.ActivePorts,
			&bID, &bCode,
		); err != nil {
			return nil, err
		}
		p.IsActive = isActive
		p.BranchID = bID
		p.BranchCode = bCode
		partners = append(partners, p)
	}
	return partners, nil
}

func (s *PostgresStorage) GetJartaplokPartnerByAPIKey(ctx context.Context, apiKey string) (*domain.JartaplokPartner, error) {
	var p domain.JartaplokPartner
	var isActive bool
	err := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, api_key, contact_phone, coverage_area,
		       COALESCE(service_type, 'SEWA_PORT_FO'), COALESCE(suspension_policy, 'ALLOWED_WITH_WAIVER'), COALESCE(pricing_model, 'Standard'),
		       COALESCE(rate_20m, 0), COALESCE(rate_30m, 0), COALESCE(rate_40m, 0),
		       COALESCE(rate_50m, 50000), COALESCE(rate_100m, 90000), COALESCE(rate_150m, 135000),
		       COALESCE(rate_200m, 180000), COALESCE(rate_300m, 270000),
		       COALESCE(otc_fee, 0), COALESCE(max_distance_meters, 250.0),
		       is_active, created_at, updated_at
		FROM jartaplok_partners
		WHERE api_key = $1 AND is_active = TRUE
	`, apiKey).Scan(
		&p.ID, &p.Code, &p.Name, &p.APIKey, &p.ContactPhone, &p.CoverageArea,
		&p.ServiceType, &p.SuspensionPolicy, &p.PricingModel,
		&p.Rate20M, &p.Rate30M, &p.Rate40M,
		&p.Rate50M, &p.Rate100M, &p.Rate150M,
		&p.Rate200M, &p.Rate300M,
		&p.OTCFee, &p.MaxDistanceMeters,
		&isActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.IsActive = isActive
	return &p, nil
}

func (s *PostgresStorage) GetJartaplokPartnerByCode(ctx context.Context, code string) (*domain.JartaplokPartner, error) {
	var p domain.JartaplokPartner
	var isActive bool
	err := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, api_key, contact_phone, coverage_area,
		       COALESCE(service_type, 'SEWA_PORT_FO'), COALESCE(suspension_policy, 'ALLOWED_WITH_WAIVER'), COALESCE(pricing_model, 'Standard'),
		       COALESCE(rate_20m, 0), COALESCE(rate_30m, 0), COALESCE(rate_40m, 0),
		       COALESCE(rate_50m, 50000), COALESCE(rate_100m, 90000), COALESCE(rate_150m, 135000),
		       COALESCE(rate_200m, 180000), COALESCE(rate_300m, 270000),
		       COALESCE(otc_fee, 0), COALESCE(max_distance_meters, 250.0),
		       is_active, created_at, updated_at
		FROM jartaplok_partners
		WHERE code = $1 OR id = $2
	`, code, code).Scan(
		&p.ID, &p.Code, &p.Name, &p.APIKey, &p.ContactPhone, &p.CoverageArea,
		&p.ServiceType, &p.SuspensionPolicy, &p.PricingModel,
		&p.Rate20M, &p.Rate30M, &p.Rate40M,
		&p.Rate50M, &p.Rate100M, &p.Rate150M,
		&p.Rate200M, &p.Rate300M,
		&p.OTCFee, &p.MaxDistanceMeters,
		&isActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.IsActive = isActive
	return &p, nil
}

func (s *PostgresStorage) CreateJartaplokPartner(ctx context.Context, p *domain.JartaplokPartner) error {
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

	if p.BranchCode != nil && *p.BranchCode != "" && *p.BranchCode != "ALL" && p.BranchID == nil {
		var bID string
		if err := s.db.QueryRowContext(ctx, `SELECT id FROM branches WHERE code = $1 LIMIT 1`, *p.BranchCode).Scan(&bID); err == nil {
			p.BranchID = &bID
		}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jartaplok_partners (
			id, code, name, api_key, contact_phone, coverage_area,
			service_type, suspension_policy, pricing_model, rate_20m, rate_30m, rate_40m,
			rate_50m, rate_100m, rate_150m, rate_200m, rate_300m,
			otc_fee, max_distance_meters, branch_id, branch_code, is_active, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19,
			$20, $21, TRUE, $22, $23
		)
	`,
		p.ID, p.Code, p.Name, p.APIKey, p.ContactPhone, p.CoverageArea,
		p.ServiceType, p.SuspensionPolicy, p.PricingModel, p.Rate20M, p.Rate30M, p.Rate40M,
		p.Rate50M, p.Rate100M, p.Rate150M, p.Rate200M, p.Rate300M,
		p.OTCFee, p.MaxDistanceMeters, p.BranchID, p.BranchCode, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *PostgresStorage) UpdateJartaplokPartner(ctx context.Context, p *domain.JartaplokPartner) error {
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
	var branchCodeVal, branchIDVal *string
	if p.BranchCode != nil && *p.BranchCode != "" {
		branchCodeVal = p.BranchCode
		var bID string
		_ = s.db.QueryRowContext(ctx, `SELECT id::text FROM branches WHERE code = $1 LIMIT 1`, *p.BranchCode).Scan(&bID)
		if bID != "" {
			branchIDVal = &bID
		}
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE jartaplok_partners
		SET name = $1, contact_phone = $2, coverage_area = $3,
		    service_type = $4, suspension_policy = $5, pricing_model = $6,
		    rate_20m = $7, rate_30m = $8, rate_40m = $9,
		    rate_50m = $10, rate_100m = $11, rate_150m = $12, rate_200m = $13, rate_300m = $14,
		    otc_fee = $15, max_distance_meters = $16,
		    is_active = $17,
		    api_key = COALESCE(NULLIF($18, ''), api_key),
		    branch_code = COALESCE($19, branch_code),
		    branch_id = COALESCE($20, branch_id),
		    updated_at = NOW()
		WHERE id = $21 OR code = $22
	`,
		p.Name, p.ContactPhone, p.CoverageArea,
		p.ServiceType, p.SuspensionPolicy, p.PricingModel,
		p.Rate20M, p.Rate30M, p.Rate40M,
		p.Rate50M, p.Rate100M, p.Rate150M, p.Rate200M, p.Rate300M,
		p.OTCFee, p.MaxDistanceMeters,
		p.IsActive, p.APIKey,
		branchCodeVal, branchIDVal,
		p.ID, p.Code,
	)
	if err != nil {
		return err
	}

	_, _ = s.db.ExecContext(ctx, `UPDATE odp_nodes SET provider_name = $1 WHERE provider_id = $2 OR provider_id = (SELECT code FROM jartaplok_partners WHERE id = $3)`, p.Name, p.Code, p.ID)
	return nil
}

func (s *PostgresStorage) GetJartaplokBillingSummary(ctx context.Context) (*domain.JartaplokBillingSummary, error) {
	return s.GetJartaplokBillingSummaryForPartner(ctx, "GNET-BIARO")
}

func (s *PostgresStorage) GetJartaplokBillingSummaryForPartner(ctx context.Context, partnerCode string) (*domain.JartaplokBillingSummary, error) {
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

	rows, err := s.db.QueryContext(ctx, `
		SELECT r.selected_plan_name, r.status, r.activated_at, r.created_at, r.suspended_at
		FROM registrations r
		JOIN odp_nodes o ON r.nearest_odp_code = o.code
		WHERE (r.status = 'ACTIVE' OR r.status = 'SUSPENDED') AND o.provider_id = $1
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
				} else if status == "SUSPENDED" {
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

	var totalODPs, totalPorts, usedPorts int
	_ = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(total_ports), 0), COALESCE(SUM(used_ports), 0)
		FROM odp_nodes
		WHERE provider_id = $1
	`, partnerCode).Scan(&totalODPs, &totalPorts, &usedPorts)

	summary.TotalODPs = totalODPs
	summary.TotalCapacityPorts = totalPorts
	summary.AvailablePorts = totalPorts - usedPorts
	if totalPorts > 0 {
		summary.OccupancyPct = (float64(usedPorts) / float64(totalPorts)) * 100.0
	}

	return summary, nil
}

func (s *PostgresStorage) GetJartaplokActivePorts(ctx context.Context) ([]domain.JartaplokActivePort, error) {
	return s.GetJartaplokActivePortsForPartner(ctx, "GNET-BIARO")
}

func (s *PostgresStorage) GetJartaplokActivePortsForPartner(ctx context.Context, partnerCode string) ([]domain.JartaplokActivePort, error) {
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
		WHERE (r.status = 'ACTIVE' OR r.status = 'SUSPENDED') AND o.provider_id = $1
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

func (s *PostgresStorage) GetCustomerPasswordHash(ctx context.Context, regID string) (string, error) {
	clean := strings.TrimSpace(regID)
	var hash string
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(customer_password_hash, '') 
		FROM registrations 
		WHERE id = $1 OR registration_no = $1
		LIMIT 1
	`, clean).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return hash, nil
}

func (s *PostgresStorage) UpdateCustomerPassword(ctx context.Context, phoneOrEmailOrID string, passwordHash string) error {
	clean := strings.TrimSpace(phoneOrEmailOrID)
	_, err := s.db.ExecContext(ctx, `
		UPDATE registrations 
		SET customer_password_hash = $1,
		    updated_at = NOW()
		WHERE id = $2 
		   OR registration_no = $2
		   OR (phone != '' AND phone = $2)
		   OR (email != '' AND LOWER(email) = LOWER($2))
	`, passwordHash, clean)
	return err
}

func (s *PostgresStorage) StoreCustomerOTP(ctx context.Context, phone string, otpCode string, expiresAt time.Time) error {
	cleanPhone := strings.TrimSpace(phone)
	id := fmt.Sprintf("otp-%d", time.Now().UnixNano())
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO customer_otps (id, phone, otp_code, expires_at, used, created_at)
		VALUES ($1, $2, $3, $4, FALSE, NOW())
	`, id, cleanPhone, otpCode, expiresAt)
	return err
}

func (s *PostgresStorage) VerifyCustomerOTP(ctx context.Context, phone string, otpCode string) (bool, error) {
	cleanPhone := strings.TrimSpace(phone)
	cleanCode := strings.TrimSpace(otpCode)
	var id string
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM customer_otps 
		WHERE phone = $1 AND otp_code = $2 AND used = FALSE AND expires_at > NOW()
		ORDER BY created_at DESC LIMIT 1
	`, cleanPhone, cleanCode).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE customer_otps SET used = TRUE WHERE id = $1`, id)
	return true, nil
}

func (s *PostgresStorage) GetStaffKPISummary(ctx context.Context, branchCode string) (*domain.StaffKPISummary, error) {
	now := time.Now()
	summary := &domain.StaffKPISummary{
		MonthYear:   now.Format("January 2006"),
		BranchCode:  branchCode,
		BranchName:  "Seluruh Wilayah (Nasional)",
		Technicians: []domain.TechnicianKPI{},
		Sales:       []domain.SalesKPI{},
	}

	var branchID *string
	cleanBranch := strings.ToUpper(strings.TrimSpace(branchCode))
	if cleanBranch != "" && cleanBranch != "ALL" {
		var bID, bName string
		err := s.db.QueryRowContext(ctx, `SELECT id, name FROM branches WHERE UPPER(code) = UPPER($1)`, cleanBranch).Scan(&bID, &bName)
		if err == nil {
			branchID = &bID
			summary.BranchName = bName
			summary.BranchCode = cleanBranch
		}
	}

	if branchID != nil {
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'ACTIVE' AND branch_id = $1`, *branchID).Scan(&summary.TotalActiveSubs)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_orders WHERE status = 'COMPLETED' AND branch_id = $1`, *branchID).Scan(&summary.TotalSPKDone)
		_ = s.db.QueryRowContext(ctx, `
			SELECT COALESCE(AVG(b.optical_power_dbm), -18.5) 
			FROM bast_reports b 
			JOIN work_orders w ON b.work_order_id = w.id 
			WHERE w.branch_id = $1
		`, *branchID).Scan(&summary.AvgTeamOpticalDBM)
	} else {
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM registrations WHERE status = 'ACTIVE'`).Scan(&summary.TotalActiveSubs)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_orders WHERE status = 'COMPLETED'`).Scan(&summary.TotalSPKDone)
		_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(AVG(optical_power_dbm), -18.5) FROM bast_reports`).Scan(&summary.AvgTeamOpticalDBM)
	}

	type techDef struct {
		Name    string
		Role    string
		Pattern string
	}

	var techList []techDef
	if branchID != nil {
		// Ambil staf teknisi yang terdaftar di cabang bersangkutan
		rowsTechUsers, err := s.db.QueryContext(ctx, `
			SELECT full_name, role 
			FROM staff_users 
			WHERE role = 'TECHNICIAN' AND branch_id = $1 AND status = 'ACTIVE'
			ORDER BY full_name ASC
		`, *branchID)
		if err == nil {
			defer rowsTechUsers.Close()
			for rowsTechUsers.Next() {
				var fn, r string
				if err := rowsTechUsers.Scan(&fn, &r); err == nil {
					techList = append(techList, techDef{
						Name:    fn,
						Role:    "Teknisi Lapangan (" + summary.BranchName + ")",
						Pattern: "%" + strings.ToLower(fn) + "%",
					})
				}
			}
		}

		// Tambahkan teknisi penugasan yang tercatat di work_orders cabang ini
		rowsTechWO, err := s.db.QueryContext(ctx, `
			SELECT DISTINCT technician_name 
			FROM work_orders 
			WHERE branch_id = $1 AND technician_name IS NOT NULL AND TRIM(technician_name) != ''
		`, *branchID)
		if err == nil {
			defer rowsTechWO.Close()
			for rowsTechWO.Next() {
				var rawName string
				if err := rowsTechWO.Scan(&rawName); err == nil {
					trimmed := strings.TrimSpace(rawName)
					if trimmed == "" {
						continue
					}
					matched := false
					for _, td := range techList {
						patClean := strings.ReplaceAll(strings.ToLower(td.Pattern), "%", "")
						if strings.Contains(strings.ToLower(trimmed), patClean) || strings.Contains(patClean, strings.ToLower(trimmed)) {
							matched = true
							break
						}
					}
					if !matched {
						techList = append(techList, techDef{
							Name:    trimmed,
							Role:    "Teknisi Lapangan (" + summary.BranchName + ")",
							Pattern: "%" + strings.ToLower(trimmed) + "%",
						})
					}
				}
			}
		}
	} else {
		techList = []techDef{
			{"Tim Teknisi Payakumbuh", "Tim Bersama / Bergilir", "%payakumbuh%"},
			{"Ricci", "Teknisi Lapangan (RICCI-TECH)", "%ricci%"},
			{"Zikka Sintio Anugrah", "Teknisi Lapangan (ZIKKA-TECH)", "%zikka%"},
			{"Egi", "Teknisi Lapangan (EGI-TECH)", "%egi%"},
			{"Nando Azkia Putra S.Kom", "Koordinator & Core FO", "%nando%"},
			{"Melkias", "Teknisi Lapangan Papua", "%melkias%"},
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
	}

	for _, t := range techList {
		tkpi := domain.TechnicianKPI{
			Name: t.Name,
			Role: t.Role,
		}

		if branchID != nil {
			_ = s.db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM work_orders 
				WHERE LOWER(technician_name) LIKE LOWER($1) AND branch_id = $2
			`, t.Pattern, *branchID).Scan(&tkpi.AssignedOrders)

			_ = s.db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM work_orders 
				WHERE LOWER(technician_name) LIKE LOWER($1) AND status = 'COMPLETED' AND branch_id = $2
			`, t.Pattern, *branchID).Scan(&tkpi.CompletedOrders)

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
				WHERE LOWER(w.technician_name) LIKE LOWER($1) AND w.branch_id = $2
			`, t.Pattern, *branchID).Scan(&avgDBM, &totalMeters)

			if avgDBM.Valid {
				tkpi.AvgOpticalPowerDBM = avgDBM.Float64
			} else {
				tkpi.AvgOpticalPowerDBM = summary.AvgTeamOpticalDBM
			}
			if totalMeters.Valid {
				tkpi.TotalDropcoreMeters = int(totalMeters.Int64)
			}
		} else {
			_ = s.db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM work_orders 
				WHERE LOWER(technician_name) LIKE LOWER($1)
			`, t.Pattern).Scan(&tkpi.AssignedOrders)

			_ = s.db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM work_orders 
				WHERE LOWER(technician_name) LIKE LOWER($1) AND status = 'COMPLETED'
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
				WHERE LOWER(w.technician_name) LIKE LOWER($1)
			`, t.Pattern).Scan(&avgDBM, &totalMeters)

			if avgDBM.Valid {
				tkpi.AvgOpticalPowerDBM = avgDBM.Float64
			} else {
				tkpi.AvgOpticalPowerDBM = summary.AvgTeamOpticalDBM
			}
			if totalMeters.Valid {
				tkpi.TotalDropcoreMeters = int(totalMeters.Int64)
			}
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
	var rowsSales *sql.Rows
	var qErr error
	if branchID != nil {
		rowsSales, qErr = s.db.QueryContext(ctx, `
			SELECT 
				p.code,
				p.name,
				COALESCE(p.commission_rate, 50000),
				COUNT(r.id) AS total_leads,
				COUNT(CASE WHEN r.status = 'ACTIVE' THEN 1 END) AS active_customers
			FROM partners p
			LEFT JOIN registrations r ON (UPPER(TRIM(r.partner_code)) = UPPER(TRIM(p.code)) AND r.branch_id = $1)
			WHERE p.is_active = true AND p.code NOT LIKE 'TEST-%' 
			  AND (p.branch_id = $1 OR ($2 = 'PAPUA' AND UPPER(p.code) LIKE '%PAPUA%') OR ($2 = 'PYK' AND (p.branch_id IS NULL OR UPPER(p.code) NOT LIKE '%PAPUA%')))
			GROUP BY p.code, p.name, p.commission_rate
			ORDER BY active_customers DESC, total_leads DESC, p.code ASC
		`, *branchID, cleanBranch)
	} else {
		rowsSales, qErr = s.db.QueryContext(ctx, `
			SELECT 
				p.code,
				p.name,
				COALESCE(p.commission_rate, 50000),
				COUNT(r.id) AS total_leads,
				COUNT(CASE WHEN r.status = 'ACTIVE' THEN 1 END) AS active_customers
			FROM partners p
			LEFT JOIN registrations r ON (UPPER(TRIM(r.partner_code)) = UPPER(TRIM(p.code)))
			WHERE p.is_active = true AND p.code NOT LIKE 'TEST-%'
			GROUP BY p.code, p.name, p.commission_rate
			ORDER BY active_customers DESC, total_leads DESC, p.code ASC
		`)
	}

	if qErr == nil && rowsSales != nil {
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

func (s *PostgresStorage) ListBranches(ctx context.Context) ([]domain.Branch, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, code, name, type, COALESCE(city, ''), COALESCE(province, ''),
		       COALESCE(revenue_share_type, 'PERCENTAGE'), COALESCE(share_percent_partner, 70.0),
		       COALESCE(flat_fee_per_sub, 35000), is_active, created_at, updated_at
		FROM branches
		ORDER BY CASE WHEN code = 'PYK' THEN 0 ELSE 1 END, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query branches: %w", err)
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

func (s *PostgresStorage) GetBranchByID(ctx context.Context, id string) (*domain.Branch, error) {
	var b domain.Branch
	err := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, type, COALESCE(city, ''), COALESCE(province, ''),
		       COALESCE(revenue_share_type, 'PERCENTAGE'), COALESCE(share_percent_partner, 70.0),
		       COALESCE(flat_fee_per_sub, 35000), is_active, created_at, updated_at
		FROM branches WHERE id = $1
	`, id).Scan(
		&b.ID, &b.Code, &b.Name, &b.Type, &b.City, &b.Province,
		&b.RevenueShareType, &b.SharePercentPartner, &b.FlatFeePerSub,
		&b.IsActive, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *PostgresStorage) GetBranchByCode(ctx context.Context, code string) (*domain.Branch, error) {
	var b domain.Branch
	err := s.db.QueryRowContext(ctx, `
		SELECT id, code, name, type, COALESCE(city, ''), COALESCE(province, ''),
		       COALESCE(revenue_share_type, 'PERCENTAGE'), COALESCE(share_percent_partner, 70.0),
		       COALESCE(flat_fee_per_sub, 35000), is_active, created_at, updated_at
		FROM branches WHERE UPPER(code) = UPPER($1)
	`, strings.TrimSpace(code)).Scan(
		&b.ID, &b.Code, &b.Name, &b.Type, &b.City, &b.Province,
		&b.RevenueShareType, &b.SharePercentPartner, &b.FlatFeePerSub,
		&b.IsActive, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

