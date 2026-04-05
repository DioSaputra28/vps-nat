package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/gorm"
)

type fakeDNSResolver struct {
	lookupIPsFn func(ctx context.Context, host string) ([]string, error)
}

func (f fakeDNSResolver) LookupIPs(ctx context.Context, host string) ([]string, error) {
	if f.lookupIPsFn == nil {
		return nil, nil
	}
	return f.lookupIPsFn(ctx, host)
}

type fakeReverseProxyClient struct {
	ensureRouteFn func(ctx context.Context, input ReverseProxyRouteInput) error
	deleteRouteFn func(ctx context.Context, routeID string, proxyMode string) error
}

func (f fakeReverseProxyClient) EnsureRoute(ctx context.Context, input ReverseProxyRouteInput) error {
	if f.ensureRouteFn == nil {
		return nil
	}
	return f.ensureRouteFn(ctx, input)
}

func (f fakeReverseProxyClient) DeleteRoute(ctx context.Context, routeID string, proxyMode string) error {
	if f.deleteRouteFn == nil {
		return nil
	}
	return f.deleteRouteFn(ctx, routeID, proxyMode)
}

type fakeNetworkForwardManager struct {
	replaceFn func(ctx context.Context, publicIP string, privateIP string, current []modelServicePortMappingLike, next []RequestedPortMapping) (func(context.Context) error, error)
}

func (f fakeNetworkForwardManager) ReplaceServiceMappings(ctx context.Context, publicIP string, privateIP string, current []modelServicePortMappingLike, next []RequestedPortMapping) (func(context.Context) error, error) {
	if f.replaceFn == nil {
		return nil, nil
	}
	return f.replaceFn(ctx, publicIP, privateIP, current, next)
}

func TestDomainPreviewIncludesDNSReadinessAndUpstreamTarget(t *testing.T) {
	t.Parallel()

	svc, db := newActionServiceTestHarness(t)
	svc.dnsResolver = fakeDNSResolver{
		lookupIPsFn: func(_ context.Context, host string) ([]string, error) {
			if host != "app.example.com" {
				t.Fatalf("unexpected host lookup: %s", host)
			}
			return []string{"192.0.2.10"}, nil
		},
	}

	now := time.Now().UTC().Truncate(time.Second)
	seedActionServiceData(t, db, actionSeedInput{
		telegramID:       3010,
		userID:           "user-1",
		walletID:         "wallet-1",
		containerID:      "container-1",
		serviceID:        "service-1",
		orderID:          "order-1",
		packageID:        "pkg-1",
		packageName:      "Package S",
		packagePrice:     25000,
		packageDuration:  30,
		serviceStatus:    "active",
		instanceStatus:   "running",
		currentPackageOn: true,
		servicePrice:     25000,
		serviceCPU:       1,
		serviceRAMMB:     1024,
		serviceDiskGB:    10,
		expiresAt:        now.Add(24 * time.Hour),
		includeSSHMapping: true,
	})

	result, err := svc.DomainPreview(context.Background(), DomainPreviewInput{
		TelegramID:  3010,
		ContainerID: "container-1",
		Domain:      "app.example.com",
		TargetPort:  8080,
		ProxyMode:   "https",
	})
	if err != nil {
		t.Fatalf("DomainPreview returned error: %v", err)
	}

	if !result.DNSReady {
		t.Fatalf("expected dns_ready true")
	}
	if result.ExpectedPublicIP == nil || *result.ExpectedPublicIP != "192.0.2.10" {
		t.Fatalf("unexpected expected public ip: %#v", result.ExpectedPublicIP)
	}
	if result.UpstreamTarget != "10.0.0.2:8080" {
		t.Fatalf("unexpected upstream target: %s", result.UpstreamTarget)
	}
}

func TestDomainSubmitActivatesDomainAndWritesActivityLog(t *testing.T) {
	t.Parallel()

	svc, db := newActionServiceTestHarness(t)
	svc.dnsResolver = fakeDNSResolver{
		lookupIPsFn: func(_ context.Context, _ string) ([]string, error) {
			return []string{"192.0.2.10"}, nil
		},
	}
	ensureCalls := 0
	svc.reverseProxy = fakeReverseProxyClient{
		ensureRouteFn: func(_ context.Context, input ReverseProxyRouteInput) error {
			ensureCalls++
			if input.Domain != "app.example.com" || input.UpstreamDial != "10.0.0.2:8080" || input.ProxyMode != "https" {
				t.Fatalf("unexpected route input: %#v", input)
			}
			return nil
		},
	}

	now := time.Now().UTC().Truncate(time.Second)
	seedActionServiceData(t, db, actionSeedInput{
		telegramID:       3011,
		userID:           "user-1",
		walletID:         "wallet-1",
		containerID:      "container-1",
		serviceID:        "service-1",
		orderID:          "order-1",
		packageID:        "pkg-1",
		packageName:      "Package S",
		packagePrice:     25000,
		packageDuration:  30,
		serviceStatus:    "active",
		instanceStatus:   "running",
		currentPackageOn: true,
		servicePrice:     25000,
		serviceCPU:       1,
		serviceRAMMB:     1024,
		serviceDiskGB:    10,
		expiresAt:        now.Add(24 * time.Hour),
		includeSSHMapping: true,
	})

	result, err := svc.DomainSubmit(context.Background(), DomainSubmitInput{
		TelegramID:  3011,
		ContainerID: "container-1",
		Domain:      "app.example.com",
		TargetPort:  8080,
		ProxyMode:   "https",
	})
	if err != nil {
		t.Fatalf("DomainSubmit returned error: %v", err)
	}
	if ensureCalls != 1 {
		t.Fatalf("expected ensure route once, got %d", ensureCalls)
	}
	if result.Status != "active" || result.Domain.Status != "active" {
		t.Fatalf("expected active domain result, got %#v", result)
	}

	var count int64
	if err := db.Model(&model.ActivityLog{}).Where("action = ?", "domain.provisioned").Count(&count).Error; err != nil {
		t.Fatalf("failed counting activity logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 activity log, got %d", count)
	}
}

func TestReconfigIPPreviewIncludesCurrentAndProposedMappings(t *testing.T) {
	t.Parallel()

	svc, db := newActionServiceTestHarness(t)
	now := time.Now().UTC().Truncate(time.Second)
	seedActionServiceData(t, db, actionSeedInput{
		telegramID:       3012,
		userID:           "user-1",
		walletID:         "wallet-1",
		containerID:      "container-1",
		serviceID:        "service-1",
		orderID:          "order-1",
		packageID:        "pkg-1",
		packageName:      "Package S",
		packagePrice:     25000,
		packageDuration:  30,
		serviceStatus:    "active",
		instanceStatus:   "running",
		currentPackageOn: true,
		servicePrice:     25000,
		serviceCPU:       1,
		serviceRAMMB:     1024,
		serviceDiskGB:    10,
		expiresAt:        now.Add(24 * time.Hour),
		includeSSHMapping: true,
	})
	seedExtraPortMapping(t, db, model.ServicePortMapping{
		ID:          "mapping-web-1",
		ServiceID:   "service-1",
		MappingType: "custom",
		PublicIP:    "192.0.2.10",
		PublicPort:  22002,
		Protocol:    "tcp",
		TargetIP:    "10.0.0.2",
		TargetPort:  8080,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	result, err := svc.ReconfigIPPreview(context.Background(), ReconfigIPPreviewInput{
		TelegramID:  3012,
		ContainerID: "container-1",
	})
	if err != nil {
		t.Fatalf("ReconfigIPPreview returned error: %v", err)
	}
	if len(result.CurrentMappings) != 2 || len(result.ProposedMappings) != 2 {
		t.Fatalf("unexpected mapping counts: current=%d proposed=%d", len(result.CurrentMappings), len(result.ProposedMappings))
	}
	if result.ProposedSSHPort == 0 {
		t.Fatalf("expected proposed ssh port")
	}
}

func TestReconfigIPSubmitRotatesBundleAndRollsBackOnDBFailure(t *testing.T) {
	t.Parallel()

	svc, db := newActionServiceTestHarness(t)
	now := time.Now().UTC().Truncate(time.Second)
	seedActionServiceData(t, db, actionSeedInput{
		telegramID:       3013,
		userID:           "user-1",
		walletID:         "wallet-1",
		containerID:      "container-1",
		serviceID:        "service-1",
		orderID:          "order-1",
		packageID:        "pkg-1",
		packageName:      "Package S",
		packagePrice:     25000,
		packageDuration:  30,
		serviceStatus:    "active",
		instanceStatus:   "running",
		currentPackageOn: true,
		servicePrice:     25000,
		serviceCPU:       1,
		serviceRAMMB:     1024,
		serviceDiskGB:    10,
		expiresAt:        now.Add(24 * time.Hour),
		includeSSHMapping: true,
	})
	seedExtraPortMapping(t, db, model.ServicePortMapping{
		ID:          "mapping-web-1",
		ServiceID:   "service-1",
		MappingType: "custom",
		PublicIP:    "192.0.2.10",
		PublicPort:  22002,
		Protocol:    "tcp",
		TargetIP:    "10.0.0.2",
		TargetPort:  8080,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	rollbackCalls := 0
	svc.networkManager = fakeNetworkForwardManager{
		replaceFn: func(_ context.Context, publicIP string, privateIP string, current []modelServicePortMappingLike, next []RequestedPortMapping) (func(context.Context) error, error) {
			if publicIP != "192.0.2.10" || privateIP != "10.0.0.2" {
				t.Fatalf("unexpected manager input: %s %s", publicIP, privateIP)
			}
			if len(current) != 2 || len(next) != 2 {
				t.Fatalf("unexpected mapping counts: current=%d next=%d", len(current), len(next))
			}
			return func(context.Context) error {
				rollbackCalls++
				return nil
			}, nil
		},
	}

	if err := db.Callback().Create().Before("gorm:create").Register("test:fail-create-new-port-mapping", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "service_port_mappings" {
			tx.AddError(errors.New("forced mapping create failure"))
		}
	}); err != nil {
		t.Fatalf("failed to register create callback: %v", err)
	}
	defer db.Callback().Create().Remove("test:fail-create-new-port-mapping")

	_, err := svc.ReconfigIPSubmit(context.Background(), ReconfigIPSubmitInput{
		TelegramID:  3013,
		ContainerID: "container-1",
	})
	if err == nil {
		t.Fatalf("expected ReconfigIPSubmit to fail")
	}
	if rollbackCalls != 1 {
		t.Fatalf("expected rollback to be called once, got %d", rollbackCalls)
	}
}

func seedExtraPortMapping(t *testing.T, db *gorm.DB, mapping model.ServicePortMapping) {
	t.Helper()
	if err := db.Create(&mapping).Error; err != nil {
		t.Fatalf("failed to create extra port mapping: %v", err)
	}
}
