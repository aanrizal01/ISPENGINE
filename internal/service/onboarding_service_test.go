package service_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"isp-onboarding/internal/billingclient"
	"isp-onboarding/internal/domain"
	"isp-onboarding/internal/repository"
	"isp-onboarding/internal/service"
)

func setupTestService(t *testing.T) (*service.OnboardingService, repository.Storage) {
	testDB := "test_onboarding.db"
	_ = os.Remove(testDB)

	repo, err := repository.NewSQLiteStorage(testDB)
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}

	t.Cleanup(func() {
		_ = repo.Close()
		_ = os.Remove(testDB)
	})

	client := billingclient.New("http://localhost:8080", "test-token")
	svc := service.NewOnboardingService(repo, client, 250.0) // 250 meter limit
	return svc, repo
}

func TestCoverageCheck(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	// Demo ODP-CKR-001 ada di -6.2088, 106.8456
	// Uji titik dekat (sekitar 50 meter)
	cov, err := svc.CheckCoverage(ctx, -6.2089, 106.8458)
	if err != nil {
		t.Fatalf("CheckCoverage error: %v", err)
	}

	if !cov.IsCovered {
		t.Errorf("Expected location to be covered, got not covered. Distance: %.2f", cov.DistanceMeters)
	}
	if cov.NearestODP == nil {
		t.Fatalf("Expected nearest ODP to not be nil")
	}
	if cov.NearestODP.Code != "ODP-CKR-001" {
		t.Errorf("Expected ODP-CKR-001, got %s", cov.NearestODP.Code)
	}

	// Uji titik jauh (> 1 km)
	farCov, err := svc.CheckCoverage(ctx, -6.2200, 106.8600)
	if err != nil {
		t.Fatalf("CheckCoverage far error: %v", err)
	}
	if farCov.IsCovered {
		t.Errorf("Expected far location to not be covered, got covered")
	}
}

func TestSubmitRegistrationAndWorkOrder(t *testing.T) {
	svc, repo := setupTestService(t)
	ctx := context.Background()

	partner, err := repo.GetPartnerByAPIKey(ctx, "part_official_key_9988")
	if err != nil {
		t.Fatalf("failed to get partner: %v", err)
	}

	// 1. Submit registrasi
	reg, err := svc.SubmitRegistration(ctx, partner, domain.SubmitRegistrationRequest{
		FullName:         "Ahmad Zaki",
		Email:            "ahmad@example.com",
		Phone:            "081234567890",
		IDCardNumber:     "3216000000000001",
		Address:          "Jl. Merpati No 15",
		Latitude:         -6.2089,
		Longitude:        106.8458,
		SelectedPlanID:   "plan-home-50m",
		SelectedPlanName: "Giga Home 50 Mbps",
	})
	if err != nil {
		t.Fatalf("SubmitRegistration failed: %v", err)
	}

	if reg.RegistrationNo == "" {
		t.Errorf("Expected non-empty registration number")
	}
	if reg.Status != "SUBMITTED" {
		t.Errorf("Expected status SUBMITTED, got %s", reg.Status)
	}

	// 2. Terbitkan Work Order Instalasi
	wo, err := svc.CreateWorkOrder(ctx, reg.ID, "INSTALLATION", "Budi Santoso (Teknisi)", time.Now().Add(24*time.Hour), "Bawa kabel 100m")
	if err != nil {
		t.Fatalf("CreateWorkOrder failed: %v", err)
	}

	if wo.OrderNo == "" {
		t.Errorf("Expected non-empty WO number")
	}

	// 3. Submit BAST dengan redaman buruk (< -27 dBm) -> harus error validasi
	_, err = svc.SubmitBASTAndPromote(ctx, wo.ID, domain.SubmitBASTRequest{
		OpticalPowerDBM: -29.5, // Buruk
		ONTSerialNumber: "ZTEGC0123456",
		ONTMACAddress:   "AA:BB:CC:DD:EE:FF",
	})
	if err == nil {
		t.Errorf("Expected error for optical power worse than -27 dBm, but got nil")
	}

	// 4. Submit BAST dengan redaman bagus (-18.5 dBm) -> harus sukses
	updatedReg, err := svc.SubmitBASTAndPromote(ctx, wo.ID, domain.SubmitBASTRequest{
		OpticalPowerDBM:      -18.5,
		ONTSerialNumber:      "ZTEGC0123456",
		ONTMACAddress:        "AA:BB:CC:DD:EE:FF",
		DropcoreLengthMeters: 85,
		SpeedtestDownMbps:    49.8,
		SpeedtestUpMbps:      48.5,
	})
	if err != nil {
		t.Fatalf("SubmitBASTAndPromote failed: %v", err)
	}

	if updatedReg.Status != "ACTIVE" {
		t.Errorf("Expected status ACTIVE, got %s", updatedReg.Status)
	}
}

func TestProrataAndJedaLayanan(t *testing.T) {
	svc, repo := setupTestService(t)
	ctx := context.Background()

	partner, err := repo.GetPartnerByAPIKey(ctx, "part_official_key_9988")
	if err != nil {
		t.Fatalf("failed to get partner: %v", err)
	}

	// 1. Submit registrasi & aktifkan
	reg, err := svc.SubmitRegistration(ctx, partner, domain.SubmitRegistrationRequest{
		FullName:         "Dedi Mizwar",
		Email:            "dedi@example.com",
		Phone:            "081299988877",
		IDCardNumber:     "3216000000000002",
		Address:          "Jl. Merpati No 16",
		Latitude:         -6.2089,
		Longitude:        106.8458,
		SelectedPlanID:   "plan-home-50m",
		SelectedPlanName: "Paket FAST 50 Mbps",
	})
	if err != nil {
		t.Fatalf("SubmitRegistration failed: %v", err)
	}

	wo, err := svc.CreateWorkOrder(ctx, reg.ID, "INSTALLATION", "Budi Santoso", time.Now(), "Pemasangan")
	if err != nil {
		t.Fatalf("CreateWorkOrder failed: %v", err)
	}

	updatedReg, err := svc.SubmitBASTAndPromote(ctx, wo.ID, domain.SubmitBASTRequest{
		OpticalPowerDBM:      -18.0,
		ONTSerialNumber:      "ONT-TEST-9999",
		ONTMACAddress:        "AA:11:22:33:44:55",
		DropcoreLengthMeters: 50,
	})
	if err != nil {
		t.Fatalf("SubmitBASTAndPromote failed: %v", err)
	}

	nearestCodeStr := ""
	if updatedReg.NearestODPCode != nil {
		nearestCodeStr = *updatedReg.NearestODPCode
	}
	t.Logf("updatedReg: status=%s, nearestCode=%s", updatedReg.Status, nearestCodeStr)

	// 2. Cek billing prorata bulan pertama
	billing, err := repo.GetJartaplokBillingSummaryForPartner(ctx, "GNET-BIARO")
	if err != nil {
		t.Fatalf("GetJartaplokBillingSummaryForPartner error: %v", err)
	}

	if billing.TotalActivePorts < 1 {
		t.Errorf("Expected at least 1 active port, got %d", billing.TotalActivePorts)
	}

	ports, err := repo.GetJartaplokActivePortsForPartner(ctx, "GNET-BIARO")
	if err != nil {
		t.Fatalf("GetJartaplokActivePortsForPartner error: %v", err)
	}

	var foundPort *domain.JartaplokActivePort
	for i := range ports {
		if ports[i].PackageSpeed == "50 Mbps" {
			foundPort = &ports[i]
			break
		}
	}
	if foundPort == nil {
		t.Fatalf("Expected to find 50 Mbps active port in partner list")
	}

	if !foundPort.IsProrated {
		t.Errorf("Expected first-month port to be marked is_prorated=true")
	}
	if foundPort.ProratedFee <= 0 || foundPort.ProratedFee > foundPort.MonthlyRental {
		t.Errorf("Expected prorated fee between 0 and %d, got %d", foundPort.MonthlyRental, foundPort.ProratedFee)
	}

	// 3. Uji Jeda Layanan (Service Pause / Suspension)
	suspendedReg, err := svc.SuspendRegistration(ctx, reg.ID, "Cuti Luar Kota 1 Bulan")
	if err != nil {
		t.Fatalf("SuspendRegistration failed: %v", err)
	}
	if suspendedReg.Status != "SUSPENDED" {
		t.Errorf("Expected status SUSPENDED, got %s", suspendedReg.Status)
	}
	if suspendedReg.SuspensionReason != "Cuti Luar Kota 1 Bulan" {
		t.Errorf("Expected reason 'Cuti Luar Kota 1 Bulan', got '%s'", suspendedReg.SuspensionReason)
	}

	// Cek status di partner portal
	billingAfterSusp, err := repo.GetJartaplokBillingSummaryForPartner(ctx, "GNET-BIARO")
	if err != nil {
		t.Fatalf("GetJartaplokBillingSummary error: %v", err)
	}
	if billingAfterSusp.TotalSuspendedPorts < 1 {
		t.Errorf("Expected at least 1 suspended port, got %d", billingAfterSusp.TotalSuspendedPorts)
	}

	// 4. Uji Pengaktifan Kembali (Resume)
	resumedReg, err := svc.ResumeRegistration(ctx, reg.ID)
	if err != nil {
		t.Fatalf("ResumeRegistration failed: %v", err)
	}
	if resumedReg.Status != "ACTIVE" {
		t.Errorf("Expected status ACTIVE after resume, got %s", resumedReg.Status)
	}
}

func TestSuspensionPolicyDisallowedAndFullBilling(t *testing.T) {
	svc, repo := setupTestService(t)
	ctx := context.Background()

	// 1. Daftarkan mitra TIF dengan kebijakan DISALLOWED
	tifPartner := &domain.JartaplokPartner{
		Code:             "TIF-TEST",
		Name:             "PT Telkom Infrastruktur Indonesia",
		APIKey:           "tif_test_key_1",
		ServiceType:      "BITSTREAM",
		SuspensionPolicy: domain.SuspensionPolicyDisallowed,
		Rate50M:          200000,
	}
	if err := repo.CreateJartaplokPartner(ctx, tifPartner); err != nil {
		t.Fatalf("Failed to create TIF partner: %v", err)
	}

	// Buat ODP milik TIF di dekat lokasi registrasi
	_, err := repo.BatchUpsertODPs(ctx, "TIF-TEST", "PT Telkom Infrastruktur Indonesia", []domain.ODPNode{
		{
			Code:        "ODP-TIF-001",
			Name:        "Tiang Bitstream TIF 001",
			Latitude:    -6.2089,
			Longitude:   106.8458,
			TotalPorts:  8,
			Status:      "AVAILABLE",
			ClusterArea: "Jakarta",
		},
	})
	if err != nil {
		t.Fatalf("BatchUpsertODPs error: %v", err)
	}

	partner, err := repo.GetPartnerByCode(ctx, "PARTNER-OFFICIAL")
	if err != nil {
		t.Fatalf("failed to get partner: %v", err)
	}

	// Daftarkan pelanggan dekat ODP TIF
	reg, err := svc.SubmitRegistration(ctx, partner, domain.SubmitRegistrationRequest{
		FullName:         "Pelanggan Area Bitstream TIF",
		Phone:            "081299998888",
		IDCardNumber:     "1307080101990009",
		Address:          "Jl. Sudirman",
		Latitude:         -6.2089,
		Longitude:        106.8458,
		SelectedPlanID:   "plan-home-50m",
		SelectedPlanName: "Paket FAST 50 Mbps",
	})
	if err != nil {
		t.Fatalf("SubmitRegistration failed: %v", err)
	}

	// Buat WO & BAST agar sirkuit ACTIVE
	wo, err := svc.CreateWorkOrder(ctx, reg.ID, "INSTALLATION", "Teknisi Lapangan", time.Now(), "Pasang")
	if err != nil {
		t.Fatalf("CreateWorkOrder failed: %v", err)
	}

	if _, err := svc.SubmitBASTAndPromote(ctx, wo.ID, domain.SubmitBASTRequest{
		OpticalPowerDBM:      -20.5,
		ONTSerialNumber:      "ZTEG12345678",
		ONTMACAddress:        "AA:BB:CC:DD:EE:FF",
		DropcoreLengthMeters: 50,
	}); err != nil {
		t.Fatalf("SubmitBASTAndPromote failed: %v", err)
	}

	// 2. Coba suspend sirkuit TIF -> HARUS GAGAL karena DISALLOWED
	_, err = svc.SuspendRegistration(ctx, reg.ID, "Minta Cuti 1 Bulan")
	if err == nil {
		t.Fatalf("Expected SuspendRegistration to fail on DISALLOWED partner, but succeeded!")
	}
	t.Logf("Sukses ditolak sesuai kebijakan DISALLOWED: %v", err)

	// 3. Ubah kebijakan mitra TIF menjadi ALLOWED_FULL_BILLING
	tifPartner.SuspensionPolicy = domain.SuspensionPolicyAllowedFullBilling
	if err := repo.UpdateJartaplokPartner(ctx, tifPartner); err != nil {
		t.Fatalf("UpdateJartaplokPartner failed: %v", err)
	}

	// 4. Coba suspend sirkuit TIF lagi -> HARUS BERHASIL
	suspendedReg, err := svc.SuspendRegistration(ctx, reg.ID, "Renovasi Rumah")
	if err != nil {
		t.Fatalf("Expected SuspendRegistration to succeed under ALLOWED_FULL_BILLING, got: %v", err)
	}
	if suspendedReg.Status != "SUSPENDED" {
		t.Errorf("Expected status SUSPENDED, got %s", suspendedReg.Status)
	}

	// 5. Cek tagihan wholesale TIF -> KARENA FULL BILLING, tagihan tetap berjalan penuh
	billingTIF, err := repo.GetJartaplokBillingSummaryForPartner(ctx, "TIF-TEST")
	if err != nil {
		t.Fatalf("GetJartaplokBillingSummaryForPartner error: %v", err)
	}
	if billingTIF.TotalBillingAmount <= 0 {
		t.Errorf("Expected TotalBillingAmount > 0 under ALLOWED_FULL_BILLING even when suspended, got %d", billingTIF.TotalBillingAmount)
	}
	t.Logf("Billing TIF under full billing: amount=%d, full=%d", billingTIF.TotalBillingAmount, billingTIF.TotalFullMonthly)
}

func TestAttachPartnerODPAndUncovered(t *testing.T) {
	svc, repo := setupTestService(t)
	ctx := context.Background()

	// 1. Daftarkan mitra TIF
	tif := &domain.JartaplokPartner{
		Code:             "TELKO-PYK",
		Name:             "PT Telkom Infrastruktur Indonesia",
		APIKey:           "key-tif",
		ServiceType:      "BITSTREAM",
		SuspensionPolicy: domain.SuspensionPolicyDisallowed,
		Rate50M:          200000,
		IsActive:         true,
	}
	_ = repo.CreateJartaplokPartner(ctx, tif)

	// 2. Buat pendaftaran pelanggan di lokasi yang jauh dari ODP GNET
	regReq := domain.SubmitRegistrationRequest{
		FullName:         "Warga Luar Coverage GNET",
		Email:            "luar@example.com",
		Phone:            "081234567890",
		IDCardNumber:     "1376012345678999",
		Address:          "Jl. Pedalaman No. 99, Payakumbuh",
		Latitude:         -0.9999,
		Longitude:        100.9999,
		SelectedPlanID:   "plan-50",
		SelectedPlanName: "50 Mbps",
	}
	reg, err := svc.SubmitRegistration(ctx, nil, regReq)
	if err != nil {
		t.Fatalf("SubmitRegistration failed: %v", err)
	}
	if reg.Status != "PENDING_SURVEY_OVERDISTANCE" {
		t.Errorf("Expected status PENDING_SURVEY_OVERDISTANCE, got %s", reg.Status)
	}

	// 3. Test Attach Manual ke TIF (karena TIF tidak beri KML)
	attachReq := domain.AttachPartnerODPRequest{
		PartnerCode:    "TELKO-PYK",
		ODPCode:        "ODP-TIF-PYK-AUTO01",
		ODPName:        "Tiang Telkom Depan Kantor Lurah",
		DistanceMeters: 65.0,
		Notes:          "Konfirmasi survei mandiri bersama PIC Telkom Payakumbuh",
	}
	attachedReg, err := svc.AttachPartnerODP(ctx, reg.ID, attachReq)
	if err != nil {
		t.Fatalf("AttachPartnerODP failed: %v", err)
	}

	if attachedReg.Status != "INSTALLATION_SCHEDULED" {
		t.Errorf("Expected status INSTALLATION_SCHEDULED after attach, got %s", attachedReg.Status)
	}
	if attachedReg.NearestODPCode == nil || *attachedReg.NearestODPCode != "ODP-TIF-PYK-AUTO01" {
		t.Errorf("Expected NearestODPCode 'ODP-TIF-PYK-AUTO01', got %v", attachedReg.NearestODPCode)
	}
	if attachedReg.PartnerName != "PT Telkom Infrastruktur Indonesia" {
		t.Errorf("Expected PartnerName 'PT Telkom Infrastruktur Indonesia', got '%s'", attachedReg.PartnerName)
	}
	if attachedReg.PartnerSuspensionPolicy != domain.SuspensionPolicyDisallowed {
		t.Errorf("Expected PartnerSuspensionPolicy DISALLOWED, got '%s'", attachedReg.PartnerSuspensionPolicy)
	}

	// 4. Test Uncovered handling pada pelanggan lain
	reg2Req := domain.SubmitRegistrationRequest{
		FullName:         "Warga Terisolir",
		Email:            "isolir@example.com",
		Phone:            "081234567888",
		IDCardNumber:     "1376012345678888",
		Address:          "Hutan Pinus No. 1",
		Latitude:         -1.5,
		Longitude:        101.5,
		SelectedPlanID:   "plan-50",
		SelectedPlanName: "50 Mbps",
	}
	reg2, err := svc.SubmitRegistration(ctx, nil, reg2Req)
	if err != nil {
		t.Fatalf("SubmitRegistration 2 failed: %v", err)
	}

	// Tandai Wishlist
	uncoveredReg, err := svc.MarkUncovered(ctx, reg2.ID, domain.RejectUncoveredRequest{
		Action: "WISHLIST",
		Reason: "Hasil cek GNET & TIF tidak ada tiang dalam radius 500m",
	})
	if err != nil {
		t.Fatalf("MarkUncovered failed: %v", err)
	}
	if uncoveredReg.Status != "UNCOVERED_WISHLIST" {
		t.Errorf("Expected status UNCOVERED_WISHLIST, got %s", uncoveredReg.Status)
	}

	// Tandai Cancel
	cancelledReg, err := svc.MarkUncovered(ctx, reg2.ID, domain.RejectUncoveredRequest{
		Action: "CANCEL",
		Reason: "Pelanggan membatalkan permohonan karena tidak ingin menunggu perluasan",
	})
	if err != nil {
		t.Fatalf("MarkUncovered cancel failed: %v", err)
	}
	if cancelledReg.Status != "CANCELLED_NO_COVERAGE" {
		t.Errorf("Expected status CANCELLED_NO_COVERAGE, got %s", cancelledReg.Status)
	}
}

func TestPartnerListAndClaimCorporateRegistration(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	// 1. Ambil 2 Sales Partner bawaan (Andi dan Siti)
	p1, err := svc.GetPartnerByCode(ctx, "ANDI-PYK")
	if err != nil || p1 == nil {
		t.Fatalf("Partner Andi not found: %v", err)
	}
	p2, err := svc.GetPartnerByCode(ctx, "SITI-PYK")
	if err != nil || p2 == nil {
		t.Fatalf("Partner Siti not found: %v", err)
	}

	// 2. Submit retail registration dengan referral Andi
	_, err = svc.SubmitRegistration(ctx, p1, domain.SubmitRegistrationRequest{
		FullName:         "Pelanggan Retail Andi",
		Email:            "retail.andi@example.com",
		Phone:            "0811111111",
		IDCardNumber:     "1376011111111111",
		Address:          "Jl. Sudirman No. 10",
		Latitude:         -0.2241,
		Longitude:        100.6315,
		SelectedPlanID:   "paket-gold-10m",
		SelectedPlanName: "Paket Gold",
	})
	if err != nil {
		t.Fatalf("Submit retail reg failed: %v", err)
	}

	// 3. Submit corporate/enterprise registration tanpa referral (Direct Business Inquiry)
	corpReg, err := svc.SubmitRegistration(ctx, nil, domain.SubmitRegistrationRequest{
		FullName:         "PT. Semen Padang Raya",
		Email:            "procurement@spr.co.id",
		Phone:            "0822222222",
		IDCardNumber:     "1376022222222222",
		Address:          "Kawasan Industri Payakumbuh",
		Latitude:         -0.2241,
		Longitude:        100.6315,
		SelectedPlanID:   "paket-custom-enterprise",
		SelectedPlanName: "Paket Custom / Corporate & Dedicated",
		CustomNotes:      "Peruntukan: Kantor | Bandwidth: 500 Mbps | IP Statis: Ya",
	})
	if err != nil {
		t.Fatalf("Submit corporate reg failed: %v", err)
	}

	// 4. Cek list untuk Andi: harus melihat 2 items (1 retail miliknya + 1 open corporate lead)
	regsAndi, err := svc.ListRegistrationsForPartner(ctx, p1.ID)
	if err != nil {
		t.Fatalf("ListRegistrationsForPartner Andi failed: %v", err)
	}
	if len(regsAndi) != 2 {
		t.Fatalf("Expected Andi to see 2 registrations (1 own + 1 open corporate), got %d", len(regsAndi))
	}

	// 5. Cek list untuk Siti: harus melihat 1 item (open corporate lead, TIDAK melihat retail Andi)
	regsSiti, err := svc.ListRegistrationsForPartner(ctx, p2.ID)
	if err != nil {
		t.Fatalf("ListRegistrationsForPartner Siti failed: %v", err)
	}
	if len(regsSiti) != 1 {
		t.Fatalf("Expected Siti to see 1 open corporate registration, got %d", len(regsSiti))
	}
	if regsSiti[0].RegistrationNo != corpReg.RegistrationNo {
		t.Errorf("Expected Siti to see corporate reg %s, got %s", corpReg.RegistrationNo, regsSiti[0].RegistrationNo)
	}

	// 6. Siti mengklaim lead corporate tersebut
	claimedReg, err := svc.ClaimRegistration(ctx, corpReg.RegistrationNo, p2.ID)
	if err != nil {
		t.Fatalf("ClaimRegistration failed: %v", err)
	}
	if claimedReg.PartnerID == nil || *claimedReg.PartnerID != p2.ID {
		t.Errorf("Expected partner_id to be %s, got %v", p2.ID, claimedReg.PartnerID)
	}
	if claimedReg.PartnerCode == nil || *claimedReg.PartnerCode != p2.Code {
		t.Errorf("Expected partner_code to be %s, got %v", p2.Code, claimedReg.PartnerCode)
	}

	// 7. Setelah diklaim oleh Siti:
	// - Siti melihat 1 item (milik dia sekarang)
	// - Andi melihat 1 item (hanya retail Andi, karena lead corporate sudah diklaim Siti)
	regsAndiAfter, _ := svc.ListRegistrationsForPartner(ctx, p1.ID)
	if len(regsAndiAfter) != 1 {
		t.Errorf("Expected Andi to see only 1 registration after claim, got %d", len(regsAndiAfter))
	}

	regsSitiAfter, _ := svc.ListRegistrationsForPartner(ctx, p2.ID)
	if len(regsSitiAfter) != 1 {
		t.Errorf("Expected Siti to see 1 claimed registration, got %d", len(regsSitiAfter))
	}
}

func TestUpdateRegistrationPricing(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	// 1. Submit registrasi custom corporate
	reg, err := svc.SubmitRegistration(ctx, nil, domain.SubmitRegistrationRequest{
		FullName:         "PT. Semen Padang Raya",
		Email:            "procurement@spr.co.id",
		Phone:            "0822222222",
		IDCardNumber:     "1376022222222222",
		Address:          "Kawasan Industri Payakumbuh",
		Latitude:         -0.2241,
		Longitude:        100.6315,
		SelectedPlanID:   "paket-custom-enterprise",
		SelectedPlanName: "Paket Custom / Corporate & Dedicated",
		CustomNotes:      "Peruntukan: Kantor | Bandwidth: 300 Mbps | IP Statis: Ya",
	})
	if err != nil {
		t.Fatalf("Submit corporate reg failed: %v", err)
	}

	// 2. Update penawaran biaya OTC & tarif bulanan
	pricingReq := domain.UpdateRegistrationPricingRequest{
		OTCFee:       1500000,
		MonthlyPrice: 2500000,
		OTCNotes:     "Termasuk router MikroTik CCR, penarikan FO 350m, 1 IP Public Statis /30",
	}

	updated, err := svc.UpdateRegistrationPricing(ctx, reg.RegistrationNo, pricingReq)
	if err != nil {
		t.Fatalf("UpdateRegistrationPricing failed: %v", err)
	}

	if updated.OTCFee != 1500000 {
		t.Errorf("Expected OTCFee 1500000, got %d", updated.OTCFee)
	}
	if updated.MonthlyPrice != 2500000 {
		t.Errorf("Expected MonthlyPrice 2500000, got %d", updated.MonthlyPrice)
	}
	if updated.OTCNotes != pricingReq.OTCNotes {
		t.Errorf("Expected OTCNotes '%s', got '%s'", pricingReq.OTCNotes, updated.OTCNotes)
	}

	// 3. Verifikasi saat di-fetch ulang dari repo
	fetched, err := svc.GetRegistrationByNo(ctx, reg.RegistrationNo)
	if err != nil {
		t.Fatalf("GetRegistrationByNo failed: %v", err)
	}
	if fetched.OTCFee != 1500000 || fetched.MonthlyPrice != 2500000 {
		t.Errorf("Fetched pricing mismatch: OTC=%d, Monthly=%d", fetched.OTCFee, fetched.MonthlyPrice)
	}
}

func TestNPWPOptionalAndPricingSync(t *testing.T) {
	svc, _ := setupTestService(t)
	ctx := context.Background()

	// 1. Submit registrasi dengan NPWP terisi
	reg, err := svc.SubmitRegistration(ctx, nil, domain.SubmitRegistrationRequest{
		FullName:         "PT. Jaya Mandiri Perkasa",
		Email:            "finance@jayamandiri.co.id",
		Phone:            "081333444555",
		IDCardNumber:     "1376033333333333",
		TaxID:            "01.234.567.8-012.000",
		Address:          "Jl. Tan Malaka No. 88, Payakumbuh",
		Latitude:         -0.2241,
		Longitude:        100.6315,
		SelectedPlanID:   "paket-custom-enterprise",
		SelectedPlanName: "Custom Enterprise",
		CustomNotes:      "Peruntukan: Kantor | Bandwidth: 200 Mbps",
	})
	if err != nil {
		t.Fatalf("SubmitRegistration failed: %v", err)
	}

	if reg.TaxID != "01.234.567.8-012.000" {
		t.Errorf("Expected TaxID '01.234.567.8-012.000', got '%s'", reg.TaxID)
	}

	// 2. Fetch ulang dari database
	fetched, err := svc.GetRegistrationByNo(ctx, reg.RegistrationNo)
	if err != nil {
		t.Fatalf("GetRegistrationByNo failed: %v", err)
	}
	if fetched.TaxID != "01.234.567.8-012.000" {
		t.Errorf("Fetched TaxID mismatch, got '%s'", fetched.TaxID)
	}

	// 3. Update NPWP via UpdateRegistrationPricing jika ada revisi NPWP 16-digit baru
	updated, err := svc.UpdateRegistrationPricing(ctx, reg.RegistrationNo, domain.UpdateRegistrationPricingRequest{
		OTCFee:       1000000,
		MonthlyPrice: 1800000,
		OTCNotes:     "Pemasangan FO 150m",
		TaxID:        "0123456789012345",
	})
	if err != nil {
		t.Fatalf("UpdateRegistrationPricing with new TaxID failed: %v", err)
	}
	if updated.TaxID != "0123456789012345" {
		t.Errorf("Expected updated TaxID '0123456789012345', got '%s'", updated.TaxID)
	}
}

func TestUpgradeBandwidth(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService(t)

	// Create test registration
	reg, err := svc.SubmitRegistration(ctx, nil, domain.SubmitRegistrationRequest{
		FullName:         "Budi Test Upgrade",
		Email:            "budi.upgrade@example.com",
		Phone:            "08123456789",
		IDCardNumber:     "1371012345670001",
		Address:          "Jl. Sudirman No 10 Payakumbuh",
		Latitude:         -0.228,
		Longitude:        100.630,
		SelectedPlanID:   "paket-diamond-20m",
		SelectedPlanName: "Paket Diamond 20M",
	})
	if err != nil {
		t.Fatalf("SubmitRegistration failed: %v", err)
	}

	// Upgrade from 20M to 50M
	upgraded, err := svc.UpgradeBandwidth(ctx, reg.RegistrationNo, domain.UpgradeBandwidthRequest{
		NewPlanID:       "paket-honor-50m",
		NewPlanName:     "Paket Honor 50M",
		NewMonthlyPrice: 389000,
		EffectiveDate:   "2026-10-01",
		Notes:           "Permintaan pelanggan naik kecepatan",
	})
	if err != nil {
		t.Fatalf("UpgradeBandwidth failed: %v", err)
	}

	if upgraded.SelectedPlanID != "paket-honor-50m" {
		t.Errorf("Expected plan ID 'paket-honor-50m', got '%s'", upgraded.SelectedPlanID)
	}
	if upgraded.SelectedPlanName != "Paket Honor 50M" {
		t.Errorf("Expected plan name 'Paket Honor 50M', got '%s'", upgraded.SelectedPlanName)
	}
	if upgraded.MonthlyPrice != 389000 {
		t.Errorf("Expected monthly price 389000, got %d", upgraded.MonthlyPrice)
	}
	if !strings.Contains(upgraded.DispatchNotes, "Upgrade Paket") {
		t.Errorf("Expected dispatch_notes to contain upgrade audit log, got: %s", upgraded.DispatchNotes)
	}
}

func TestDualCredentialMapping(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService(t)

	// Daftarkan mitra TIF
	tif := &domain.JartaplokPartner{
		Code:             "TELKO-PYK",
		Name:             "PT Telkom Infrastruktur Indonesia",
		APIKey:           "key-tif",
		ServiceType:      "BITSTREAM",
		SuspensionPolicy: domain.SuspensionPolicyDisallowed,
		Rate50M:          200000,
		IsActive:         true,
	}
	_ = repo.CreateJartaplokPartner(ctx, tif)

	// 1. Submit Registration
	regReq := domain.SubmitRegistrationRequest{
		FullName:         "Pak Budi Santoso",
		Email:            "budi@example.com",
		Phone:            "081299998888",
		IDCardNumber:     "1376019999888800",
		Address:          "Jl. Soekarno Hatta No. 10",
		Latitude:         -0.22,
		Longitude:        100.63,
		SelectedPlanID:   "paket-home-30m",
		SelectedPlanName: "Paket Home 30M",
	}
	reg, err := svc.SubmitRegistration(ctx, nil, regReq)
	if err != nil {
		t.Fatalf("SubmitRegistration failed: %v", err)
	}

	canonicalUser := reg.PPPoEUsername
	canonicalPass := reg.PPPoEPassword
	if canonicalUser == "" || canonicalPass == "" {
		t.Fatalf("Expected canonical PPPoE credentials to be generated, got user=%s pass=%s", canonicalUser, canonicalPass)
	}

	// 2. Attach Partner ODP with Upstream PPPoE credentials (TIF / Telkom)
	attachReq := domain.AttachPartnerODPRequest{
		PartnerCode:           "TELKO-PYK",
		ODPCode:               "ODP-TIF-PYK-099",
		ODPName:               "Tiang Telkom Soehatta 10",
		DistanceMeters:        45.0,
		Notes:                 "Attach ke TIF bitstream",
		UpstreamPPPoEUsername: "111400888999@telkom",
		UpstreamPPPoEPassword: "PasswordONT99",
	}
	attached, err := svc.AttachPartnerODP(ctx, reg.ID, attachReq)
	if err != nil {
		t.Fatalf("AttachPartnerODP failed: %v", err)
	}

	// Verify canonical credentials unchanged
	if attached.PPPoEUsername != canonicalUser || attached.PPPoEPassword != canonicalPass {
		t.Errorf("Canonical PPPoE credentials should remain unchanged! Expected %s got %s", canonicalUser, attached.PPPoEUsername)
	}
	// Verify upstream credentials recorded
	if attached.UpstreamPPPoEUsername != "111400888999@telkom" {
		t.Errorf("Expected UpstreamPPPoEUsername '111400888999@telkom', got '%s'", attached.UpstreamPPPoEUsername)
	}
	if attached.UpstreamPPPoEPassword != "PasswordONT99" {
		t.Errorf("Expected UpstreamPPPoEPassword 'PasswordONT99', got '%s'", attached.UpstreamPPPoEPassword)
	}

	// 3. Update Upstream PPPoE credentials directly (e.g. from Detail Modal)
	updated, err := svc.UpdateUpstreamPPPoE(ctx, reg.ID, domain.UpdateUpstreamPPPoERequest{
		UpstreamPPPoEUsername: "111400999000@telkom",
		UpstreamPPPoEPassword: "NewSecretPassword123",
	})
	if err != nil {
		t.Fatalf("UpdateUpstreamPPPoE failed: %v", err)
	}

	if updated.PPPoEUsername != canonicalUser {
		t.Errorf("Canonical PPPoE username changed unexpectedly to '%s'", updated.PPPoEUsername)
	}
	if updated.UpstreamPPPoEUsername != "111400999000@telkom" {
		t.Errorf("Expected updated UpstreamPPPoEUsername '111400999000@telkom', got '%s'", updated.UpstreamPPPoEUsername)
	}
	if updated.UpstreamPPPoEPassword != "NewSecretPassword123" {
		t.Errorf("Expected updated UpstreamPPPoEPassword 'NewSecretPassword123', got '%s'", updated.UpstreamPPPoEPassword)
	}
}






