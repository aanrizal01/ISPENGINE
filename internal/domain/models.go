package domain

import "time"

// Branch merepresentasikan kantor cabang internal atau mitra reseller
type Branch struct {
	ID                  string    `json:"id"`
	Code                string    `json:"code"` // Contoh: PYK, PAPUA, GNET-BIARO
	Name                string    `json:"name"`
	Type                string    `json:"type"` // OWNED_BRANCH atau RESELLER
	City                string    `json:"city,omitempty"`
	Province            string    `json:"province,omitempty"`
	RevenueShareType    string    `json:"revenue_share_type,omitempty"`
	SharePercentPartner float64   `json:"share_percent_partner,omitempty"`
	FlatFeePerSub       float64   `json:"flat_fee_per_sub,omitempty"`
	IsActive            bool      `json:"is_active"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ODPNode merepresentasikan titik Optical Distribution Point / FAT di tiang
type ODPNode struct {
	ID            string    `json:"id"`
	Code          string    `json:"code"` // Contoh: ODP-CKR-001
	Name          string    `json:"name"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	TotalPorts    int       `json:"total_ports"`
	UsedPorts     int       `json:"used_ports"`
	Status        string    `json:"status"` // AVAILABLE, FULL, MAINTENANCE
	ClusterArea   string    `json:"cluster_area"`
	ProviderID    string    `json:"provider_id"`
	ProviderName  string    `json:"provider_name"`
	BranchID      *string   `json:"branch_id,omitempty"`
	BranchCode    *string   `json:"branch_code,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (o *ODPNode) AvailablePorts() int {
	return o.TotalPorts - o.UsedPorts
}

// ClusterSummary ringkasan wilayah coverage (Payakumbuh, Bukittinggi/Biaro, dll)
type ClusterSummary struct {
	Name       string `json:"name"`
	TotalODPs  int    `json:"total_odps"`
	ActiveODPs int    `json:"active_odps"`
	IsActive   bool   `json:"is_active"`
}

// Partner merepresentasikan mitra/agen/reseller yang punya portal pendaftaran
type Partner struct {
	ID             string    `json:"id"`
	Code           string    `json:"code"` // Contoh: AGENT-001 / SUBISP-BINTARO
	Name           string    `json:"name"`
	APIKey         string    `json:"api_key"`
	CommissionRate float64   `json:"commission_rate"` // Persentase atau flat
	ContactPhone   string    `json:"contact_phone"`
	BranchID       *string   `json:"branch_id,omitempty"`
	BranchCode     *string   `json:"branch_code,omitempty"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

// Registration merepresentasikan berkas pengajuan pendaftaran pelanggan baru
type Registration struct {
	ID                     string    `json:"id"`
	RegistrationNo         string    `json:"registration_no"` // REG-202609-0001
	PartnerID              *string   `json:"partner_id,omitempty"` // ID mitra jika daftar via portal agen
	PartnerCode            *string   `json:"partner_code,omitempty"`
	FullName               string    `json:"full_name"`
	Email                  string    `json:"email"`
	Phone                  string    `json:"phone"`
	IDCardNumber           string    `json:"id_card_number"` // NIK KTP
	TaxID                  string    `json:"tax_id,omitempty"` // NPWP (Nomor Pokok Wajib Pajak - Opsional)
	Address                string    `json:"address"`
	Latitude               float64   `json:"latitude"`
	Longitude              float64   `json:"longitude"`
	SelectedPlanID         string    `json:"selected_plan_id"`
	SelectedPlanName       string    `json:"selected_plan_name"`
	NearestODPID           *string   `json:"nearest_odp_id,omitempty"`
	NearestODPCode         *string   `json:"nearest_odp_code,omitempty"`
	DistanceToODPMeters    float64   `json:"distance_to_odp_meters"`
	Status                 string     `json:"status"` // SUBMITTED, SURVEY, INSTALLATION, ACTIVE, SUSPENDED, REJECTED
	KTPPhotoURL            string     `json:"ktp_photo_url,omitempty"`
	HousePhotoURL          string     `json:"house_photo_url,omitempty"`
	SitePICName            string     `json:"site_pic_name,omitempty"`
	SitePICPhone           string     `json:"site_pic_phone,omitempty"`
	ContractSignatureURL   string     `json:"contract_signature_url,omitempty"`
	ContractSignedAt       *time.Time `json:"contract_signed_at,omitempty"`
	ActivatedAt            *time.Time `json:"activated_at,omitempty"`
	SuspendedAt            *time.Time `json:"suspended_at,omitempty"`
	SuspensionReason       string     `json:"suspension_reason,omitempty"`
	DispatchNotes          string     `json:"dispatch_notes,omitempty"`
	CustomNotes            string     `json:"custom_notes,omitempty"`
	OTCFee                 int64      `json:"otc_fee"`
	MonthlyPrice           int64      `json:"monthly_price"`
	OTCNotes               string     `json:"otc_notes,omitempty"`
	PPPoEUsername          string     `json:"pppoe_username,omitempty"`
	PPPoEPassword          string     `json:"pppoe_password,omitempty"`
	UpstreamPPPoEUsername  string     `json:"upstream_pppoe_username,omitempty"` // Akun Dial Modem Jartaplok / Upstream (e.g. TIF / Telkom)
	UpstreamPPPoEPassword  string     `json:"upstream_pppoe_password,omitempty"`
	PartnerSuspensionPolicy string    `json:"partner_suspension_policy,omitempty"`
	PartnerName             string    `json:"partner_name,omitempty"`
	GigabillCustomerID     *string    `json:"gigabill_customer_id,omitempty"`
	GigabillSubscriptionID *string    `json:"gigabill_subscription_id,omitempty"`
	WorkOrderID            *string    `json:"work_order_id,omitempty"`
	WorkOrderNo            *string    `json:"work_order_no,omitempty"`
	WorkOrderStatus        *string    `json:"work_order_status,omitempty"`
	TechnicianName         *string    `json:"technician_name,omitempty"`
	BranchID               *string    `json:"branch_id,omitempty"`
	BranchCode             *string    `json:"branch_code,omitempty"`
	HasFTTXIntegration     bool       `json:"has_fttx_integration"`
	ONTSerialNumber        string     `json:"ont_serial_number,omitempty"`
	ONTOpticalPower        float64    `json:"ont_optical_power,omitempty"`
	ONTStatus              string     `json:"ont_status,omitempty"`
	CustomerPasswordHash   string     `json:"-"` // Hash kata sandi portal pelanggan (bcrypt)
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// WorkOrder merepresentasikan Surat Perintah Kerja (Survei / Instalasi)
type WorkOrder struct {
	ID             string        `json:"id"`
	OrderNo        string        `json:"order_no"` // WO-202609-0001
	RegistrationID string        `json:"registration_id"`
	Registration   *Registration `json:"registration,omitempty"`
	Type           string        `json:"type"` // SURVEY, INSTALLATION, DISMANTLE
	TechnicianName string        `json:"technician_name"`
	ScheduledAt    time.Time     `json:"scheduled_at"`
	Status         string        `json:"status"` // ASSIGNED, IN_PROGRESS, COMPLETED, FAILED
	Notes          string        `json:"notes,omitempty"`
	BranchID       *string       `json:"branch_id,omitempty"`
	BranchCode     *string       `json:"branch_code,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	BAST           *BASTReport   `json:"bast,omitempty"`
}

// BASTReport adalah Berita Acara Serah Terima Digital oleh Teknisi
type BASTReport struct {
	ID                   string    `json:"id"`
	WorkOrderID          string    `json:"work_order_id"`
	OpticalPowerDBM      float64   `json:"optical_power_dbm"` // Redaman optik, e.g. -18.5 dBm
	ONTSerialNumber      string    `json:"ont_serial_number"`
	ONTMACAddress        string    `json:"ont_mac_address"`
	DropcoreLengthMeters int       `json:"dropcore_length_meters"`
	CustomerSignatureURL string    `json:"customer_signature_url,omitempty"`
	ProofPhotoURL        string    `json:"proof_photo_url,omitempty"` // Foto ONT / Modem
	HousePhotoURL        string    `json:"house_photo_url,omitempty"` // Foto Rumah Tampak Depan
	SpeedtestDownMbps    float64   `json:"speedtest_down_mbps"`
	SpeedtestUpMbps      float64   `json:"speedtest_up_mbps"`
	UpstreamPPPoEUsername string   `json:"upstream_pppoe_username,omitempty"` // Akun Dial Jartaplok saat instalasi
	UpstreamPPPoEPassword string   `json:"upstream_pppoe_password,omitempty"`
	Notes                string    `json:"notes,omitempty"`
	TechnicianName       string    `json:"technician_name,omitempty"` // Nama teknisi pelaksana BAST
	CreatedAt            time.Time `json:"created_at"`
}

// CustomerDocumentSite merepresentasikan data berkas pendaftaran dan BAST untuk satu lokasi pelanggan
type CustomerDocumentSite struct {
	RegistrationID       string      `json:"registration_id"`
	RegistrationNo       string      `json:"registration_no"`
	FullName             string      `json:"full_name"`
	IDCardNumber         string      `json:"id_card_number"`
	TaxID                string      `json:"tax_id,omitempty"`
	Phone                string      `json:"phone"`
	Email                string      `json:"email"`
	Address              string      `json:"address"`
	Latitude             float64     `json:"latitude"`
	Longitude            float64     `json:"longitude"`
	SelectedPlanID       string      `json:"selected_plan_id"`
	SelectedPlanName     string      `json:"selected_plan_name"`
	NearestODPCode       *string     `json:"nearest_odp_code,omitempty"`
	DistanceToODPMeters  float64     `json:"distance_to_odp_meters"`
	Status               string      `json:"status"`
	KTPPhotoURL          string      `json:"ktp_photo_url,omitempty"`
	HousePhotoURL        string      `json:"house_photo_url,omitempty"`
	ContractSignatureURL string      `json:"contract_signature_url,omitempty"`
	ContractSignedAt     *time.Time  `json:"contract_signed_at,omitempty"`
	WorkOrder            *WorkOrder  `json:"work_order,omitempty"`
	CreatedAt            time.Time   `json:"created_at"`
}

// CustomerDocumentsData agregat seluruh dokumen (KTP, Kontrak, BAST) dari semua lokasi seorang pelanggan
type CustomerDocumentsData struct {
	CustomerID   string                 `json:"customer_id"`
	FullName     string                 `json:"full_name"`
	Phone        string                 `json:"phone"`
	Email        string                 `json:"email"`
	IDCardNumber string                 `json:"id_card_number"`
	Sites        []CustomerDocumentSite `json:"sites"`
}

// DTOs
type CoverageCheckRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type CoverageCheckResponse struct {
	IsCovered      bool     `json:"is_covered"`
	DistanceMeters float64  `json:"distance_meters"`
	MaxDistance    float64  `json:"max_distance"`
	NearestODP     *ODPNode `json:"nearest_odp,omitempty"`
	Message        string   `json:"message"`
}

type SubmitRegistrationRequest struct {
	FullName         string  `json:"full_name"`
	Email            string  `json:"email"`
	Phone            string  `json:"phone"`
	IDCardNumber     string  `json:"id_card_number"`
	TaxID            string  `json:"tax_id,omitempty"` // NPWP (Opsional)
	Address          string  `json:"address"`
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	SelectedPlanID   string  `json:"selected_plan_id"`
	SelectedPlanName string  `json:"selected_plan_name"`
	MonthlyPrice     int64   `json:"monthly_price,omitempty"`
	OTCFee           int64   `json:"otc_fee,omitempty"`
	CustomNotes      string  `json:"custom_notes,omitempty"`
	KTPPhotoURL      string  `json:"ktp_photo_url,omitempty"`
	HousePhotoURL    string  `json:"house_photo_url,omitempty"`
	SitePICName      string  `json:"site_pic_name,omitempty"`
	SitePICPhone     string  `json:"site_pic_phone,omitempty"`
	ReferralCode     string  `json:"referral_code,omitempty"`
	BranchCode       string  `json:"branch_code,omitempty"`
}

type UpdateSitePICRequest struct {
	SitePICName  string `json:"site_pic_name"`
	SitePICPhone string `json:"site_pic_phone"`
}

type SubmitBASTRequest struct {
	ActualODPCode        string  `json:"actual_odp_code,omitempty"`
	OpticalPowerDBM      float64 `json:"optical_power_dbm"`
	ONTSerialNumber      string  `json:"ont_serial_number"`
	ONTMACAddress        string  `json:"ont_mac_address"`
	DropcoreLengthMeters int     `json:"dropcore_length_meters"`
	CustomerSignatureURL string  `json:"customer_signature_url,omitempty"`
	ProofPhotoURL        string  `json:"proof_photo_url,omitempty"` // Foto ONT
	HousePhotoURL        string  `json:"house_photo_url,omitempty"` // Foto Rumah Tampak Depan
	SpeedtestDownMbps    float64 `json:"speedtest_down_mbps"`
	SpeedtestUpMbps      float64 `json:"speedtest_up_mbps"`
	UpstreamPPPoEUsername string `json:"upstream_pppoe_username,omitempty"` // Akun Dial Modem Jartaplok (TIF/Telkom)
	UpstreamPPPoEPassword string `json:"upstream_pppoe_password,omitempty"`
	Notes                string  `json:"notes,omitempty"`
	TechnicianName       string  `json:"technician_name,omitempty"` // Teknisi pelaksana aktual
}

type ReassignODPRequest struct {
	ODPCode        string   `json:"odp_code"`
	DistanceMeters *float64 `json:"distance_meters,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

type CustomerLoginRequest struct {
	Email        string `json:"email,omitempty"`
	Identifier   string `json:"identifier,omitempty"`   // Email, WhatsApp, atau No. Registrasi
	Password     string `json:"password,omitempty"`     // Kata sandi akun pelanggan
	OTP          string `json:"otp,omitempty"`          // Kode OTP WhatsApp (jika auth_method=otp)
	SessionToken string `json:"session_token,omitempty"`// Token sesi aktif (jika auth_method=session)
	AuthMethod   string `json:"auth_method,omitempty"`  // "password" (default), "otp", atau "session"
}

type CustomerChangePasswordRequest struct {
	Identifier  string `json:"identifier"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type CustomerRequestOTPRequest struct {
	Phone string `json:"phone"`
}

type CustomerOTP struct {
	ID        string    `json:"id"`
	Phone     string    `json:"phone"`
	OTPCode   string    `json:"otp_code"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

type SuspendRegistrationRequest struct {
	Reason string `json:"reason"` // Alasan jeda layanan (e.g. Cuti, Renovasi, Tunggakan, dsb.)
}

type CustomerSiteAccount struct {
	Registration *Registration `json:"registration"`
	WorkOrder    *WorkOrder    `json:"work_order,omitempty"`
}

type CustomerPortalData struct {
	SessionToken string                `json:"session_token,omitempty"`
	Registration *Registration         `json:"registration"`
	WorkOrder    *WorkOrder            `json:"work_order,omitempty"`
	Locations    []CustomerSiteAccount `json:"locations,omitempty"`
}

// StaffUser merepresentasikan pengguna internal ISP (Super Admin, NOC, Teknisi, Sales)
type StaffUser struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	FullName     string    `json:"full_name"`
	Role         string    `json:"role"` // Primary role: SUPER_ADMIN, ADMIN_NOC, TECHNICIAN, SALES, FINANCE, JARTAPLOK
	Roles        []string  `json:"roles"` // Multi-roles for Single Sign-On (e.g. ["ADMIN_NOC", "TECHNICIAN"])
	IsSuperuser  bool      `json:"is_superuser"`
	ContactPhone string    `json:"contact_phone"`
	Status       string    `json:"status"` // ACTIVE, INACTIVE
	BranchID     *string   `json:"branch_id,omitempty"`
	BranchCode   *string   `json:"branch_code,omitempty"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// LoginRequest merepresentasikan payload login akun staf internal
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ChangePasswordRequest merepresentasikan payload penggantian password mandiri
type ChangePasswordRequest struct {
	Username        string `json:"username"`
	OldPassword     string `json:"old_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// ResetPasswordRequest merepresentasikan payload reset password oleh Super User
type ResetPasswordRequest struct {
	Username    string `json:"username"`
	NewPassword string `json:"new_password"`
}

// LoginResponse merepresentasikan response token dan profil setelah login sukses
type LoginResponse struct {
	Token        string    `json:"token"`
	Role         string    `json:"role"`
	Roles        []string  `json:"roles"`
	IsSuperuser  bool      `json:"is_superuser"`
	FullName     string    `json:"full_name"`
	Username     string    `json:"username"`
	ContactPhone string    `json:"contact_phone"`
	BranchID     *string   `json:"branch_id,omitempty"`
	BranchCode   *string   `json:"branch_code,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// AuthSession merepresentasikan sesi aktif yang divalidasi oleh middleware
type AuthSession struct {
	Token       string    `json:"token"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	Role        string    `json:"role"`
	Roles       []string  `json:"roles"`
	IsSuperuser bool      `json:"is_superuser"`
	BranchID    *string   `json:"branch_id,omitempty"`
	BranchCode  *string   `json:"branch_code,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// ExecutiveOverview ringkasan metrik finansial & jaringan untuk Super User
type ExecutiveOverview struct {
	TotalActiveCustomers     int     `json:"total_active_customers"`
	TotalSuspendedCustomers  int     `json:"total_suspended_customers"`
	TotalRegisteredCustomers int     `json:"total_registered_customers"`
	EstimatedMRR             int64   `json:"estimated_mrr"`              // Proyeksi Omset Bulanan Rupiah (MRR)
	AnnualRunRate            int64   `json:"annual_run_rate"`            // Proyeksi Omset Tahunan (ARR = MRR * 12)
	ARPU                     int64   `json:"arpu"`                       // Average Revenue Per User (MRR / Active Cust)
	TotalSalesCommission     int64   `json:"total_sales_commission"`     // Total kewajiban komisi mitra
	TotalJartaplokCost       int64   `json:"total_jartaplok_cost"`       // Total HPP Sewa Port Rekanan JARTAPLOK (prorata & jeda)
	CostOfGoodsSoldPct       float64 `json:"cogs_pct"`                   // Rasio Beban Pokok Jartaplok vs MRR (%)
	NetGrossMargin           int64   `json:"net_gross_margin"`           // Laba Kotor Bersih ISP (MRR - Jartaplok)
	NetContributionMargin    int64   `json:"net_contribution_margin"`    // Laba Kontribusi (NetGrossMargin - Sales Commission)
	JartaplokMarginPct       float64 `json:"jartaplok_margin_pct"`       // Persentase Net Margin ISP
	TotalODPs                int     `json:"total_odps"`
	TotalODPPorts            int     `json:"total_odp_ports"`
	UsedODPPorts             int     `json:"used_odp_ports"`
	AvailableODPPorts        int     `json:"available_odp_ports"`
	PotentialHeadroomMRR     int64   `json:"potential_headroom_mrr"`     // Potensi omset jika sisa port terisi (AvailablePorts * ARPU)
	PendingWorkOrders        int     `json:"pending_work_orders"`
	CompletedWorkOrders      int     `json:"completed_work_orders"`
	BranchCode               string  `json:"branch_code,omitempty"`
	BranchName               string  `json:"branch_name,omitempty"`
}

// JartaplokTierSummary ringkasan tarif sewa port per tier kapasitas
type JartaplokTierSummary struct {
	SpeedMbps      int   `json:"speed_mbps"`
	RatePerPort    int64 `json:"rate_per_port"`
	ActivePorts    int   `json:"active_ports"`
	SuspendedPorts int   `json:"suspended_ports"`
	FullSubtotal   int64 `json:"full_subtotal"` // Subtotal jika tanpa prorata
	Subtotal       int64 `json:"subtotal"`      // Tagihan aktual (dengan prorata bulan ke-1 & jeda)
}

// JartaplokBillingSummary rekap tagihan wholesale bulanan untuk rekanan JARTAPLOK / Bitstream
type JartaplokBillingSummary struct {
	PartnerName         string                 `json:"partner_name"`
	ServiceType         string                 `json:"service_type"`        // SEWA_PORT_FO, BITSTREAM
	PricingModel        string                 `json:"pricing_model"`       // e.g. Zona-2 Sumatera (Symetric 1:1)
	OTCFee              int64                  `json:"otc_fee"`             // Biaya Pasang Baru / Aktivasi
	MaxDistanceMeters   float64                `json:"max_distance_meters"` // Batas Jarak Dropcore ODP (150m / 250m)
	PeriodMonth         string                 `json:"period_month"`
	TotalActivePorts    int                    `json:"total_active_ports"`
	TotalSuspendedPorts int                    `json:"total_suspended_ports"`
	TotalBillingAmount  int64                  `json:"total_billing_amount"`  // Total tagihan aktual setelah prorata & jeda
	TotalFullMonthly    int64                  `json:"total_full_monthly"`    // Total jika tarif sebulan penuh tanpa diskon prorata
	TotalSavingsProrata int64                  `json:"total_savings_prorata"` // Selisih penghematan dari prorata / masa jeda
	Tiers               []JartaplokTierSummary `json:"tiers"`
	TotalODPs           int                    `json:"total_odps"`
	TotalCapacityPorts  int                    `json:"total_capacity_ports"`
	AvailablePorts      int                    `json:"available_ports"`
	OccupancyPct        float64                `json:"occupancy_pct"`
}

// JartaplokActivePort sirkuit teknis port aktif/jeda tanpa data pribadi pelanggan ritel
type JartaplokActivePort struct {
	CircuitID        string `json:"circuit_id"`     // Sanitized ID, e.g. CKT-GNETBIARO-0001
	ODPCode          string `json:"odp_code"`
	ODPName          string `json:"odp_name"`
	PortNumber       int    `json:"port_number"`
	PackageSpeed     string `json:"package_speed"`
	MonthlyRental    int64  `json:"monthly_rental"`     // Tarif dasar 1 bulan penuh
	ProratedFee      int64  `json:"prorated_fee"`       // Tagihan periode ini setelah prorata/jeda
	IsProrated       bool   `json:"is_prorated"`        // True jika dihitung prorata bulan pertama atau jeda
	ActiveDays       int    `json:"active_days"`        // Jumlah hari aktif di bulan berjalan
	TotalDays        int    `json:"total_days"`         // Total hari dalam bulan kalender berjalan (28-31)
	ActivatedAt      string `json:"activated_at"`
	SuspendedAt      string `json:"suspended_at,omitempty"`
	SuspensionReason string `json:"suspension_reason,omitempty"`
	Status           string `json:"status"`             // ACTIVE atau SUSPENDED
}

// Konstanta Kebijakan Jeda Layanan (Suspension Policy) Rekanan Wholesale JARTAPLOK
const (
	SuspensionPolicyAllowedWithWaiver  = "ALLOWED_WITH_WAIVER"  // Boleh jeda & sewa port wholesale gratis / prorata
	SuspensionPolicyDisallowed         = "DISALLOWED"           // Dilarang jeda layanan karena komitmen kontrak (TIF)
	SuspensionPolicyAllowedFullBilling = "ALLOWED_FULL_BILLING" // Pelanggan boleh dijeda, tapi tagihan wholesale rekanan tetap 100% penuh
)

// JartaplokPartner profil perusahaan rekanan penyelenggara JARTAPLOK / Bitstream Wholesale
type JartaplokPartner struct {
	ID                string    `json:"id"`
	Code              string    `json:"code"` // e.g. GNET-BIARO, TIF
	Name              string    `json:"name"` // e.g. PT. GNET BIARO AKSES, PT Telkom Infrastruktur Indonesia
	APIKey            string    `json:"api_key"`
	ContactPhone      string    `json:"contact_phone"`
	CoverageArea      string    `json:"coverage_area"`
	ServiceType       string    `json:"service_type"`        // SEWA_PORT_FO (Port Sharing) atau BITSTREAM
	SuspensionPolicy  string    `json:"suspension_policy"`   // ALLOWED_WITH_WAIVER, DISALLOWED, ALLOWED_FULL_BILLING
	Rate20M           int64     `json:"rate_20m"`            // Tarif 20 Mbps (Bitstream)
	Rate30M           int64     `json:"rate_30m"`            // Tarif 30 Mbps (Bitstream)
	Rate40M           int64     `json:"rate_40m"`            // Tarif 40 Mbps (Bitstream)
	Rate50M           int64     `json:"rate_50m"`            // Tarif 50 Mbps
	Rate100M          int64     `json:"rate_100m"`           // Tarif 100 Mbps
	Rate150M          int64     `json:"rate_150m"`           // Tarif 150 Mbps
	Rate200M          int64     `json:"rate_200m"`           // Tarif 200 Mbps
	Rate300M          int64     `json:"rate_300m"`           // Tarif 300 Mbps
	OTCFee            int64     `json:"otc_fee"`             // Biaya Pasang Baru / Aktivasi per port (Rp 500.000 untuk TIF)
	MaxDistanceMeters float64   `json:"max_distance_meters"` // Batas Tarikan Kabel ODP ke Pelanggan (150m untuk TIF, 250m untuk GNET)
	PricingModel      string    `json:"pricing_model"`       // e.g. "Zona-2 Sumatera - Bitstream Intra Standard Symetric 1:1"
	TotalODPs         int       `json:"total_odps,omitempty"`
	TotalPorts        int       `json:"total_ports,omitempty"`
	ActivePorts       int       `json:"active_ports,omitempty"`
	BranchID          *string   `json:"branch_id,omitempty"`
	BranchCode        *string   `json:"branch_code,omitempty"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// KMLUploadResult ringkasan hasil impor/sinkronisasi file KML ODP
type KMLUploadResult struct {
	TotalPlacemarks int      `json:"total_placemarks"`
	Inserted        int      `json:"inserted"`
	Updated         int      `json:"updated"`
	Skipped         int      `json:"skipped"`
	ProviderID      string   `json:"provider_id"`
	ProviderName    string   `json:"provider_name"`
	Details         []string `json:"details,omitempty"`
}

// AttachPartnerODPRequest permohonan oper order / manual attach ke infrastruktur mitra wholesale (misal TIF)
type AttachPartnerODPRequest struct {
	PartnerCode           string  `json:"partner_code"`              // e.g. "TELKO-PYK"
	ODPCode               string  `json:"odp_code"`                  // e.g. "ODP-TIF-PYK-001"
	ODPName               string  `json:"odp_name,omitempty"`        // e.g. "Tiang TIF Sudirman No. 45"
	DistanceMeters        float64 `json:"distance_meters"`           // Jarak kabel (meter)
	UpstreamPPPoEUsername string  `json:"upstream_pppoe_username,omitempty"` // Akun Dial Modem Jartaplok (Opsional)
	UpstreamPPPoEPassword string  `json:"upstream_pppoe_password,omitempty"`
	Notes                 string  `json:"notes,omitempty"`           // Catatan oper order internal
}

// RejectUncoveredRequest permohonan pengalihan status pelanggan yang tidak tercover sama sekali
type RejectUncoveredRequest struct {
	Action string `json:"action"`           // "WISHLIST" (Antrean Perluasan) atau "CANCEL" (Batal Resmi)
	Reason string `json:"reason,omitempty"` // Alasan teknis internal
}

// UpdateRegistrationPricingRequest permohonan penetapan/perubahan biaya OTC dan tarif bulanan nego
type UpdateRegistrationPricingRequest struct {
	OTCFee                int64  `json:"otc_fee"`
	MonthlyPrice          int64  `json:"monthly_price"`
	OTCNotes              string `json:"otc_notes,omitempty"`
	TaxID                 string `json:"tax_id,omitempty"` // NPWP (Opsional)
	SelectedPlanID        string `json:"selected_plan_id,omitempty"`
	SelectedPlanName      string `json:"selected_plan_name,omitempty"`
	PromoteToInstall      bool   `json:"promote_to_install,omitempty"`
	RequestNOCApproval    bool   `json:"request_noc_approval,omitempty"`
	ODPCode               string `json:"odp_code,omitempty"`
	UpstreamPPPoEUsername string `json:"upstream_pppoe_username,omitempty"` // Akun Dial Modem Jartaplok
	UpstreamPPPoEPassword string `json:"upstream_pppoe_password,omitempty"`
}

// UpdateUpstreamPPPoERequest permohonan update akun dial transport modem jartaplok
type UpdateUpstreamPPPoERequest struct {
	UpstreamPPPoEUsername string `json:"upstream_pppoe_username"`
	UpstreamPPPoEPassword string `json:"upstream_pppoe_password"`
}

// AssignWorkOrderRequest permohonan penunjukan teknisi atau alokasi ke job pool
type AssignWorkOrderRequest struct {
	TechnicianName string `json:"technician_name"`
	Notes          string `json:"notes,omitempty"`
}

// NOCApprovalActionRequest permohonan persetujuan atau penolakan penarikan khusus dari NOC / Atasan
type NOCApprovalActionRequest struct {
	Action       string `json:"action"` // "APPROVE" atau "REJECT"
	ApproverName string `json:"approver_name,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

// UpgradeBandwidthRequest permohonan pergantian paket / upgrade bandwidth pelanggan
type UpgradeBandwidthRequest struct {
	NewPlanID       string `json:"new_plan_id"`
	NewPlanName     string `json:"new_plan_name"`
	NewMonthlyPrice int64  `json:"new_monthly_price"`
	EffectiveDate   string `json:"effective_date"` // "IMMEDIATE", "NEXT_BILLING_CYCLE", atau YYYY-MM-DD
	Notes           string `json:"notes,omitempty"`
}

// LiveSessionInfo merepresentasikan status koneksi PPPoE aktif dari FreeRADIUS
type LiveSessionInfo struct {
	RadAcctID        int64     `json:"radacctid"`
	AcctSessionID    string    `json:"acctsessionid"`
	Username         string    `json:"username"`
	GroupName        string    `json:"groupname"`
	NasIPAddress     string    `json:"nasipaddress"`
	NasPortID        string    `json:"nasportid"`
	AcctStartTime    time.Time `json:"acctstarttime"`
	AcctSessionTime  int64     `json:"acctsessiontime"` // detik
	CallingStationID string    `json:"callingstationid"` // MAC modem
	FramedIPAddress  string    `json:"framedipaddress"`  // IP pelanggan
	IsOnline         bool      `json:"is_online"`
}

// KickSessionRequest permintaan reset / disconnect sesi PPPoE dari dashboard
type KickSessionRequest struct {
	PPPoEUsername string `json:"pppoe_username"`
}

// KPI Structures
type TechnicianKPI struct {
	Name                string  `json:"name"`
	Role                string  `json:"role"`
	AssignedOrders      int     `json:"assigned_orders"`
	CompletedOrders     int     `json:"completed_orders"`
	PendingOrders       int     `json:"pending_orders"`
	AvgOpticalPowerDBM  float64 `json:"avg_optical_power_dbm"`
	TotalDropcoreMeters int     `json:"total_dropcore_meters"`
	ComplianceRatePct   float64 `json:"compliance_rate_pct"`
	Score               int     `json:"score"`
	Grade               string  `json:"grade"`
}

type SalesKPI struct {
	PartnerCode       string  `json:"partner_code"`
	Name              string  `json:"name"`
	TotalLeads        int     `json:"total_leads"`
	ActiveCustomers   int     `json:"active_customers"`
	ConversionRatePct float64 `json:"conversion_rate_pct"`
	TotalCommission   int64   `json:"total_commission"`
	Score             int     `json:"score"`
	Grade             string  `json:"grade"`
}

type StaffKPISummary struct {
	MonthYear         string          `json:"month_year"`
	BranchCode        string          `json:"branch_code,omitempty"`
	BranchName        string          `json:"branch_name,omitempty"`
	Technicians       []TechnicianKPI `json:"technicians"`
	Sales             []SalesKPI      `json:"sales"`
	TotalActiveSubs   int             `json:"total_active_subs"`
	TotalSPKDone      int             `json:"total_spk_done"`
	AvgTeamOpticalDBM float64         `json:"avg_team_optical_dbm"`
}

// ReferralCheckResult merepresentasikan hasil validasi auto-check referral
type ReferralCheckResult struct {
	Valid          bool    `json:"valid"`
	Type           string  `json:"type"` // "SALES_PARTNER" atau "CUSTOMER_REFERRAL"
	PartnerID      string  `json:"partner_id,omitempty"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	CommissionRate float64 `json:"commission_rate,omitempty"`
}

// ClusterSmartOLTConfig merepresentasikan konfigurasi SmartOLT atau FTTX per wilayah cluster
type ClusterSmartOLTConfig struct {
	ID              string    `json:"id"`
	ClusterName     string    `json:"cluster_name"`
	ProviderID      string    `json:"provider_id"`       // GNET-BIARO, TELKO-PYK, GOGIGA
	IntegrationType string    `json:"integration_type"` // SMARTOLT atau FTTX_INTERNAL
	SmartOLTURL     string    `json:"smartolt_url"`
	SmartOLTKey     string    `json:"smartolt_api_key"`
	OLTID           string    `json:"olt_id"`            // OLT ID di SmartOLT (misal "4")
	ZoneID          string    `json:"zone_id"`           // Zone ID di SmartOLT (misal "122")
	ZoneName        string    `json:"zone_name"`         // Nama Zone (misal "GOGIGA")
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type TestSmartOLTRequest struct {
	BaseURL     string `json:"base_url"`
	SmartOLTURL string `json:"smartolt_url,omitempty"`
	APIKey      string `json:"api_key"`
	APIToken    string `json:"api_token,omitempty"`
}

// FiberGridSyncRequest parameter koneksi dari ISP ke FiberGrid Jartaplok
type FiberGridSyncRequest struct {
	ServerURL      string `json:"server_url"`
	APIKey         string `json:"api_key"`
	ClusterArea    string `json:"cluster_area"`
	AutoImportODPs bool   `json:"auto_import_odps"`
}

// FiberGridContractProfile profil kemitraan wholesale di server FiberGrid
type FiberGridContractProfile struct {
	ID             string `json:"id"`
	CompanyName    string `json:"company_name"`
	PartnerInitial string `json:"partner_initial"`
	VlanID         int    `json:"vlan_id"`
	IPTVVlanID     int    `json:"iptv_vlan_id"`
	MaxPorts       int    `json:"max_ports"`
	Status         string `json:"status"`
	BillingModel   string `json:"billing_model"`
}

// FiberGridODP DTO titik pasif ODP yang disewakan oleh FiberGrid
type FiberGridODP struct {
	ID         string  `json:"id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	TotalPorts int     `json:"total_ports"`
	UsedPorts  int     `json:"used_ports"`
	Status     string  `json:"status"`
}

// FiberGridONT DTO perangkat ONT yang dialokasikan ke kontrak wholesale
type FiberGridONT struct {
	ID           string   `json:"id"`
	SerialNumber string   `json:"serial_number"`
	CustomerName string   `json:"customer_name"`
	PONPort      string   `json:"pon_port"`
	RxPowerDBM   *float64 `json:"rx_power_dbm"`
	TxPowerDBM   *float64 `json:"tx_power_dbm"`
	IsActive     bool     `json:"is_active"`
}

// FiberGridSyncResult hasil tarikan data otomatis dari FiberGrid
type FiberGridSyncResult struct {
	ConnectedAt       string                   `json:"connected_at"`
	ServerURL         string                   `json:"server_url"`
	Contract          FiberGridContractProfile `json:"contract"`
	TotalODPsFetched  int                      `json:"total_odps_fetched"`
	TotalODPsImported int                      `json:"total_odps_imported"`
	TotalONTsFetched  int                      `json:"total_onts_fetched"`
	MikrotikScript    string                   `json:"mikrotik_script"`
	ODPs              []FiberGridODP           `json:"odps"`
	ONTs              []FiberGridONT           `json:"onts"`
}



