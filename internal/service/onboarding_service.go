package service

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"math/rand"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"isp-onboarding/internal/billingclient"
	"isp-onboarding/internal/domain"
	"isp-onboarding/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type OnboardingService struct {
	repo              repository.Storage
	billingClient     *billingclient.Client
	maxCoverageMeters float64
}

func NewOnboardingService(repo repository.Storage, billingClient *billingclient.Client, maxCoverageMeters float64) *OnboardingService {
	return &OnboardingService{
		repo:              repo,
		billingClient:     billingClient,
		maxCoverageMeters: maxCoverageMeters,
	}
}

func (s *OnboardingService) CheckCoverage(ctx context.Context, lat, lng float64) (*domain.CoverageCheckResponse, error) {
	nearestODP, dist, err := s.repo.FindNearestAvailableODP(ctx, lat, lng, s.maxCoverageMeters*3)
	if err != nil {
		return nil, fmt.Errorf("error finding nearest odp: %w", err)
	}

	if nearestODP == nil {
		return &domain.CoverageCheckResponse{
			IsCovered:      false,
			DistanceMeters: 0,
			MaxDistance:    s.maxCoverageMeters,
			Message:        "Maaf, lokasi Anda belum terjangkau jaringan fiber optic kami saat ini.",
		}, nil
	}

	isCovered := dist <= s.maxCoverageMeters
	var msg string
	if isCovered {
		msg = fmt.Sprintf("Selamat! Lokasi Anda terjangkau ODP %s dengan jarak %.1f meter (Port Tersedia: %d).",
			nearestODP.Code, dist, nearestODP.AvailablePorts())
	} else {
		msg = fmt.Sprintf("Lokasi Anda berjarak %.1f meter dari ODP %s terdekat (melebihi batas standar %.0fm). Membutuhkan survei teknis jalur khusus.",
			dist, nearestODP.Code, s.maxCoverageMeters)
	}

	return &domain.CoverageCheckResponse{
		IsCovered:      isCovered,
		DistanceMeters: dist,
		MaxDistance:    s.maxCoverageMeters,
		NearestODP:     nearestODP,
		Message:        msg,
	}, nil
}

func (s *OnboardingService) SubmitRegistration(ctx context.Context, partner *domain.Partner, req domain.SubmitRegistrationRequest) (*domain.Registration, error) {
	// 1. Cek coverage
	cov, err := s.CheckCoverage(ctx, req.Latitude, req.Longitude)
	if err != nil {
		return nil, err
	}

	var nearestID, nearestCode *string
	var dist float64
	if cov.NearestODP != nil {
		nearestID = &cov.NearestODP.ID
		nearestCode = &cov.NearestODP.Code
		dist = cov.DistanceMeters
	}

	// 2. Penentuan Status Berdasarkan Jarak
	// Jika <= 250m: SUBMITTED (Siap jadwalkan instalasi)
	// Jika > 250m: PENDING_SURVEY_OVERDISTANCE (Tetap diterima, masuk antrean survei jalur khusus)
	status := "SUBMITTED"
	if !cov.IsCovered || dist > s.maxCoverageMeters {
		status = "PENDING_SURVEY_OVERDISTANCE"
	} else if req.SelectedPlanID == "paket-custom-enterprise" || req.SelectedPlanID == "custom-enterprise" || strings.Contains(strings.ToLower(req.SelectedPlanName), "custom") {
		// Pelanggan Custom Corporate & Dedicated selalu dijadwalkan survei B2B terlebih dahulu
		status = "SURVEY_SCHEDULED"
	}

	// 3. Generate Nomor Registrasi: REG-YYYYMM-XXXX
	regNo := fmt.Sprintf("REG-%s-%04d", time.Now().Format("200601"), rand.Intn(9000)+1000)

	var partnerID, partnerCode *string
	if partner != nil {
		partnerID = &partner.ID
		partnerCode = &partner.Code
	} else if strings.TrimSpace(req.ReferralCode) != "" {
		refCheck, err := s.CheckReferralCode(ctx, req.ReferralCode)
		if err == nil && refCheck != nil && refCheck.Valid {
			if refCheck.Type == "SALES_PARTNER" {
				partnerID = &refCheck.PartnerID
				canonicalCode := refCheck.Code
				partnerCode = &canonicalCode
			} else {
				canonicalCode := refCheck.Code
				partnerCode = &canonicalCode
			}
		} else {
			code := strings.ToUpper(strings.TrimSpace(req.ReferralCode))
			partnerCode = &code
		}
	}

	picName := strings.TrimSpace(req.SitePICName)
	picPhone := strings.TrimSpace(req.SitePICPhone)
	if picName == "" {
		picName = strings.TrimSpace(req.FullName)
	}
	if picPhone == "" {
		picPhone = strings.TrimSpace(req.Phone)
	}

	monthlyPrice := req.MonthlyPrice
	if monthlyPrice <= 0 && req.SelectedPlanID != "paket-custom-enterprise" && !strings.Contains(strings.ToLower(req.SelectedPlanName), "custom") {
		monthlyPrice = ResolveDefaultPlanPrice(req.SelectedPlanID, req.SelectedPlanName)
	}

	var branchID, branchCode *string
	if cov.NearestODP != nil && cov.NearestODP.BranchCode != nil && *cov.NearestODP.BranchCode != "" {
		branchID = cov.NearestODP.BranchID
		branchCode = cov.NearestODP.BranchCode
	} else if partner != nil && partner.BranchCode != nil && *partner.BranchCode != "" {
		branchID = partner.BranchID
		branchCode = partner.BranchCode
	} else if strings.TrimSpace(req.BranchCode) != "" {
		b := strings.ToUpper(strings.TrimSpace(req.BranchCode))
		branchCode = &b
	} else {
		addrUpper := strings.ToUpper(req.Address)
		if strings.Contains(addrUpper, "PAPUA") || strings.Contains(addrUpper, "JAYAPURA") || req.Longitude > 130.0 {
			papua := "PAPUA"
			branchCode = &papua
		} else {
			pyk := "PYK"
			branchCode = &pyk
		}
	}

	reg := &domain.Registration{
		RegistrationNo:      regNo,
		PartnerID:           partnerID,
		PartnerCode:         partnerCode,
		BranchID:            branchID,
		BranchCode:          branchCode,
		FullName:            strings.TrimSpace(req.FullName),
		Email:               strings.TrimSpace(req.Email),
		Phone:               strings.TrimSpace(req.Phone),
		IDCardNumber:        strings.TrimSpace(req.IDCardNumber),
		TaxID:               strings.TrimSpace(req.TaxID),
		Address:             strings.TrimSpace(req.Address),
		Latitude:            req.Latitude,
		Longitude:           req.Longitude,
		SelectedPlanID:      req.SelectedPlanID,
		SelectedPlanName:    req.SelectedPlanName,
		NearestODPID:        nearestID,
		NearestODPCode:      nearestCode,
		DistanceToODPMeters: dist,
		Status:              status,
		KTPPhotoURL:         req.KTPPhotoURL,
		HousePhotoURL:       req.HousePhotoURL,
		SitePICName:         picName,
		SitePICPhone:        picPhone,
		CustomNotes:         strings.TrimSpace(req.CustomNotes),
		MonthlyPrice:        monthlyPrice,
		OTCFee:              req.OTCFee,
	}

	// Otomatis wariskan KTP yang sudah terverifikasi dari registrasi sebelumnya jika pelanggan sama
	if reg.KTPPhotoURL == "" {
		lookupKeys := []string{reg.Email, reg.Phone, reg.IDCardNumber}
		for _, key := range lookupKeys {
			if key == "" {
				continue
			}
			if existingList, err := s.repo.ListRegistrationsByCustomer(ctx, key); err == nil {
				for _, ex := range existingList {
					if ex.KTPPhotoURL != "" {
						reg.KTPPhotoURL = ex.KTPPhotoURL
						break
					}
				}
			}
			if reg.KTPPhotoURL != "" {
				break
			}
		}
	}

	// 4. Generate Kredensial PPPoE GoGiga Resmi: [KodeCabang|KodeClusterWilayah|Urut@gogiga.net.id] & net[4_digit_terakhir_no_hp]
	branchPrefix := ResolveBranchPrefix(reg.BranchCode)
	clusterCode := "001"
	if cov.NearestODP != nil {
		clusterCode = ResolveClusterCode(cov.NearestODP.ClusterArea, cov.NearestODP.Code)
	}
	nextSeq, _ := s.repo.GetNextPPPoESequence(ctx, clusterCode)
	reg.PPPoEUsername = fmt.Sprintf("%s%s%05d@gogiga.net.id", branchPrefix, clusterCode, nextSeq)

	cleanPhone := regexp.MustCompile(`\D`).ReplaceAllString(strings.TrimSpace(req.Phone), "")
	pin := "8821"
	if len(cleanPhone) >= 4 {
		pin = cleanPhone[len(cleanPhone)-4:]
	} else if len(regNo) >= 4 {
		pin = regNo[len(regNo)-4:]
	}
	reg.PPPoEPassword = fmt.Sprintf("net%s", pin)

	if err := s.repo.CreateRegistration(ctx, reg); err != nil {
		return nil, fmt.Errorf("failed to save registration: %w", err)
	}

	// 4. Jika dalam jangkauan standar (<= 250m), kurangi ketersediaan port ODP
	if cov.NearestODP != nil && cov.IsCovered && dist <= s.maxCoverageMeters {
		_ = s.repo.IncrementODPUsedPort(ctx, cov.NearestODP.ID)
	}

	// 5. Otomatis terbitkan Surat Perintah Kerja (SPK) untuk Teknisi Lapangan
	woType := "INSTALLATION"
	if status == "PENDING_SURVEY_OVERDISTANCE" || status == "SURVEY_SCHEDULED" {
		woType = "SURVEY"
	}
	techTeam := "Tim Teknisi Payakumbuh"
	if reg.BranchCode != nil && *reg.BranchCode != "" && *reg.BranchCode != "PYK" {
		techTeam = fmt.Sprintf("Tim Teknisi Cabang %s", *reg.BranchCode)
	}
	wo, err := s.CreateWorkOrder(ctx, reg.ID, woType, techTeam, time.Now().Add(24*time.Hour), "Penugasan Pasang Baru Otomatis Sistem")
	if err == nil && wo != nil {
		reg.WorkOrderID = &wo.ID
		reg.WorkOrderNo = &wo.OrderNo
		reg.WorkOrderStatus = &wo.Status
		reg.TechnicianName = &wo.TechnicianName
	}

	// Otomatis daftarkan Customer & Subscription ke GOGIGABILL Core untuk menerbitkan Faktur Tagihan Awal
	_ = s.EnsureGigabillCustomerAndSubscription(ctx, reg)

	return reg, nil
}

func (s *OnboardingService) CreateWorkOrder(ctx context.Context, regID string, woType string, techName string, scheduledAt time.Time, notes string) (*domain.WorkOrder, error) {
	reg, err := s.repo.GetRegistrationByID(ctx, regID)
	if err != nil {
		return nil, fmt.Errorf("registration not found: %w", err)
	}

	// Idempotency: Cegah pembuatan SPK ganda jika registrasi sudah memiliki Work Order
	if existingWO, err := s.repo.GetWorkOrderByRegistrationID(ctx, reg.ID); err == nil && existingWO != nil {
		return existingWO, nil
	}

	orderNo := fmt.Sprintf("WO-%s-%04d", time.Now().Format("200601"), rand.Intn(9000)+1000)
	wo := &domain.WorkOrder{
		OrderNo:        orderNo,
		RegistrationID: reg.ID,
		Type:           woType,
		TechnicianName: techName,
		ScheduledAt:    scheduledAt,
		Status:         "ASSIGNED",
		Notes:          notes,
	}

	if err := s.repo.CreateWorkOrder(ctx, wo); err != nil {
		return nil, fmt.Errorf("failed to create work order: %w", err)
	}

	// Update status registrasi HANYA jika pelanggan belum aktif atau selesai instalasi
	if reg.Status != "ACTIVE" && reg.Status != "INSTALLED" {
		newStatus := "SURVEY_SCHEDULED"
		if woType == "INSTALLATION" {
			newStatus = "INSTALLATION_SCHEDULED"
		}
		_ = s.repo.UpdateRegistrationStatus(ctx, reg.ID, newStatus, nil, nil)
	}

	return wo, nil
}

func (s *OnboardingService) SubmitBASTAndPromote(ctx context.Context, workOrderID string, req domain.SubmitBASTRequest) (*domain.Registration, error) {
	wo, err := s.repo.GetWorkOrderByID(ctx, workOrderID)
	if err != nil {
		return nil, fmt.Errorf("work order not found: %w", err)
	}

	reg, err := s.repo.GetRegistrationByID(ctx, wo.RegistrationID)
	if err != nil {
		return nil, fmt.Errorf("registration not found: %w", err)
	}

	// Validasi Redaman Optik (Standar GPON: -14 dBm s/d -25 dBm, batas maksimal -27 dBm)
	if req.OpticalPowerDBM < -27.0 {
		return nil, fmt.Errorf("redaman optik terlalu buruk (%.2f dBm). Batas toleransi maksimal adalah -27.0 dBm. Mohon perbaiki sambungan/splicing sebelum submit BAST", req.OpticalPowerDBM)
	}

	bast := &domain.BASTReport{
		WorkOrderID:          workOrderID,
		OpticalPowerDBM:      req.OpticalPowerDBM,
		ONTSerialNumber:      req.ONTSerialNumber,
		ONTMACAddress:        req.ONTMACAddress,
		DropcoreLengthMeters: req.DropcoreLengthMeters,
		CustomerSignatureURL: req.CustomerSignatureURL,
		ProofPhotoURL:        req.ProofPhotoURL,
		HousePhotoURL:        req.HousePhotoURL,
		SpeedtestDownMbps:    req.SpeedtestDownMbps,
		SpeedtestUpMbps:      req.SpeedtestUpMbps,
		UpstreamPPPoEUsername: strings.TrimSpace(req.UpstreamPPPoEUsername),
		UpstreamPPPoEPassword: strings.TrimSpace(req.UpstreamPPPoEPassword),
		Notes:                 req.Notes,
		TechnicianName:        strings.TrimSpace(req.TechnicianName),
	}

	if err := s.repo.SaveBAST(ctx, bast); err != nil {
		return nil, fmt.Errorf("failed to save BAST: %w", err)
	}

	if req.UpstreamPPPoEUsername != "" {
		reg.UpstreamPPPoEUsername = strings.TrimSpace(req.UpstreamPPPoEUsername)
		reg.UpstreamPPPoEPassword = strings.TrimSpace(req.UpstreamPPPoEPassword)
		_ = s.repo.UpdateRegistrationUpstreamPPPoE(ctx, reg.RegistrationNo, reg.UpstreamPPPoEUsername, reg.UpstreamPPPoEPassword)
	}

	// Penyesuaian Titik ODP Aktual di Lapangan (jika teknisi mencolok ke ODP yang berbeda dari rekomendasi awal peta)
	actualODPCode := strings.ToUpper(strings.TrimSpace(req.ActualODPCode))
	if actualODPCode != "" {
		currentODPCode := ""
		if reg.NearestODPCode != nil {
			currentODPCode = strings.ToUpper(strings.TrimSpace(*reg.NearestODPCode))
		}
		if currentODPCode == "" || currentODPCode != actualODPCode {
			newODP, err := s.repo.GetODPByCode(ctx, actualODPCode)
			if err == nil && newODP != nil {
				// Kurangi port ODP lama jika berbeda
				if reg.NearestODPID != nil && *reg.NearestODPID != "" && *reg.NearestODPID != newODP.ID {
					_ = s.repo.DecrementODPUsedPort(ctx, *reg.NearestODPID)
				}
				// Tambah port ODP baru
				if reg.NearestODPID == nil || *reg.NearestODPID != newODP.ID {
					_ = s.repo.IncrementODPUsedPort(ctx, newODP.ID)
				}

				actualDist := float64(req.DropcoreLengthMeters)
				if actualDist <= 0 {
					actualDist = repository.HaversineDistanceMeters(reg.Latitude, reg.Longitude, newODP.Latitude, newODP.Longitude)
				}

				changeNote := fmt.Sprintf("Teknisi menghubungkan ke ODP %s di lapangan (kabel dropcore: %.0fm)", newODP.Code, actualDist)
				_ = s.repo.UpdateRegistrationODP(ctx, reg.ID, newODP.ID, newODP.Code, actualDist, changeNote)

				reg.NearestODPID = &newODP.ID
				reg.NearestODPCode = &newODP.Code
				reg.DistanceToODPMeters = actualDist
			}
		}
	}

	if req.HousePhotoURL != "" && reg.HousePhotoURL == "" {
		reg.HousePhotoURL = req.HousePhotoURL
		_ = s.repo.UpdateRegistrationHousePhoto(ctx, reg.RegistrationNo, req.HousePhotoURL)
	}

	// Integrasi & Promosi ke GOGIGABILL Core
	_ = s.EnsureGigabillCustomerAndSubscription(ctx, reg)

	_ = s.repo.UpdateRegistrationStatus(ctx, reg.ID, "ACTIVE", reg.GigabillCustomerID, reg.GigabillSubscriptionID)
	reg.Status = "ACTIVE"

	return reg, nil
}

// EnsureGigabillCustomerAndSubscription ensures that a customer account, subscription, and initial invoice
// are created in GOGIGABILL Core immediately upon registration or login.
func (s *OnboardingService) EnsureGigabillCustomerAndSubscription(ctx context.Context, reg *domain.Registration) error {
	if reg == nil || s.billingClient == nil {
		return nil
	}

	// Sudah lengkap sinkronisasinya
	if reg.GigabillCustomerID != nil && *reg.GigabillCustomerID != "" &&
		reg.GigabillSubscriptionID != nil && *reg.GigabillSubscriptionID != "" {
		return nil
	}

	branchPrefix := ResolveBranchPrefix(reg.BranchCode)
	bCode := "PYK"
	if reg.BranchCode != nil && *reg.BranchCode != "" {
		bCode = *reg.BranchCode
	}

	city := "Payakumbuh"
	province := "Sumatera Barat"
	if bCode == "PAPUA" {
		city = "Jayapura"
		province = "Papua"
	} else {
		addrUpper := strings.ToUpper(reg.Address)
		if strings.Contains(addrUpper, "HARAU") || strings.Contains(addrUpper, "LIMA PULUH") || strings.Contains(addrUpper, "50 KOTA") {
			city = "Lima Puluh Kota"
		}
	}

	// 1. Buat Customer di GOGIGABILL jika belum ada
	if reg.GigabillCustomerID == nil || *reg.GigabillCustomerID == "" {
		if existing, _ := s.billingClient.FindCustomerByPhone(ctx, reg.Phone); existing != nil && existing.ID != "" {
			reg.GigabillCustomerID = &existing.ID
		} else {
			var emailPtr *string
			if strings.TrimSpace(reg.Email) != "" {
				trimmed := strings.TrimSpace(reg.Email)
				emailPtr = &trimmed
			}

			branchTag := fmt.Sprintf("[CABANG: %s]", bCode)
			notes := branchTag
			if reg.CustomNotes != "" {
				notes = fmt.Sprintf("%s %s", branchTag, reg.CustomNotes)
			}

			custReq := billingclient.CreateCustomerRequest{
				PartnerID: reg.PartnerID,
				FullName:  reg.FullName,
				Email:     emailPtr,
				Phone:     reg.Phone,
				Notes:     &notes,
				Street:    reg.Address,
				City:      city,
				Province:  &province,
			}

			createdCust, err := s.billingClient.CreateCustomer(ctx, custReq)
			if err != nil {
				return fmt.Errorf("failed to create gigabill customer: %w", err)
			}
			reg.GigabillCustomerID = &createdCust.ID
		}
	}

	// 2. Buat Subscription di GOGIGABILL jika belum ada (otomatis menerbitkan Faktur Tagihan Awal)
	if reg.GigabillSubscriptionID == nil || *reg.GigabillSubscriptionID == "" {
		pppoeUser := reg.PPPoEUsername
		if pppoeUser == "" {
			clusterCode := "001"
			if reg.NearestODPCode != nil {
				clusterCode = ResolveClusterCode("", *reg.NearestODPCode)
			}
			pppoeUser = fmt.Sprintf("%s%s00001@gogiga.net.id", branchPrefix, clusterCode)
		}
		pppoePass := reg.PPPoEPassword
		if pppoePass == "" {
			cleanPhone := regexp.MustCompile(`\D`).ReplaceAllString(reg.Phone, "")
			pin := "8821"
			if len(cleanPhone) >= 4 {
				pin = cleanPhone[len(cleanPhone)-4:]
			}
			pppoePass = fmt.Sprintf("net%s", pin)
		}

		planUUID := resolvePlanUUID(reg.SelectedPlanID)
		accessType := "PPPOE"

		subReq := billingclient.CreateSubscriptionRequest{
			CustomerID:        *reg.GigabillCustomerID,
			PlanID:            planUUID,
			InitialAccessType: &accessType,
			InitialUsername:   &pppoeUser,
			InitialPassword:   &pppoePass,
		}

		createdSub, subErr := s.billingClient.CreateSubscription(ctx, subReq)
		if subErr != nil && strings.Contains(subErr.Error(), "duplicate key") {
			clusterCode := "001"
			if reg.NearestODPCode != nil {
				clusterCode = ResolveClusterCode("", *reg.NearestODPCode)
			}
			newSeq := (time.Now().Unix() % 90000) + 10000
			altUser := fmt.Sprintf("%s%s%05d@gogiga.net.id", branchPrefix, clusterCode, newSeq)
			subReq.InitialUsername = &altUser
			createdSub, subErr = s.billingClient.CreateSubscription(ctx, subReq)
			if subErr == nil && createdSub != nil {
				reg.PPPoEUsername = altUser
			}
		}
		if subErr != nil {
			return fmt.Errorf("failed to create gigabill subscription: %w", subErr)
		}
		reg.GigabillSubscriptionID = &createdSub.ID
	}

	// Simpan gigabill_customer_id & gigabill_subscription_id ke database onboarding.db
	_ = s.repo.UpdateRegistrationStatus(ctx, reg.ID, reg.Status, reg.GigabillCustomerID, reg.GigabillSubscriptionID)
	return nil
}

func (s *OnboardingService) enrichRegistrationProvider(ctx context.Context, reg *domain.Registration) {
	if reg == nil {
		return
	}
	reg.HasFTTXIntegration = false
	if reg.NearestODPCode != nil && *reg.NearestODPCode != "" {
		if odp, err := s.repo.GetODPByCode(ctx, *reg.NearestODPCode); err == nil && odp != nil {
			pID := strings.ToUpper(strings.TrimSpace(odp.ProviderID))
			pName := strings.ToUpper(strings.TrimSpace(odp.ProviderName))
			if pID == "GNET2" || strings.Contains(pName, "(2)") || strings.Contains(pID, "GNET-2") {
				reg.HasFTTXIntegration = true
			}
		}
	}
}

func (s *OnboardingService) GetRegistrationByNo(ctx context.Context, regNo string) (*domain.Registration, error) {
	reg, err := s.repo.GetRegistrationByNo(ctx, regNo)
	if err == nil && reg != nil {
		if reg.GigabillCustomerID == nil || reg.GigabillSubscriptionID == nil {
			_ = s.EnsureGigabillCustomerAndSubscription(ctx, reg)
		}
		s.enrichRegistrationProvider(ctx, reg)
	}
	return reg, err
}

func (s *OnboardingService) CustomerLogin(ctx context.Context, req domain.CustomerLoginRequest) (*domain.CustomerPortalData, error) {
	authMethod := strings.ToLower(strings.TrimSpace(req.AuthMethod))

	var primaryReg *domain.Registration
	var err error

	if authMethod == "session" {
		sessToken := strings.TrimSpace(req.SessionToken)
		if sessToken == "" {
			return nil, fmt.Errorf("token sesi tidak valid")
		}
		sess, errSess := s.repo.GetAuthSession(ctx, sessToken)
		if errSess != nil || sess == nil || sess.Role != "customer" {
			return nil, fmt.Errorf("sesi Anda telah kedaluwarsa, silakan login kembali")
		}
		primaryReg, err = s.repo.GetRegistrationByID(ctx, sess.UserID)
		if err != nil || primaryReg == nil {
			primaryReg, err = s.repo.GetRegistrationByNo(ctx, sess.Username)
		}
		if err != nil || primaryReg == nil {
			return nil, fmt.Errorf("data akun pelanggan tidak ditemukan")
		}
	} else {
		cleanIdentifier := strings.TrimSpace(req.Email)
		if cleanIdentifier == "" {
			cleanIdentifier = strings.TrimSpace(req.Identifier)
		}
		if cleanIdentifier == "" {
			return nil, fmt.Errorf("nomor WhatsApp, email, atau nomor registrasi wajib diisi")
		}

		primaryReg, err = s.repo.GetRegistrationByNo(ctx, cleanIdentifier)
		if (err != nil || primaryReg == nil) && s.billingClient != nil {
			// Auto-sync Fallback: Check if identifier was updated in GOGIGABILL Billing System
			if cust, _ := s.billingClient.FindCustomerByQuery(ctx, cleanIdentifier); cust != nil && cust.ID != "" {
				if reg, _ := s.repo.GetRegistrationByGigabillCustomerID(ctx, cust.ID); reg != nil {
					if cust.Email != "" && cust.Email != reg.Email {
						reg.Email = cust.Email
					}
					if cust.Phone != "" && cust.Phone != reg.Phone {
						reg.Phone = cust.Phone
					}
					_ = s.repo.UpdateRegistrationContact(ctx, reg.ID, reg.Email, reg.Phone)
					primaryReg = reg
					err = nil
				}
			}
		}
		if err != nil || primaryReg == nil {
			return nil, fmt.Errorf("akun pelanggan dengan nomor/identitas '%s' tidak ditemukan", cleanIdentifier)
		}

		// ── VERIFIKASI KEAMANAN (PASSWORD ATAU OTP) ──
		if authMethod == "otp" {
			cleanOTP := strings.TrimSpace(req.OTP)
			if cleanOTP == "" {
				return nil, fmt.Errorf("kode OTP wajib diisi")
			}
			valid, err := s.repo.VerifyCustomerOTP(ctx, primaryReg.Phone, cleanOTP)
			if err != nil || !valid {
				return nil, fmt.Errorf("kode OTP salah atau telah kedaluwarsa")
			}
		} else {
			enteredPass := strings.TrimSpace(req.Password)
			if enteredPass == "" {
				return nil, fmt.Errorf("kata sandi akun wajib diisi")
			}

			storedHash, _ := s.repo.GetCustomerPasswordHash(ctx, primaryReg.ID)
			if storedHash != "" {
				if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(enteredPass)); err != nil {
					return nil, fmt.Errorf("kata sandi yang Anda masukkan salah")
				}
			} else {
				cleanPhone := regexp.MustCompile(`\D`).ReplaceAllString(primaryReg.Phone, "")
				var defaultPass string
				if len(cleanPhone) >= 6 {
					defaultPass = cleanPhone[len(cleanPhone)-6:]
				} else {
					defaultPass = cleanPhone
				}

				isMatch := (defaultPass != "" && enteredPass == defaultPass) || enteredPass == "gogiga123"
				if !isMatch {
					return nil, fmt.Errorf("kata sandi salah. Gunakan 6 digit terakhir nomor WhatsApp Anda sebagai kata sandi awal")
				}

				if hashed, err := bcrypt.GenerateFromPassword([]byte(enteredPass), bcrypt.DefaultCost); err == nil {
					_ = s.repo.UpdateCustomerPassword(ctx, primaryReg.Phone, string(hashed))
				}
			}
		}
	}

	// Keep email and phone in sync from billing system
	if primaryReg.GigabillCustomerID != nil && s.billingClient != nil {
		if cust, _ := s.billingClient.FindCustomerByQuery(ctx, *primaryReg.GigabillCustomerID); cust != nil && cust.ID != "" {
			var needUpdate bool
			if cust.Email != "" && cust.Email != primaryReg.Email {
				primaryReg.Email = cust.Email
				needUpdate = true
			}
			if cust.Phone != "" && cust.Phone != primaryReg.Phone {
				primaryReg.Phone = cust.Phone
				needUpdate = true
			}
			if needUpdate {
				_ = s.repo.UpdateRegistrationContact(ctx, primaryReg.ID, primaryReg.Email, primaryReg.Phone)
			}
		}
	}

	// Fetch all locations registered by this customer (via email, phone, or reg_no)
	var allRegs []domain.Registration
	if primaryReg.Email != "" {
		allRegs, _ = s.repo.ListRegistrationsByCustomer(ctx, primaryReg.Email)
	} else if primaryReg.Phone != "" {
		allRegs, _ = s.repo.ListRegistrationsByCustomer(ctx, primaryReg.Phone)
	}
	if len(allRegs) == 0 {
		allRegs = []domain.Registration{*primaryReg}
	}

	var rawLocations []domain.CustomerSiteAccount
	for i := range allRegs {
		regCopy := allRegs[i]
		if regCopy.GigabillCustomerID == nil || regCopy.GigabillSubscriptionID == nil {
			_ = s.EnsureGigabillCustomerAndSubscription(ctx, &regCopy)
			allRegs[i] = regCopy
		}
		s.enrichRegistrationProvider(ctx, &regCopy)
		site := domain.CustomerSiteAccount{
			Registration: &regCopy,
		}
		if wo, err := s.repo.GetWorkOrderByRegistrationID(ctx, regCopy.ID); err == nil && wo != nil {
			wo.Registration = &regCopy
			site.WorkOrder = wo
		}
		if regCopy.ID == primaryReg.ID {
			rawLocations = append([]domain.CustomerSiteAccount{site}, rawLocations...)
		} else {
			rawLocations = append(rawLocations, site)
		}
	}

	var deduped []domain.CustomerSiteAccount
	seen := make(map[string]bool)
	for _, loc := range rawLocations {
		if !seen[loc.Registration.ID] {
			seen[loc.Registration.ID] = true
			deduped = append(deduped, loc)
		}
	}

	var activeSessionToken string
	if authMethod == "session" {
		activeSessionToken = req.SessionToken
	} else {
		newSessToken := uuid.New().String()
		sess := &domain.AuthSession{
			Token:     newSessToken,
			UserID:    primaryReg.ID,
			Username:  primaryReg.Phone,
			Role:      "customer",
			ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
			CreatedAt: time.Now(),
		}
		_ = s.repo.CreateAuthSession(ctx, sess)
		activeSessionToken = newSessToken
	}

	data := &domain.CustomerPortalData{
		SessionToken: activeSessionToken,
		Registration: deduped[0].Registration,
		WorkOrder:    deduped[0].WorkOrder,
		Locations:    deduped,
	}

	return data, nil
}

func (s *OnboardingService) ChangeCustomerPassword(ctx context.Context, req domain.CustomerChangePasswordRequest) error {
	identifier := strings.TrimSpace(req.Identifier)
	oldPass := strings.TrimSpace(req.OldPassword)
	newPass := strings.TrimSpace(req.NewPassword)

	if identifier == "" {
		return fmt.Errorf("nomor WhatsApp atau email akun wajib diisi")
	}
	if len(newPass) < 6 {
		return fmt.Errorf("kata sandi baru minimal harus 6 karakter")
	}

	primaryReg, err := s.repo.GetRegistrationByNo(ctx, identifier)
	if err != nil || primaryReg == nil {
		return fmt.Errorf("akun pelanggan dengan nomor/identitas '%s' tidak ditemukan", identifier)
	}

	storedHash, _ := s.repo.GetCustomerPasswordHash(ctx, primaryReg.ID)
	if storedHash != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(oldPass)); err != nil {
			return fmt.Errorf("kata sandi lama yang Anda masukkan salah")
		}
	} else {
		cleanPhone := regexp.MustCompile(`\D`).ReplaceAllString(primaryReg.Phone, "")
		var defaultPass string
		if len(cleanPhone) >= 6 {
			defaultPass = cleanPhone[len(cleanPhone)-6:]
		} else {
			defaultPass = cleanPhone
		}
		isMatch := (defaultPass != "" && oldPass == defaultPass) || oldPass == "gogiga123"
		if !isMatch {
			return fmt.Errorf("kata sandi lama salah. (Gunakan 6 digit terakhir nomor WhatsApp Anda jika belum pernah mengganti sandi)")
		}
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("gagal mengenkripsi kata sandi baru: %w", err)
	}

	target := primaryReg.Phone
	if target == "" {
		target = primaryReg.Email
	}
	if target == "" {
		target = primaryReg.ID
	}

	return s.repo.UpdateCustomerPassword(ctx, target, string(newHash))
}

func (s *OnboardingService) RequestCustomerOTP(ctx context.Context, phone string) (string, error) {
	cleanPhone := strings.TrimSpace(phone)
	if cleanPhone == "" {
		return "", fmt.Errorf("nomor WhatsApp wajib diisi")
	}

	primaryReg, err := s.repo.GetRegistrationByNo(ctx, cleanPhone)
	if err != nil || primaryReg == nil {
		return "", fmt.Errorf("nomor WhatsApp '%s' tidak terdaftar sebagai pelanggan GOGIGANET", cleanPhone)
	}

	// Buat kode OTP 4 digit acak
	otpCode := fmt.Sprintf("%04d", rand.Intn(10000))
	expiresAt := time.Now().Add(5 * time.Minute)

	if err := s.repo.StoreCustomerOTP(ctx, primaryReg.Phone, otpCode, expiresAt); err != nil {
		return "", fmt.Errorf("gagal menyimpan sesi OTP: %w", err)
	}

	return otpCode, nil
}

func (s *OnboardingService) UpdateRegistrationKTP(ctx context.Context, regNo string, ktpURL string) error {
	reg, err := s.repo.GetRegistrationByNo(ctx, regNo)
	if err != nil || reg == nil {
		return fmt.Errorf("registrasi tidak ditemukan")
	}
	if reg.Status == "ACTIVE" {
		return fmt.Errorf("DOKUMEN_TERKUNCI: Identitas e-KTP telah terverifikasi resmi dan terkunci karena layanan sudah aktif. Hubungi Admin NOC untuk permohonan perubahan data")
	}

	// SOP: KTP melekat pada orang/pelanggan. Jika pelanggan sudah memiliki layanan AKTIF di lokasi lain, KTP terkunci terpusat!
	var customerRegs []domain.Registration
	if reg.Email != "" {
		customerRegs, _ = s.repo.ListRegistrationsByCustomer(ctx, reg.Email)
	} else if reg.Phone != "" {
		customerRegs, _ = s.repo.ListRegistrationsByCustomer(ctx, reg.Phone)
	}
	for _, cr := range customerRegs {
		if cr.Status == "ACTIVE" {
			return fmt.Errorf("DOKUMEN_TERKUNCI: Identitas e-KTP Anda telah terverifikasi resmi dan terkunci terpusat pada layanan aktif (%s). Hubungi Admin NOC untuk permohonan perubahan data", cr.RegistrationNo)
		}
	}

	return s.repo.UpdateRegistrationKTP(ctx, regNo, ktpURL)
}

func (s *OnboardingService) UpdateRegistrationSitePIC(ctx context.Context, regNo string, picName string, picPhone string) error {
	return s.repo.UpdateRegistrationSitePIC(ctx, regNo, strings.TrimSpace(picName), strings.TrimSpace(picPhone))
}

func (s *OnboardingService) SignRegistrationContract(ctx context.Context, regNo string, signatureURL string) (*domain.Registration, error) {
	reg, err := s.repo.GetRegistrationByNo(ctx, regNo)
	if err != nil || reg == nil {
		return nil, fmt.Errorf("nomor registrasi %s tidak ditemukan", regNo)
	}
	if reg.Status == "CANCELLED" || reg.Status == "CANCELLED_NO_COVERAGE" || reg.Status == "REJECTED" {
		return nil, fmt.Errorf("permohonan registrasi %s telah dibatalkan, kontrak tidak dapat ditandatangani", regNo)
	}
	if err := s.repo.UpdateRegistrationContract(ctx, regNo, signatureURL); err != nil {
		return nil, err
	}
	return s.repo.GetRegistrationByNo(ctx, regNo)
}

func (s *OnboardingService) ListRegistrations(ctx context.Context, partnerID *string, status *string, branchCode *string) ([]domain.Registration, error) {
	regs, err := s.repo.ListRegistrations(ctx, partnerID, status, branchCode)
	if err != nil {
		return nil, err
	}
	allWOs, _ := s.repo.ListWorkOrders(ctx, nil)
	regWOMap := make(map[string]domain.WorkOrder)
	for _, w := range allWOs {
		if existing, ok := regWOMap[w.RegistrationID]; !ok || existing.Status == "COMPLETED" {
			regWOMap[w.RegistrationID] = w
		}
	}
	for i := range regs {
		wo, ok := regWOMap[regs[i].ID]
		if !ok && (regs[i].Status == "SUBMITTED" || regs[i].Status == "PENDING_SURVEY_OVERDISTANCE" || regs[i].Status == "INSTALLATION_SCHEDULED" || regs[i].Status == "SURVEY_SCHEDULED") {
			woType := "INSTALLATION"
			if regs[i].Status == "PENDING_SURVEY_OVERDISTANCE" {
				woType = "SURVEY"
			}
			newWO, err := s.CreateWorkOrder(ctx, regs[i].ID, woType, "Tim Teknisi Payakumbuh", time.Now().Add(24*time.Hour), "Penugasan Pasang Baru Otomatis Sistem")
			if err == nil && newWO != nil {
				wo = *newWO
				regWOMap[regs[i].ID] = wo
				ok = true
			}
		}
		if ok {
			regs[i].WorkOrderID = &wo.ID
			regs[i].WorkOrderNo = &wo.OrderNo
			regs[i].WorkOrderStatus = &wo.Status
			regs[i].TechnicianName = &wo.TechnicianName
		}
	}
	return regs, nil
}

func (s *OnboardingService) ListRegistrationsForPartner(ctx context.Context, partnerID string) ([]domain.Registration, error) {
	regs, err := s.repo.ListRegistrationsForPartner(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	allWOs, _ := s.repo.ListWorkOrders(ctx, nil)
	regWOMap := make(map[string]domain.WorkOrder)
	for _, w := range allWOs {
		regWOMap[w.RegistrationID] = w
	}
	for i := range regs {
		if wo, ok := regWOMap[regs[i].ID]; ok {
			regs[i].WorkOrderID = &wo.ID
			regs[i].WorkOrderNo = &wo.OrderNo
			regs[i].WorkOrderStatus = &wo.Status
			regs[i].TechnicianName = &wo.TechnicianName
		}
	}
	return regs, nil
}

func (s *OnboardingService) ClaimRegistration(ctx context.Context, regNo string, partnerID string) (*domain.Registration, error) {
	return s.repo.ClaimRegistration(ctx, regNo, partnerID)
}

func (s *OnboardingService) UpdateRegistrationPricing(ctx context.Context, idOrNo string, req domain.UpdateRegistrationPricingRequest) (*domain.Registration, error) {
	updated, err := s.repo.UpdateRegistrationPricing(ctx, idOrNo, req)
	if err != nil {
		return nil, err
	}
	// Terbitkan invoice hanya jika status sudah sah INSTALLATION_SCHEDULED (bukan WAITING_APPROVAL_NOC)
	if updated != nil && s.billingClient != nil && updated.Status == "INSTALLATION_SCHEDULED" {
		_ = s.EnsureGigabillCustomerAndSubscription(ctx, updated)
	}
	return updated, nil
}

func (s *OnboardingService) NOCApprovalRegistration(ctx context.Context, idOrNo string, req domain.NOCApprovalActionRequest) (*domain.Registration, error) {
	approver := req.ApproverName
	if approver == "" {
		approver = "NOC Operations"
	}
	updated, err := s.repo.NOCApprovalRegistration(ctx, idOrNo, req.Action, approver, req.Notes)
	if err != nil {
		return nil, err
	}
	// Jika disetujui (APPROVE), otomatis sinkronisasi ke GOGIGABILL untuk terbitkan Subscription & Faktur Tagihan Awal
	if updated != nil && strings.ToUpper(strings.TrimSpace(req.Action)) == "APPROVE" && s.billingClient != nil {
		_ = s.EnsureGigabillCustomerAndSubscription(ctx, updated)
	}
	return updated, nil
}

func (s *OnboardingService) UpgradeBandwidth(ctx context.Context, idOrNo string, req domain.UpgradeBandwidthRequest) (*domain.Registration, error) {
	return s.repo.UpgradeBandwidth(ctx, idOrNo, req)
}


func (s *OnboardingService) ListODPs(ctx context.Context, branchCode string) ([]domain.ODPNode, error) {
	return s.repo.ListODPs(ctx, branchCode)
}

func (s *OnboardingService) CreateODP(ctx context.Context, odp *domain.ODPNode) error {
	return s.repo.CreateODP(ctx, odp)
}

func (s *OnboardingService) DeleteODP(ctx context.Context, idOrCode string) error {
	return s.repo.DeleteODP(ctx, idOrCode)
}

func (s *OnboardingService) DeleteRegistration(ctx context.Context, idOrRegNo string) error {
	return s.repo.DeleteRegistration(ctx, idOrRegNo)
}

func (s *OnboardingService) CreatePartner(ctx context.Context, p *domain.Partner) error {
	return s.repo.CreatePartner(ctx, p)
}

func (s *OnboardingService) GetPartnerByCode(ctx context.Context, code string) (*domain.Partner, error) {
	return s.repo.GetPartnerByCode(ctx, code)
}

func (s *OnboardingService) CheckReferralCode(ctx context.Context, rawCode string) (*domain.ReferralCheckResult, error) {
	code := strings.TrimSpace(rawCode)
	if code == "" {
		return &domain.ReferralCheckResult{Valid: false}, nil
	}

	// 1. Cek Partner Resmi (Sales/Mitra/Teknisi)
	if p, err := s.repo.GetPartnerByCode(ctx, code); err == nil && p != nil && p.IsActive {
		return &domain.ReferralCheckResult{
			Valid:          true,
			Type:           "SALES_PARTNER",
			PartnerID:      p.ID,
			Code:           p.Code,
			Name:           p.Name,
			CommissionRate: p.CommissionRate,
		}, nil
	}

	// 2. Cek Program Referral Pelanggan Aktif (Berdasarkan No Registrasi atau No HP)
	if cust, err := s.repo.GetActiveCustomerByReferralCode(ctx, code); err == nil && cust != nil {
		return &domain.ReferralCheckResult{
			Valid: true,
			Type:  "CUSTOMER_REFERRAL",
			Code:  cust.RegistrationNo,
			Name:  cust.FullName,
		}, nil
	}

	return &domain.ReferralCheckResult{Valid: false}, nil
}

func (s *OnboardingService) GetPartnerByID(ctx context.Context, id string) (*domain.Partner, error) {
	return s.repo.GetPartnerByID(ctx, id)
}

func (s *OnboardingService) UpdatePartner(ctx context.Context, p *domain.Partner) error {
	return s.repo.UpdatePartner(ctx, p)
}

func (s *OnboardingService) ListPartners(ctx context.Context, branchCode string) ([]domain.Partner, error) {
	return s.repo.ListPartners(ctx, branchCode)
}

func (s *OnboardingService) ListWorkOrders(ctx context.Context, status *string) ([]domain.WorkOrder, error) {
	// Auto-create Work Order for any registration that doesn't have one yet
	regs, _ := s.repo.ListRegistrations(ctx, nil, nil, nil)
	existingWOs, err := s.repo.ListWorkOrders(ctx, nil)
	if err == nil {
		regWOMap := make(map[string]bool)
		for _, w := range existingWOs {
			regWOMap[w.RegistrationID] = true
		}
		for _, r := range regs {
			if !regWOMap[r.ID] && (r.Status == "SUBMITTED" || r.Status == "PENDING_SURVEY_OVERDISTANCE" || r.Status == "INSTALLATION_SCHEDULED" || r.Status == "SURVEY_SCHEDULED") {
				woType := "INSTALLATION"
				if r.Status == "PENDING_SURVEY_OVERDISTANCE" {
					woType = "SURVEY"
				}
				_, _ = s.CreateWorkOrder(ctx, r.ID, woType, "Tim Teknisi Payakumbuh", time.Now().Add(24*time.Hour), "Penugasan Pasang Baru Otomatis Sistem")
			}
		}
	}

	wos, err := s.repo.ListWorkOrders(ctx, status)
	if err != nil {
		return nil, err
	}

	// Hydrate Registration & BAST for each work order
	for i := range wos {
		if reg, err := s.repo.GetRegistrationByID(ctx, wos[i].RegistrationID); err == nil && reg != nil {
			wos[i].Registration = reg
		}
		if wos[i].BAST == nil {
			if fullWO, err := s.repo.GetWorkOrderByID(ctx, wos[i].ID); err == nil && fullWO != nil {
				wos[i].BAST = fullWO.BAST
			}
		}
	}

	return wos, nil
}

func (s *OnboardingService) GetWorkOrderByID(ctx context.Context, id string) (*domain.WorkOrder, error) {
	wo, err := s.repo.GetWorkOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if reg, err := s.repo.GetRegistrationByID(ctx, wo.RegistrationID); err == nil && reg != nil {
		wo.Registration = reg
	}
	return wo, nil
}

func (s *OnboardingService) ListClusters(ctx context.Context, branchCode string) ([]domain.ClusterSummary, error) {
	return s.repo.ListClusters(ctx, branchCode)
}

func (s *OnboardingService) SetClusterStatus(ctx context.Context, clusterArea string, active bool, branchCode string) error {
	return s.repo.SetClusterStatus(ctx, clusterArea, active, branchCode)
}

func (s *OnboardingService) ListStaffUsers(ctx context.Context) ([]domain.StaffUser, error) {
	return s.repo.ListStaffUsers(ctx)
}

func (s *OnboardingService) CreateStaffUser(ctx context.Context, u *domain.StaffUser) error {
	return s.repo.CreateStaffUser(ctx, u)
}

func (s *OnboardingService) UpdateStaffUser(ctx context.Context, u *domain.StaffUser) error {
	return s.repo.UpdateStaffUser(ctx, u)
}

func (s *OnboardingService) UpdateStaffPassword(ctx context.Context, username, newPasswordHash string) error {
	return s.repo.UpdateStaffPassword(ctx, username, newPasswordHash)
}


func (s *OnboardingService) GetExecutiveOverview(ctx context.Context, branchCode string) (*domain.ExecutiveOverview, error) {
	return s.repo.GetExecutiveOverview(ctx, branchCode)
}

// ── JARTAPLOK & KML PROCESSING ────────────────────────────

type kmlRoot struct {
	Document kmlDocument `xml:"Document"`
}

type kmlDocument struct {
	Name    string      `xml:"name"`
	Folders []kmlFolder `xml:"Folder"`
}

type kmlFolder struct {
	Name       string         `xml:"name"`
	Placemarks []kmlPlacemark `xml:"Placemark"`
}

type kmlPlacemark struct {
	Name        string    `xml:"name"`
	Description string    `xml:"description"`
	Point       *kmlPoint `xml:"Point"`
}

type kmlPoint struct {
	Coordinates string `xml:"coordinates"`
}

func (s *OnboardingService) ProcessKMLUpload(ctx context.Context, providerID, providerName string, kmlBytes []byte) (*domain.KMLUploadResult, error) {
	var kml kmlRoot
	if err := xml.Unmarshal(kmlBytes, &kml); err != nil {
		return nil, fmt.Errorf("format file KML tidak valid: %w", err)
	}

	var odps []domain.ODPNode
	counters := make(map[string]int)

	for _, folder := range kml.Document.Folders {
		folderName := strings.TrimSpace(folder.Name)
		if strings.Contains(strings.ToLower(folderName), "wireless") {
			continue
		}

		var prefix string
		switch {
		case strings.Contains(folderName, "Biaro"):
			prefix = "ODP-BIO"
		case strings.Contains(folderName, "Payakumbuh"):
			prefix = "ODP-PYK"
		case strings.Contains(folderName, "Geringging"):
			prefix = "ODP-SGG"
		default:
			cleanPfx := strings.ReplaceAll(strings.ToUpper(providerID), "-", "")
			if len(cleanPfx) > 4 {
				cleanPfx = cleanPfx[:4]
			}
			prefix = "ODP-" + cleanPfx
		}

		for _, pm := range folder.Placemarks {
			if pm.Point == nil {
				continue
			}
			rawCoords := strings.TrimSpace(pm.Point.Coordinates)
			parts := strings.Split(rawCoords, ",")
			if len(parts) < 2 {
				continue
			}
			lng, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			lat, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err1 != nil || err2 != nil {
				continue
			}

			counters[prefix]++
			idx := counters[prefix]
			odpCode := fmt.Sprintf("%s-%04d", prefix, idx)
			pmName := strings.TrimSpace(pm.Name)
			if strings.HasPrefix(strings.ToUpper(pmName), "ODP-") {
				token := strings.Split(pmName, " ")[0]
				odpCode = strings.ToUpper(strings.TrimSpace(token))
			}

			odpName := pmName
			if odpName == "" || odpName == "Untitled Placemark" {
				odpName = fmt.Sprintf("Tiang %s #%d", folderName, idx)
			}

			odps = append(odps, domain.ODPNode{
				Code:        odpCode,
				Name:        odpName,
				Latitude:    lat,
				Longitude:   lng,
				TotalPorts:  8,
				ClusterArea: folderName,
				ProviderID:  providerID,
				ProviderName: providerName,
				Status:      "AVAILABLE",
			})
		}
	}

	if len(odps) == 0 {
		return nil, fmt.Errorf("tidak ditemukan titik koordinat (Placemark) tiang ODP yang valid dalam file KML ini")
	}

	return s.repo.BatchUpsertODPs(ctx, providerID, providerName, odps)
}

func (s *OnboardingService) ListODPsByProvider(ctx context.Context, providerID string) ([]domain.ODPNode, error) {
	return s.repo.ListODPsByProvider(ctx, providerID)
}

func (s *OnboardingService) ListJartaplokPartners(ctx context.Context, branchCode string) ([]domain.JartaplokPartner, error) {
	return s.repo.ListJartaplokPartners(ctx, branchCode)
}

func (s *OnboardingService) GetJartaplokPartnerByAPIKey(ctx context.Context, apiKey string) (*domain.JartaplokPartner, error) {
	return s.repo.GetJartaplokPartnerByAPIKey(ctx, apiKey)
}

func (s *OnboardingService) CreateJartaplokPartner(ctx context.Context, p *domain.JartaplokPartner) error {
	return s.repo.CreateJartaplokPartner(ctx, p)
}

func (s *OnboardingService) UpdateJartaplokPartner(ctx context.Context, p *domain.JartaplokPartner) error {
	return s.repo.UpdateJartaplokPartner(ctx, p)
}

func (s *OnboardingService) SuspendRegistration(ctx context.Context, idOrNo string, reason string) (*domain.Registration, error) {
	// Ambil data registrasi untuk memeriksa kebijakan mitra wholesale
	reg, err := s.repo.GetRegistrationByNo(ctx, idOrNo)
	if err != nil {
		reg, err = s.repo.GetRegistrationByID(ctx, idOrNo)
	}
	if err != nil || reg == nil {
		return nil, fmt.Errorf("registrasi %s tidak ditemukan: %w", idOrNo, err)
	}

	// Blokir jika rekanan wholesale menerapkan kebijakan DISALLOWED
	if reg.PartnerSuspensionPolicy == domain.SuspensionPolicyDisallowed {
		partnerName := reg.PartnerName
		if partnerName == "" {
			partnerName = "Penyelenggara Wholesale Jartaplok/Bitstream"
		}
		return nil, fmt.Errorf("jeda layanan ditolak: sirkuit berada di jaringan %s yang menerapkan komitmen kontrak penuh tanpa jeda layanan", partnerName)
	}

	if err := s.repo.SuspendRegistration(ctx, idOrNo, reason); err != nil {
		return nil, err
	}
	updatedReg, err := s.repo.GetRegistrationByNo(ctx, idOrNo)
	if err != nil {
		updatedReg, err = s.repo.GetRegistrationByID(ctx, idOrNo)
	}

	// Sinkronisasi ke GOGIGABILL Core -> FreeRADIUS Isolir Group & RFC 3576 CoA Disconnect ke MikroTik
	if updatedReg != nil && s.billingClient != nil && updatedReg.GigabillSubscriptionID != nil && *updatedReg.GigabillSubscriptionID != "" {
		_ = s.billingClient.SuspendSubscription(ctx, *updatedReg.GigabillSubscriptionID)
	}
	return updatedReg, err
}

func (s *OnboardingService) ResumeRegistration(ctx context.Context, idOrNo string) (*domain.Registration, error) {
	if err := s.repo.ResumeRegistration(ctx, idOrNo); err != nil {
		return nil, err
	}
	reg, err := s.repo.GetRegistrationByNo(ctx, idOrNo)
	if err != nil {
		reg, err = s.repo.GetRegistrationByID(ctx, idOrNo)
	}

	// Sinkronisasi ke GOGIGABILL Core -> FreeRADIUS Restore Plan Group & RFC 3576 CoA Disconnect ke MikroTik
	if reg != nil && s.billingClient != nil && reg.GigabillSubscriptionID != nil && *reg.GigabillSubscriptionID != "" {
		_ = s.billingClient.ReactivateSubscription(ctx, *reg.GigabillSubscriptionID)
	}
	return reg, err
}

// GetLiveRadiusSessions mengembalikan map username PPPoE -> info sesi aktif real-time dari FreeRADIUS
func (s *OnboardingService) GetLiveRadiusSessions(ctx context.Context) (map[string]domain.LiveSessionInfo, error) {
	if s.billingClient == nil {
		return make(map[string]domain.LiveSessionInfo), nil
	}
	sessions, err := s.billingClient.ListActiveRadiusSessions(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]domain.LiveSessionInfo, len(sessions))
	for _, sess := range sessions {
		ip := ""
		if sess.FramedIPAddress != nil {
			ip = *sess.FramedIPAddress
			ip = strings.TrimSuffix(ip, "/32")
		}
		port := ""
		if sess.NasPortID != nil {
			port = *sess.NasPortID
		}
		nasIP := strings.TrimSuffix(sess.NasIPAddress, "/32")
		result[sess.Username] = domain.LiveSessionInfo{
			RadAcctID:        sess.RadAcctID,
			AcctSessionID:    sess.AcctSessionID,
			Username:         sess.Username,
			GroupName:        sess.GroupName,
			NasIPAddress:     nasIP,
			NasPortID:        port,
			AcctStartTime:    sess.AcctStartTime,
			AcctSessionTime:  sess.AcctSessionTime,
			CallingStationID: sess.CallingStationID,
			FramedIPAddress:  ip,
			IsOnline:         sess.IsActive,
		}
	}
	return result, nil
}

// KickPPPoESession mengirim sinyal CoA Disconnect ke MikroTik untuk me-reset sesi koneksi pelanggan
func (s *OnboardingService) KickPPPoESession(ctx context.Context, username string) error {
	cleanUser := strings.TrimSpace(username)
	if cleanUser == "" {
		return fmt.Errorf("pppoe username cannot be empty")
	}

	targetFramedIP := ""
	nasIP := "103.179.65.30"

	if s.billingClient != nil {
		sessions, err := s.billingClient.ListActiveRadiusSessions(ctx)
		if err == nil {
			for _, sess := range sessions {
				if strings.EqualFold(sess.Username, cleanUser) {
					if sess.FramedIPAddress != nil {
						targetFramedIP = strings.TrimSuffix(*sess.FramedIPAddress, "/32")
					}
					nasIP = strings.TrimSuffix(sess.NasIPAddress, "/32")
					_ = s.billingClient.DisconnectRadiusSession(ctx, nasIP, sess.Username, targetFramedIP, sess.AcctSessionID)
					break
				}
			}
		}
	}

	// Trigger real RFC 3576 CoA Disconnect packet via radclient in FreeRADIUS container
	var radInput bytes.Buffer
	radInput.WriteString(fmt.Sprintf("User-Name=%s\n", cleanUser))
	if targetFramedIP != "" {
		radInput.WriteString(fmt.Sprintf("Framed-IP-Address=%s\n", targetFramedIP))
	}
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", "gogigabill-freeradius", "radclient", "-x", nasIP+":3799", "disconnect", "ITzY02nmpkgxiZfR")
	cmd.Stdin = &radInput
	_ = cmd.Run()

	return nil
}

func (s *OnboardingService) AttachPartnerODP(ctx context.Context, idOrNo string, req domain.AttachPartnerODPRequest) (*domain.Registration, error) {
	if strings.TrimSpace(req.ODPCode) == "" {
		return nil, fmt.Errorf("kode ODP wajib diisi")
	}
	if req.PartnerCode == "" {
		req.PartnerCode = "TELKO-PYK"
	}
	return s.repo.AttachRegistrationToPartnerODP(ctx, idOrNo, req)
}

func (s *OnboardingService) UpdateUpstreamPPPoE(ctx context.Context, idOrNo string, req domain.UpdateUpstreamPPPoERequest) (*domain.Registration, error) {
	cleanUser := strings.TrimSpace(req.UpstreamPPPoEUsername)
	cleanPass := strings.TrimSpace(req.UpstreamPPPoEPassword)
	if err := s.repo.UpdateRegistrationUpstreamPPPoE(ctx, idOrNo, cleanUser, cleanPass); err != nil {
		return nil, fmt.Errorf("gagal memperbarui akun dial jartaplok: %w", err)
	}
	if strings.HasPrefix(idOrNo, "REG-") {
		return s.repo.GetRegistrationByNo(ctx, idOrNo)
	}
	return s.repo.GetRegistrationByID(ctx, idOrNo)
}

func (s *OnboardingService) MarkUncovered(ctx context.Context, idOrNo string, req domain.RejectUncoveredRequest) (*domain.Registration, error) {
	action := strings.ToUpper(strings.TrimSpace(req.Action))
	if action != "CANCEL" && action != "WISHLIST" {
		action = "WISHLIST"
	}
	return s.repo.MarkRegistrationUncovered(ctx, idOrNo, action, req.Reason)
}

func (s *OnboardingService) ReassignODP(ctx context.Context, idOrNo string, req domain.ReassignODPRequest) (*domain.Registration, error) {
	cleanCode := strings.ToUpper(strings.TrimSpace(req.ODPCode))
	if cleanCode == "" {
		return nil, fmt.Errorf("kode ODP wajib diisi")
	}

	reg, err := s.repo.GetRegistrationByNo(ctx, idOrNo)
	if err != nil || reg == nil {
		reg, err = s.repo.GetRegistrationByID(ctx, idOrNo)
		if err != nil || reg == nil {
			return nil, fmt.Errorf("registrasi %s tidak ditemukan", idOrNo)
		}
	}

	newODP, err := s.repo.GetODPByCode(ctx, cleanCode)
	if err != nil || newODP == nil {
		return nil, fmt.Errorf("ODP dengan kode '%s' tidak ditemukan di database", cleanCode)
	}

	// Kurangi port ODP lama jika berbeda
	if reg.NearestODPID != nil && *reg.NearestODPID != "" && *reg.NearestODPID != newODP.ID {
		_ = s.repo.DecrementODPUsedPort(ctx, *reg.NearestODPID)
	}

	// Tambah port ODP baru jika belum terhitung
	if reg.NearestODPID == nil || *reg.NearestODPID != newODP.ID {
		_ = s.repo.IncrementODPUsedPort(ctx, newODP.ID)
	}

	dist := 50.0
	if req.DistanceMeters != nil && *req.DistanceMeters > 0 {
		dist = *req.DistanceMeters
	} else {
		dist = repository.HaversineDistanceMeters(reg.Latitude, reg.Longitude, newODP.Latitude, newODP.Longitude)
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "Penyesuaian rute jalur tiang lapangan"
	}
	auditNote := fmt.Sprintf("Titik ODP dialihkan ke %s (%s)", newODP.Code, reason)

	if err := s.repo.UpdateRegistrationODP(ctx, reg.ID, newODP.ID, newODP.Code, dist, auditNote); err != nil {
		return nil, fmt.Errorf("gagal memperbarui titik ODP: %w", err)
	}

	return s.repo.GetRegistrationByID(ctx, reg.ID)
}

// ResolveDefaultPlanPrice returns the default retail price for standard packages in IDR
func ResolveDefaultPlanPrice(planID, planName string) int64 {
	pID := strings.ToLower(planID)
	pName := strings.ToLower(planName)

	switch {
	case strings.Contains(pID, "gold") || strings.Contains(pName, "gold") || strings.Contains(pName, "10m"):
		return 183150
	case strings.Contains(pID, "diamond") || strings.Contains(pName, "diamond") || strings.Contains(pName, "20m"):
		return 259000
	case strings.Contains(pID, "epic") || strings.Contains(pName, "epic") || strings.Contains(pName, "30m"):
		return 299000
	case strings.Contains(pID, "legend") || strings.Contains(pName, "legend") || strings.Contains(pName, "40m"):
		return 329000
	case strings.Contains(pID, "honor") || strings.Contains(pName, "honor") || strings.Contains(pName, "50m"):
		return 389000
	case strings.Contains(pID, "glory") || strings.Contains(pName, "glory") || strings.Contains(pName, "100m"):
		return 599000
	case strings.Contains(pID, "corp-150m") || strings.Contains(pName, "150m"):
		return 950000
	case strings.Contains(pID, "corp-200m") || strings.Contains(pName, "200m"):
		return 1450000
	case strings.Contains(pID, "corp-300m") || strings.Contains(pName, "300m"):
		return 2100000
	default:
		return 0
	}
}

// ResolveClusterCode menentukan 3-digit kode cluster wilayah GoGiga (default 001)
func ResolveClusterCode(clusterArea string, odpCode string) string {
	cUpper := strings.ToUpper(clusterArea)
	oUpper := strings.ToUpper(odpCode)

	// Cek jika sudah ada 3 digit eksplisit di nama cluster (contoh: "Cluster 002")
	re := regexp.MustCompile(`\b(\d{3})\b`)
	if match := re.FindString(cUpper); match != "" {
		return match
	}

	// Pemetaan cluster wilayah GoGiga & Jartaplok
	if strings.Contains(cUpper, "BIARO") || strings.Contains(oUpper, "BIA") || strings.Contains(oUpper, "BIO") {
		return "001"
	}
	if strings.Contains(cUpper, "PAYAKUMBUH") || strings.Contains(oUpper, "PYK") {
		return "002"
	}
	if strings.Contains(cUpper, "GERINGGING") || strings.Contains(cUpper, "GARINGGING") || strings.Contains(oUpper, "SGG") {
		return "003"
	}
	if strings.Contains(cUpper, "HARAU") || strings.Contains(oUpper, "HRU") {
		return "004"
	}

	return "001"
}

// ResolveBranchPrefix menentukan 2-digit kode prefix cabang untuk akun PPPoE (default 14 untuk Payakumbuh)
func ResolveBranchPrefix(branchCode *string) string {
	if branchCode == nil || *branchCode == "" {
		return "14" // Default Payakumbuh
	}
	b := strings.ToUpper(strings.TrimSpace(*branchCode))
	switch b {
	case "PYK", "PAYAKUMBUH", "SUMBAR", "14":
		return "14"
	case "PAPUA", "JAYAPURA", "91":
		return "91"
	case "JAKARTA", "JKT", "21":
		return "21"
	case "BANDUNG", "BDG", "22":
		return "22"
	case "SURABAYA", "SBY", "31":
		return "31"
	default:
		if len(b) == 2 && b[0] >= '0' && b[0] <= '9' && b[1] >= '0' && b[1] <= '9' {
			return b
		}
		return "14"
	}
}

func resolvePlanUUID(idOrSlug string) string {
	clean := strings.ToLower(strings.TrimSpace(idOrSlug))
	switch {
	case strings.Contains(clean, "gold") || strings.Contains(clean, "10m"):
		return "c0020001-0000-0000-0000-000000000001"
	case strings.Contains(clean, "diamond") || strings.Contains(clean, "20m"):
		return "c0020002-0000-0000-0000-000000000002"
	case strings.Contains(clean, "epic") || strings.Contains(clean, "30m"):
		return "c0020003-0000-0000-0000-000000000003"
	case strings.Contains(clean, "legend") || strings.Contains(clean, "40m"):
		return "c0020004-0000-0000-0000-000000000004"
	case strings.Contains(clean, "honor") || strings.Contains(clean, "50m"):
		return "c0020005-0000-0000-0000-000000000005"
	case strings.Contains(clean, "glory") || strings.Contains(clean, "100m"):
		return "c0020006-0000-0000-0000-000000000006"
	case strings.Contains(clean, "custom") || strings.Contains(clean, "enterprise") || strings.Contains(clean, "corporate") || strings.Contains(clean, "150m"):
		return "c0020007-0000-0000-0000-000000000007"
	default:
		if len(idOrSlug) == 36 && strings.Count(idOrSlug, "-") == 4 {
			return idOrSlug
		}
		return "c0020001-0000-0000-0000-000000000001"
	}
}

func (s *OnboardingService) GetCustomerDocuments(ctx context.Context, customerIDOrNo string, email string, phone string) (*domain.CustomerDocumentsData, error) {
	cleanID := strings.TrimSpace(customerIDOrNo)
	var regs []domain.Registration

	if cleanID != "" {
		regs, _ = s.repo.ListRegistrationsByCustomer(ctx, cleanID)
	}
	if len(regs) == 0 && email != "" {
		regs, _ = s.repo.ListRegistrationsByCustomer(ctx, strings.TrimSpace(email))
	}
	if len(regs) == 0 && phone != "" {
		regs, _ = s.repo.ListRegistrationsByCustomer(ctx, strings.TrimSpace(phone))
	}

	result := &domain.CustomerDocumentsData{
		CustomerID: customerIDOrNo,
		Sites:      make([]domain.CustomerDocumentSite, 0, len(regs)),
	}

	for _, reg := range regs {
		if result.FullName == "" {
			result.FullName = reg.FullName
		}
		if result.Phone == "" {
			result.Phone = reg.Phone
		}
		if result.Email == "" {
			result.Email = reg.Email
		}
		if result.IDCardNumber == "" {
			result.IDCardNumber = reg.IDCardNumber
		}

		site := domain.CustomerDocumentSite{
			RegistrationID:       reg.ID,
			RegistrationNo:       reg.RegistrationNo,
			FullName:             reg.FullName,
			IDCardNumber:         reg.IDCardNumber,
			TaxID:                reg.TaxID,
			Phone:                reg.Phone,
			Email:                reg.Email,
			Address:              reg.Address,
			Latitude:             reg.Latitude,
			Longitude:            reg.Longitude,
			SelectedPlanID:       reg.SelectedPlanID,
			SelectedPlanName:     reg.SelectedPlanName,
			NearestODPCode:       reg.NearestODPCode,
			DistanceToODPMeters:  reg.DistanceToODPMeters,
			Status:               reg.Status,
			KTPPhotoURL:          reg.KTPPhotoURL,
			HousePhotoURL:        reg.HousePhotoURL,
			ContractSignatureURL: reg.ContractSignatureURL,
			ContractSignedAt:     reg.ContractSignedAt,
			CreatedAt:            reg.CreatedAt,
		}

		if wo, err := s.repo.GetWorkOrderByRegistrationID(ctx, reg.ID); err == nil && wo != nil {
			site.WorkOrder = wo
		}

		result.Sites = append(result.Sites, site)
	}

	return result, nil
}

func (s *OnboardingService) GetStaffKPISummary(ctx context.Context, branchCode string) (*domain.StaffKPISummary, error) {
	return s.repo.GetStaffKPISummary(ctx, branchCode)
}

func (s *OnboardingService) ListBranches(ctx context.Context) ([]domain.Branch, error) {
	return s.repo.ListBranches(ctx)
}

func (s *OnboardingService) GetBranchByID(ctx context.Context, id string) (*domain.Branch, error) {
	return s.repo.GetBranchByID(ctx, id)
}









