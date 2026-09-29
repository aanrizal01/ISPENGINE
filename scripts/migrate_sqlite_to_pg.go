package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func main() {
	sqlitePath := flag.String("sqlite", "onboarding.db", "Path to source SQLite file")
	pgDSN := flag.String("pg", "postgres://postgres:postgres_dev_password@localhost:5432/isp_billing?sslmode=disable", "Target PostgreSQL DSN")
	flag.Parse()

	log.Printf("=== MIGRATOR SQLITE -> POSTGRESQL (POSTGIS) ===")
	log.Printf("Source SQLite : %s", *sqlitePath)
	log.Printf("Target PG DSN : %s", *pgDSN)

	// 1. Open SQLite
	sqliteDB, err := sql.Open("sqlite", *sqlitePath)
	if err != nil {
		log.Fatalf("Gagal membuka SQLite: %v", err)
	}
	defer sqliteDB.Close()

	// 2. Open PostgreSQL
	pgDB, err := sql.Open("pgx", *pgDSN)
	if err != nil {
		log.Fatalf("Gagal membuka PostgreSQL: %v", err)
	}
	defer pgDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := pgDB.PingContext(ctx); err != nil {
		log.Fatalf("Koneksi PostgreSQL gagal: %v", err)
	}

	log.Printf("Koneksi SQLite & PostgreSQL berhasil terhubung.")

	// Migrasi 1: Jartaplok Partners
	migrateJartaplokPartners(ctx, sqliteDB, pgDB)

	// Migrasi 2: Partners
	migratePartners(ctx, sqliteDB, pgDB)

	// Migrasi 3: ODP Nodes (GIS Point terisi otomatis oleh trigger)
	migrateODPNodes(ctx, sqliteDB, pgDB)

	// Migrasi 4: Registrations (GIS Point & PPPoE terisi otomatis)
	migrateRegistrations(ctx, sqliteDB, pgDB)

	// Migrasi 5: Work Orders
	migrateWorkOrders(ctx, sqliteDB, pgDB)

	// Migrasi 6: BAST Reports
	migrateBastReports(ctx, sqliteDB, pgDB)

	// Migrasi 7: Staff Users
	migrateStaffUsers(ctx, sqliteDB, pgDB)

	// Migrasi 8: Auth Sessions
	migrateAuthSessions(ctx, sqliteDB, pgDB)

	log.Printf("==================================================")
	log.Printf("MIGRASI DATA KE POSTGRESQL SELESAI DENGAN SUKSES!")
	log.Printf("==================================================")
}

func migrateJartaplokPartners(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx, `
		SELECT id, code, name, api_key, contact_phone, coverage_area,
		       COALESCE(service_type, 'SEWA_PORT_FO'), COALESCE(suspension_policy, 'ALLOWED_WITH_WAIVER'), COALESCE(pricing_model, 'Standard'),
		       COALESCE(rate_20m, 0), COALESCE(rate_30m, 0), COALESCE(rate_40m, 0),
		       COALESCE(rate_50m, 50000), COALESCE(rate_100m, 90000), COALESCE(rate_150m, 135000),
		       COALESCE(rate_200m, 180000), COALESCE(rate_300m, 270000),
		       COALESCE(otc_fee, 0), COALESCE(max_distance_meters, 250.0),
		       is_active, created_at, updated_at
		FROM jartaplok_partners
	`)
	if err != nil {
		log.Printf("[JartaplokPartners] Skip / query error: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, code, name, apiKey, phone, area, sType, suspPol, pModel string
		var r20, r30, r40, r50, r100, r150, r200, r300, otc int64
		var maxDist float64
		var activeInt int
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&id, &code, &name, &apiKey, &phone, &area, &sType, &suspPol, &pModel, &r20, &r30, &r40, &r50, &r100, &r150, &r200, &r300, &otc, &maxDist, &activeInt, &createdAt, &updatedAt); err != nil {
			log.Printf("[JartaplokPartners] Scan error: %v", err)
			continue
		}

		_, err := dst.ExecContext(ctx, `
			INSERT INTO jartaplok_partners (
				id, code, name, api_key, contact_phone, coverage_area,
				service_type, suspension_policy, pricing_model, rate_20m, rate_30m, rate_40m,
				rate_50m, rate_100m, rate_150m, rate_200m, rate_300m, otc_fee, max_distance_meters,
				is_active, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
			) ON CONFLICT (code) DO UPDATE SET
				name = EXCLUDED.name,
				contact_phone = EXCLUDED.contact_phone,
				coverage_area = EXCLUDED.coverage_area,
				updated_at = NOW();
		`, id, code, name, apiKey, phone, area, sType, suspPol, pModel, r20, r30, r40, r50, r100, r150, r200, r300, otc, maxDist, activeInt == 1, createdAt, updatedAt)
		if err != nil {
			log.Printf("[JartaplokPartners] Insert %s error: %v", code, err)
		} else {
			count++
		}
	}
	log.Printf("[JartaplokPartners] Berhasil migrasi %d mitra", count)
}

func migratePartners(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx, `SELECT id, code, name, COALESCE(api_key, ''), COALESCE(commission_rate, 0.0), COALESCE(contact_phone, ''), is_active, created_at FROM partners`)
	if err != nil {
		log.Printf("[Partners] Skip / query error: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, code, name, apiKey, phone string
		var comm float64
		var activeInt int
		var createdAt time.Time

		if err := rows.Scan(&id, &code, &name, &apiKey, &comm, &phone, &activeInt, &createdAt); err != nil {
			log.Printf("[Partners] Scan error: %v", err)
			continue
		}

		// Ensure valid UUID for Postgres uuid column
		var validUUID string
		if _, err := uuid.Parse(id); err != nil {
			validUUID = uuid.NewString()
		} else {
			validUUID = id
		}

		_, err := dst.ExecContext(ctx, `
			INSERT INTO partners (id, code, name, api_key, commission_rate, contact_phone, is_active, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'ACTIVE', $8, $8)
			ON CONFLICT (code) DO UPDATE SET
				name = EXCLUDED.name,
				api_key = EXCLUDED.api_key,
				commission_rate = EXCLUDED.commission_rate,
				contact_phone = EXCLUDED.contact_phone,
				is_active = EXCLUDED.is_active,
				updated_at = NOW();
		`, validUUID, code, name, apiKey, comm, phone, activeInt == 1, createdAt)
		if err != nil {
			log.Printf("[Partners] Insert %s error: %v", code, err)
		} else {
			count++
		}
	}
	log.Printf("[Partners] Berhasil migrasi %d partners", count)
}

func migrateODPNodes(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx, `
		SELECT id, code, name, latitude, longitude, total_ports, used_ports, status, cluster_area,
		       COALESCE(provider_id, 'GNET-BIARO'), COALESCE(provider_name, 'PT. GNET BIARO AKSES'),
		       created_at, updated_at
		FROM odp_nodes
	`)
	if err != nil {
		log.Printf("[ODPNodes] Skip / query error: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, code, name, status, area, provID, provName string
		var lat, lng float64
		var totalPorts, usedPorts int
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&id, &code, &name, &lat, &lng, &totalPorts, &usedPorts, &status, &area, &provID, &provName, &createdAt, &updatedAt); err != nil {
			continue
		}

		_, err := dst.ExecContext(ctx, `
			INSERT INTO odp_nodes (
				id, code, name, latitude, longitude, total_ports, used_ports, status, cluster_area, provider_id, provider_name, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (code) DO UPDATE SET
				name = EXCLUDED.name,
				latitude = EXCLUDED.latitude,
				longitude = EXCLUDED.longitude,
				total_ports = EXCLUDED.total_ports,
				used_ports = EXCLUDED.used_ports,
				cluster_area = EXCLUDED.cluster_area,
				provider_id = EXCLUDED.provider_id,
				provider_name = EXCLUDED.provider_name,
				updated_at = NOW();
		`, id, code, name, lat, lng, totalPorts, usedPorts, status, area, provID, provName, createdAt, updatedAt)
		if err == nil {
			count++
		}
	}
	log.Printf("[ODPNodes] Berhasil migrasi %d titik ODP ke GIS PostGIS", count)
}

func migrateRegistrations(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx, `
		SELECT id, registration_no,
		       COALESCE(partner_id, ''), COALESCE(partner_code, ''),
		       full_name, COALESCE(email, ''), phone, id_card_number, COALESCE(tax_id, ''),
		       address, latitude, longitude,
		       selected_plan_id, selected_plan_name,
		       COALESCE(nearest_odp_id, ''), COALESCE(nearest_odp_code, ''),
		       COALESCE(distance_to_odp_meters, 0.0),
		       status,
		       COALESCE(ktp_photo_url, ''), COALESCE(house_photo_url, ''),
		       COALESCE(site_pic_name, ''), COALESCE(site_pic_phone, ''),
		       COALESCE(contract_signature_url, ''), contract_signed_at,
		       COALESCE(gigabill_customer_id, ''), COALESCE(gigabill_subscription_id, ''),
		       activated_at, suspended_at,
		       COALESCE(suspension_reason, ''), COALESCE(dispatch_notes, ''), COALESCE(custom_notes, ''),
		       COALESCE(otc_fee, 0), COALESCE(monthly_price, 0), COALESCE(otc_notes, ''),
		       COALESCE(pppoe_username, ''), COALESCE(pppoe_password, ''),
		       created_at, updated_at
		FROM registrations
	`)
	if err != nil {
		log.Printf("[Registrations] Skip / query error: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, regNo, partnerID, partnerCode, fullName, email, phone, idCard, taxID, address, planID, planName, odpID, odpCode, status string
		var ktpURL, houseURL, sitePicName, sitePicPhone, contractSig, gigaCustID, gigaSubID, suspReason, dispNotes, customNotes, otcNotes, pppoeUser, pppoePass string
		var lat, lng, dist float64
		var otcFee, monthlyPrice int64
		var contractSignedAt, actAt, suspAt sql.NullTime
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&id, &regNo, &partnerID, &partnerCode, &fullName, &email, &phone, &idCard, &taxID,
			&address, &lat, &lng, &planID, &planName, &odpID, &odpCode,
			&dist, &status, &ktpURL, &houseURL, &sitePicName, &sitePicPhone, &contractSig, &contractSignedAt, &gigaCustID, &gigaSubID,
			&actAt, &suspAt, &suspReason, &dispNotes, &customNotes, &otcFee, &monthlyPrice, &otcNotes, &pppoeUser, &pppoePass,
			&createdAt, &updatedAt,
		)
		if err != nil {
			log.Printf("[Registrations] Scan error: %v", err)
			continue
		}

		_, err = dst.ExecContext(ctx, `
			INSERT INTO registrations (
				id, registration_no, partner_id, partner_code, full_name, email, phone, id_card_number, tax_id,
				address, latitude, longitude, selected_plan_id, selected_plan_name, nearest_odp_id, nearest_odp_code,
				distance_to_odp_meters, status, ktp_photo_url, house_photo_url, site_pic_name, site_pic_phone,
				contract_signature_url, contract_signed_at, gigabill_customer_id, gigabill_subscription_id,
				activated_at, suspended_at, suspension_reason, dispatch_notes, custom_notes, otc_fee, monthly_price, otc_notes,
				pppoe_username, pppoe_password, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9,
				$10, $11, $12, $13, $14, $15, $16,
				$17, $18, $19, $20, $21, $22,
				$23, $24, $25, $26,
				$27, $28, $29, $30, $31, $32, $33, $34,
				$35, $36, $37, $38
			) ON CONFLICT (registration_no) DO UPDATE SET
				full_name = EXCLUDED.full_name,
				email = EXCLUDED.email,
				phone = EXCLUDED.phone,
				address = EXCLUDED.address,
				latitude = EXCLUDED.latitude,
				longitude = EXCLUDED.longitude,
				selected_plan_id = EXCLUDED.selected_plan_id,
				selected_plan_name = EXCLUDED.selected_plan_name,
				nearest_odp_id = EXCLUDED.nearest_odp_id,
				nearest_odp_code = EXCLUDED.nearest_odp_code,
				distance_to_odp_meters = EXCLUDED.distance_to_odp_meters,
				pppoe_username = EXCLUDED.pppoe_username,
				pppoe_password = EXCLUDED.pppoe_password,
				status = EXCLUDED.status,
				gigabill_customer_id = EXCLUDED.gigabill_customer_id,
				gigabill_subscription_id = EXCLUDED.gigabill_subscription_id,
				updated_at = NOW();
		`,
			id, regNo, partnerID, partnerCode, fullName, email, phone, idCard, taxID,
			address, lat, lng, planID, planName, odpID, odpCode,
			dist, status, ktpURL, houseURL, sitePicName, sitePicPhone,
			contractSig, contractSignedAt, gigaCustID, gigaSubID,
			actAt, suspAt, suspReason, dispNotes, customNotes, otcFee, monthlyPrice, otcNotes,
			pppoeUser, pppoePass, createdAt, updatedAt,
		)
		if err != nil {
			log.Printf("[Registrations] Insert %s error: %v", regNo, err)
		} else {
			count++
		}
	}
	log.Printf("[Registrations] Berhasil migrasi %d pelanggan", count)
}

func migrateStaffUsers(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx, `SELECT id, username, full_name, role, contact_phone, status, password_hash, created_at, updated_at FROM staff_users`)
	if err != nil {
		log.Printf("[StaffUsers] Skip / query error: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, username, fullName, role, phone, status, passHash string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &username, &fullName, &role, &phone, &status, &passHash, &createdAt, &updatedAt); err != nil {
			continue
		}

		_, err := dst.ExecContext(ctx, `
			INSERT INTO staff_users (id, username, full_name, role, contact_phone, status, password_hash, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (username) DO UPDATE SET full_name = EXCLUDED.full_name;
		`, id, username, fullName, role, phone, status, passHash, createdAt, updatedAt)
		if err == nil {
			count++
		}
	}
	log.Printf("[StaffUsers] Berhasil migrasi %d staff", count)
}

func migrateWorkOrders(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx, `SELECT id, order_no, registration_id, type, technician_name, scheduled_at, status, notes, created_at, updated_at FROM work_orders`)
	if err != nil {
		log.Printf("[WorkOrders] Skip / query error: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, orderNo, regID, woType, techName, status, notes string
		var schedAt, createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &orderNo, &regID, &woType, &techName, &schedAt, &status, &notes, &createdAt, &updatedAt); err != nil {
			continue
		}

		_, err := dst.ExecContext(ctx, `
			INSERT INTO work_orders (id, order_no, registration_id, type, technician_name, scheduled_at, status, notes, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (order_no) DO UPDATE SET status = EXCLUDED.status, updated_at = NOW();
		`, id, orderNo, regID, woType, techName, schedAt, status, notes, createdAt, updatedAt)
		if err == nil {
			count++
		}
	}
	log.Printf("[WorkOrders] Berhasil migrasi %d SPK (Work Orders)", count)
}

func migrateBastReports(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx, `
		SELECT id, work_order_id, optical_power_dbm, ont_serial_number, ont_mac_address,
		       dropcore_length_meters, customer_signature_url, proof_photo_url, COALESCE(house_photo_url, ''),
		       speedtest_down_mbps, speedtest_up_mbps, notes, created_at
		FROM bast_reports
	`)
	if err != nil {
		log.Printf("[BastReports] Skip / query error: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, woID, ontSN, ontMAC, custSig, proofURL, houseURL, notes string
		var opPower, speedDown, speedUp float64
		var dropLength int
		var createdAt time.Time

		if err := rows.Scan(&id, &woID, &opPower, &ontSN, &ontMAC, &dropLength, &custSig, &proofURL, &houseURL, &speedDown, &speedUp, &notes, &createdAt); err != nil {
			continue
		}

		_, err := dst.ExecContext(ctx, `
			INSERT INTO bast_reports (
				id, work_order_id, optical_power_dbm, ont_serial_number, ont_mac_address,
				dropcore_length_meters, customer_signature_url, proof_photo_url, house_photo_url,
				speedtest_down_mbps, speedtest_up_mbps, notes, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (work_order_id) DO NOTHING;
		`, id, woID, opPower, ontSN, ontMAC, dropLength, custSig, proofURL, houseURL, speedDown, speedUp, notes, createdAt)
		if err == nil {
			count++
		}
	}
	log.Printf("[BastReports] Berhasil migrasi %d BAST Digital", count)
}

func migrateAuthSessions(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx, `SELECT token, user_id, username, role, expires_at, created_at FROM auth_sessions`)
	if err != nil {
		log.Printf("[AuthSessions] Skip / query error: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var token, userID, username, role string
		var expiresAt, createdAt time.Time
		if err := rows.Scan(&token, &userID, &username, &role, &expiresAt, &createdAt); err != nil {
			continue
		}

		_, err := dst.ExecContext(ctx, `
			INSERT INTO auth_sessions (token, user_id, username, role, expires_at, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (token) DO UPDATE SET expires_at = EXCLUDED.expires_at;
		`, token, userID, username, role, expiresAt, createdAt)
		if err == nil {
			count++
		}
	}
	log.Printf("[AuthSessions] Berhasil migrasi %d sesi login staf", count)
}

