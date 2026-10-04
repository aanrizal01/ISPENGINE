package repository

import (
	"context"
	"ispsync/internal/domain"
)

type Storage interface {
	// Tenant
	GetTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error)
	GetTenantByCustomDomain(ctx context.Context, domainName string) (*domain.Tenant, error)
	ListTenants(ctx context.Context) ([]domain.Tenant, error)
	CreateTenant(ctx context.Context, tenant *domain.Tenant) error

	// Users / Auth
	GetUserByUsername(ctx context.Context, tenantID, username string) (*domain.User, error)
	ListUsersByTenant(ctx context.Context, tenantID string) ([]domain.User, error)

	// Plans
	ListPlans(ctx context.Context, tenantID string) ([]domain.Plan, error)
	GetPlanByID(ctx context.Context, tenantID, id string) (*domain.Plan, error)

	// ODPs
	ListODPs(ctx context.Context, tenantID string) ([]domain.ODP, error)
	GetNearestODP(ctx context.Context, tenantID string, lat, lng float64) (*domain.ODP, float64, error)
	UpsertODP(ctx context.Context, odp *domain.ODP) error

	// OLTs
	ListOLTs(ctx context.Context, tenantID string) ([]domain.OLT, error)
	GetOLTByID(ctx context.Context, tenantID, id string) (*domain.OLT, error)

	// Subscribers
	ListSubscribers(ctx context.Context, tenantID string, status string) ([]domain.Subscriber, error)
	GetSubscriberByID(ctx context.Context, tenantID, id string) (*domain.Subscriber, error)
	GetSubscriberByNo(ctx context.Context, tenantID, subNo string) (*domain.Subscriber, error)
	CreateSubscriber(ctx context.Context, sub *domain.Subscriber) error
	UpdateSubscriberStatus(ctx context.Context, tenantID, id, status string) error
	UpdateSubscriberProvisioning(ctx context.Context, tenantID, id string, oltID *string, ponPort *string, onuID *int, sn, mac *string, rxPower *float64, pppoeUser, pppoePass *string, vlan *int, ip *string) error

	// Work Orders
	ListWorkOrders(ctx context.Context, tenantID string, status string) ([]domain.WorkOrder, error)
	GetWorkOrderByID(ctx context.Context, tenantID, id string) (*domain.WorkOrder, error)
	CreateWorkOrder(ctx context.Context, wo *domain.WorkOrder) error
	CompleteWorkOrderBAST(ctx context.Context, tenantID, id string, rxPower float64, sn, mac, notes string) error

	// Custom Domain & TLS Check
	UpdateTenantCustomDomain(ctx context.Context, tenantID, customDomain string) error
	UpdateTenantProfile(ctx context.Context, tenantID, logoUrl, brandColor, contactPhone, contactEmail string) error
	CreateMikrotikRouter(ctx context.Context, router *domain.MikrotikRouter) error
	GetMikrotikRouter(ctx context.Context, tenantID string) (*domain.MikrotikRouter, error)
	ValidateDomainForTLS(ctx context.Context, domainName string) bool

	// Invoices (Siklus Penagihan Bulanan)
	ListInvoices(ctx context.Context, tenantID string, status string) ([]domain.Invoice, error)
	CreateInvoice(ctx context.Context, inv *domain.Invoice) error
	MarkInvoicePaid(ctx context.Context, tenantID, invoiceID string) error

	// Vouchers (Hotspot & Loket Fisik)
	ListVouchers(ctx context.Context, tenantID string) ([]domain.Voucher, error)
	GenerateVouchers(ctx context.Context, tenantID string, profileName string, speedDown, speedUp int, price float64, count int) (*domain.VoucherBatch, error)

	// Jartaplok (Local Fixed Network Sharing)
	ListJartaplokAgreements(ctx context.Context, tenantID string) ([]domain.JartaplokAgreement, error)
	CreateJartaplokAgreement(ctx context.Context, providerID, clientSlug, scope string, rate float64) (*domain.JartaplokAgreement, error)
	AddSharedODP(ctx context.Context, providerID, agreementID, odpCode string, ports int) error
	SetJartaplokAgreementStatus(ctx context.Context, providerID, agreementID, status string) error
	ConsumeSharedODPPort(ctx context.Context, clientID, odpCode string) (bool, error)
	GetTenantCapabilities(ctx context.Context, tenantID string) (domain.TenantCapabilities, error)
	SetTenantCapabilities(ctx context.Context, caps domain.TenantCapabilities) error

	// Add-ons & Staff Quota
	GetStaffQuotaStatus(ctx context.Context, tenantID string) (*domain.StaffQuotaStatus, error)
	ListTenantAddons(ctx context.Context, tenantID string) ([]domain.TenantAddon, error)
	CreateTenantAddon(ctx context.Context, addon *domain.TenantAddon) error
	CreateUser(ctx context.Context, user *domain.User, rawPassword string) error
	UpdateUserStatus(ctx context.Context, tenantID, userID, status string) error
	ResetUserPassword(ctx context.Context, tenantID, username, newPassword string) error

	// Close
	Close() error
}

