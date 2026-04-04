package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/api"
	"gorm.io/gorm"
)

type SyncStartInput struct {
	TelegramID       int64
	TelegramUsername *string
	DisplayName      string
	FirstName        *string
	LastName         *string
}

type SyncStartResult struct {
	User    model.User
	Created bool
}

type HomeInput struct {
	TelegramID int64
}

type HomeResult struct {
	User             HomeUser
	Wallet           HomeWallet
	Packages         []HomePackage
	OperatingSystems []string
	Rules            []string
	Platform         HomePlatform
}

type BuyVPSInput struct {
	TelegramID int64
}

type BuyVPSResult struct {
	User     HomeUser
	Wallet   HomeWallet
	Packages []HomePackage
}

type MyVPSInput struct {
	TelegramID int64
}

type MyVPSResult struct {
	Items []MyVPSItem `json:"items"`
}

type MyVPSItem struct {
	ContainerID       string     `json:"container_id"`
	ServiceID         string     `json:"service_id"`
	IncusInstanceName string     `json:"incus_instance_name"`
	PackageName       string     `json:"package_name"`
	Status            string     `json:"status"`
	ServiceStatus     string     `json:"service_status"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	RemainingDays     *int       `json:"remaining_days,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	LiveAvailable     bool       `json:"live_available"`
}

type MyVPSDetailInput struct {
	TelegramID  int64
	ContainerID string
}

type MyVPSDetailResult struct {
	ContainerID       string          `json:"container_id"`
	ServiceID         string          `json:"service_id"`
	IncusInstanceName string          `json:"incus_instance_name"`
	ImageAlias        string          `json:"image_alias"`
	OSFamily          *string         `json:"os_family,omitempty"`
	InternalIP        *string         `json:"internal_ip,omitempty"`
	MainPublicIP      *string         `json:"main_public_ip,omitempty"`
	SSHPort           *int            `json:"ssh_port,omitempty"`
	PackageName       string          `json:"package_name"`
	Status            string          `json:"status"`
	ServiceStatus     string          `json:"service_status"`
	CPULimit          int             `json:"cpu_limit"`
	RAMMBLimit        int             `json:"ram_mb_limit"`
	DiskGBLimit       int             `json:"disk_gb_limit"`
	Price             int64           `json:"price"`
	ExpiresAt         *time.Time      `json:"expires_at,omitempty"`
	RemainingDays     *int            `json:"remaining_days,omitempty"`
	UptimeSeconds     *int64          `json:"uptime_seconds,omitempty"`
	UptimeHuman       *string         `json:"uptime_human,omitempty"`
	Live              MyVPSLiveDetail `json:"live"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type MyVPSLiveDetail struct {
	Available   bool       `json:"available"`
	Status      string     `json:"status,omitempty"`
	StatusCode  int        `json:"status_code,omitempty"`
	Error       *string    `json:"error,omitempty"`
	ResourceUse MyVPSUsage `json:"resource_usage"`
}

type MyVPSUsage struct {
	CPU    MyVPSCPUUsage    `json:"cpu"`
	Memory MyVPSMemoryUsage `json:"memory"`
	Disk   MyVPSDiskUsage   `json:"disk"`
}

type MyVPSCPUUsage struct {
	UsageNS int64 `json:"usage_ns"`
}

type MyVPSMemoryUsage struct {
	UsageBytes   int64    `json:"usage_bytes"`
	TotalBytes   int64    `json:"total_bytes"`
	UsagePercent *float64 `json:"usage_percent,omitempty"`
}

type MyVPSDiskUsage struct {
	UsageBytes   int64    `json:"usage_bytes"`
	TotalBytes   int64    `json:"total_bytes"`
	UsagePercent *float64 `json:"usage_percent,omitempty"`
}

type HomeUser struct {
	ID               string    `json:"id"`
	TelegramID       int64     `json:"telegram_id"`
	TelegramUsername *string   `json:"telegram_username,omitempty"`
	DisplayName      string    `json:"display_name"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type HomeWallet struct {
	ID        string    `json:"id"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type HomePackage struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	CPU          int     `json:"cpu"`
	RAMMB        int     `json:"ram_mb"`
	DiskGB       int     `json:"disk_gb"`
	Price        int64   `json:"price"`
	DurationDays int     `json:"duration_days"`
}

type HomePlatform struct {
	Name           string `json:"name"`
	ServiceType    string `json:"service_type"`
	Virtualization string `json:"virtualization"`
	AccessMethod   string `json:"access_method"`
}

func (s *Service) SyncStart(ctx context.Context, input SyncStartInput) (*SyncStartResult, error) {
	if input.TelegramID <= 0 {
		return nil, ErrInvalidTelegramUser
	}

	displayName := resolveDisplayName(input.DisplayName, input.FirstName, input.LastName, input.TelegramUsername)
	if displayName == "" {
		return nil, ErrInvalidTelegramUser
	}

	username := normalizeOptionalString(input.TelegramUsername)
	now := time.Now().UTC()

	existing, err := s.repo.FindUserByTelegramID(ctx, input.TelegramID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		user := model.User{
			ID:               uuid.NewString(),
			TelegramID:       input.TelegramID,
			TelegramUsername: username,
			DisplayName:      displayName,
			Status:           "active",
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		wallet := model.Wallet{
			ID:        uuid.NewString(),
			UserID:    user.ID,
			Balance:   0,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := s.repo.CreateOrUpdate(ctx, &user, &wallet, true, false); err != nil {
			return nil, err
		}

		user.Wallet = &wallet
		return &SyncStartResult{User: user, Created: true}, nil
	}

	existing.TelegramUsername = username
	existing.DisplayName = displayName
	existing.UpdatedAt = now

	walletExists := existing.Wallet != nil
	var wallet model.Wallet
	if walletExists {
		wallet = *existing.Wallet
	} else {
		wallet = model.Wallet{
			ID:        uuid.NewString(),
			UserID:    existing.ID,
			Balance:   0,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	if err := s.repo.CreateOrUpdate(ctx, existing, &wallet, false, walletExists); err != nil {
		return nil, err
	}

	existing.Wallet = &wallet
	return &SyncStartResult{User: *existing, Created: false}, nil
}

func (s *Service) Home(ctx context.Context, input HomeInput) (*HomeResult, error) {
	if input.TelegramID <= 0 {
		return nil, ErrInvalidTelegramUser
	}

	user, err := s.repo.FindUserByTelegramID(ctx, input.TelegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTelegramUserNotFound
		}
		return nil, err
	}

	packages, err := s.repo.FindActivePackages(ctx)
	if err != nil {
		return nil, err
	}

	result := &HomeResult{
		User: HomeUser{
			ID:               user.ID,
			TelegramID:       user.TelegramID,
			TelegramUsername: user.TelegramUsername,
			DisplayName:      user.DisplayName,
			Status:           user.Status,
			CreatedAt:        user.CreatedAt,
			UpdatedAt:        user.UpdatedAt,
		},
		OperatingSystems: defaultOperatingSystems(),
		Rules:            defaultRules(),
		Platform:         defaultPlatform(),
	}

	if user.Wallet != nil {
		result.Wallet = HomeWallet{
			ID:        user.Wallet.ID,
			Balance:   user.Wallet.Balance,
			CreatedAt: user.Wallet.CreatedAt,
			UpdatedAt: user.Wallet.UpdatedAt,
		}
	}

	result.Packages = make([]HomePackage, 0, len(packages))
	for i := range packages {
		result.Packages = append(result.Packages, HomePackage{
			ID:           packages[i].ID,
			Name:         packages[i].Name,
			Description:  packages[i].Description,
			CPU:          packages[i].CPU,
			RAMMB:        packages[i].RAMMB,
			DiskGB:       packages[i].DiskGB,
			Price:        packages[i].Price,
			DurationDays: packages[i].DurationDays,
		})
	}

	return result, nil
}

func (s *Service) BuyVPS(ctx context.Context, input BuyVPSInput) (*BuyVPSResult, error) {
	home, err := s.Home(ctx, HomeInput{TelegramID: input.TelegramID})
	if err != nil {
		return nil, err
	}

	return &BuyVPSResult{
		User:     home.User,
		Wallet:   home.Wallet,
		Packages: home.Packages,
	}, nil
}

func (s *Service) MyVPS(ctx context.Context, input MyVPSInput) (*MyVPSResult, error) {
	if input.TelegramID <= 0 {
		return nil, ErrInvalidTelegramUser
	}

	user, err := s.repo.FindUserByTelegramID(ctx, input.TelegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTelegramUserNotFound
		}
		return nil, err
	}

	instances, err := s.repo.FindUserServiceInstances(ctx, user.TelegramID)
	if err != nil {
		return nil, err
	}

	liveMap := s.fetchMyVPSLiveStatuses(instances)
	result := &MyVPSResult{Items: make([]MyVPSItem, 0, len(instances))}
	for i := range instances {
		result.Items = append(result.Items, buildMyVPSItem(&instances[i], liveMap))
	}

	return result, nil
}

func (s *Service) MyVPSDetail(ctx context.Context, input MyVPSDetailInput) (*MyVPSDetailResult, error) {
	if input.TelegramID <= 0 || strings.TrimSpace(input.ContainerID) == "" {
		return nil, ErrInvalidTelegramUser
	}

	instance, err := s.repo.FindUserServiceInstanceByID(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMyVPSNotFound
		}
		return nil, err
	}

	result := &MyVPSDetailResult{
		ContainerID:       instance.ID,
		ServiceID:         instance.ServiceID,
		IncusInstanceName: instance.IncusInstanceName,
		ImageAlias:        instance.ImageAlias,
		OSFamily:          instance.OSFamily,
		InternalIP:        instance.InternalIP,
		MainPublicIP:      instance.MainPublicIP,
		SSHPort:           instance.SSHPort,
		Status:            instance.Status,
		CreatedAt:         instance.CreatedAt,
		UpdatedAt:         instance.UpdatedAt,
		Live: MyVPSLiveDetail{
			Available: false,
			ResourceUse: MyVPSUsage{
				CPU:    MyVPSCPUUsage{},
				Memory: MyVPSMemoryUsage{},
				Disk:   MyVPSDiskUsage{},
			},
		},
	}

	if instance.Service != nil {
		result.PackageName = instance.Service.PackageNameSnapshot
		result.ServiceStatus = instance.Service.Status
		result.CPULimit = instance.Service.CPUSnapshot
		result.RAMMBLimit = instance.Service.RAMMBSnapshot
		result.DiskGBLimit = instance.Service.DiskGBSnapshot
		result.Price = instance.Service.PriceSnapshot
		result.ExpiresAt = instance.Service.ExpiresAt
		result.RemainingDays = remainingDays(instance.Service.ExpiresAt)
	}

	if s.incus == nil || s.incus.Server() == nil {
		result.Live.Error = stringPointer("incus client is not configured")
		return result, nil
	}

	live, _, err := s.incus.Server().GetInstanceFull(instance.IncusInstanceName)
	if err != nil {
		result.Live.Error = stringPointer(err.Error())
		return result, nil
	}

	result.Status = normalizeStatus(live.Status, result.Status)
	result.Live.Available = true
	result.Live.Status = live.Status
	result.Live.StatusCode = int(live.StatusCode)

	if live.State != nil {
		result.Live.ResourceUse = buildMyVPSUsage(live.State)
		if !live.State.StartedAt.IsZero() {
			uptimeSeconds := int64(time.Since(live.State.StartedAt).Seconds())
			result.UptimeSeconds = &uptimeSeconds
			human := humanizeDuration(time.Duration(uptimeSeconds) * time.Second)
			result.UptimeHuman = &human
		}
	}

	return result, nil
}

func resolveDisplayName(displayName string, firstName *string, lastName *string, username *string) string {
	if trimmed := strings.TrimSpace(displayName); trimmed != "" {
		return trimmed
	}

	parts := []string{}
	if firstName != nil {
		if trimmed := strings.TrimSpace(*firstName); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if lastName != nil {
		if trimmed := strings.TrimSpace(*lastName); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}

	if username != nil {
		if trimmed := strings.TrimSpace(*username); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func defaultOperatingSystems() []string {
	return []string{"Debian 10", "Debian 11", "Debian 12", "Debian 13", "Ubuntu 20.04", "Ubuntu 22.04", "Ubuntu 24.04", "Kali Linux"}
}

func defaultRules() []string {
	return []string{
		"Dilarang melakukan aktivitas ilegal atau penyalahgunaan layanan.",
		"Dilarang DDoS, botnet, phishing, carding, atau spam abuse.",
		"Dilarang penggunaan resource secara berlebihan dan terus-menerus tanpa batas wajar.",
		"Dilarang VPN, proxy, dan tunneling berlebihan jika mengganggu stabilitas node.",
		"Pelanggaran dapat menyebabkan suspend permanen tanpa refund.",
	}
}

func defaultPlatform() HomePlatform {
	return HomePlatform{Name: "VPS NAT", ServiceType: "VPS NAT", Virtualization: "Incus Container", AccessMethod: "SSH"}
}

func fetchStatusMap(instances []model.ServiceInstance, fullInstances []api.InstanceFull) map[string]api.InstanceFull {
	result := make(map[string]api.InstanceFull, len(instances))
	if len(fullInstances) == 0 {
		return result
	}

	fullMap := make(map[string]api.InstanceFull, len(fullInstances))
	for i := range fullInstances {
		fullMap[fullInstances[i].Name] = fullInstances[i]
	}
	for i := range instances {
		if live, ok := fullMap[instances[i].IncusInstanceName]; ok {
			result[instances[i].IncusInstanceName] = live
		}
	}
	return result
}

func (s *Service) fetchMyVPSLiveStatuses(instances []model.ServiceInstance) map[string]api.InstanceFull {
	if s.incus == nil || s.incus.Server() == nil || len(instances) == 0 {
		return map[string]api.InstanceFull{}
	}

	fullInstances, err := s.incus.Server().GetInstancesFull(api.InstanceTypeContainer)
	if err != nil {
		return map[string]api.InstanceFull{}
	}
	return fetchStatusMap(instances, fullInstances)
}

func buildMyVPSItem(instance *model.ServiceInstance, liveMap map[string]api.InstanceFull) MyVPSItem {
	item := MyVPSItem{
		ContainerID:       instance.ID,
		ServiceID:         instance.ServiceID,
		IncusInstanceName: instance.IncusInstanceName,
		Status:            instance.Status,
		CreatedAt:         instance.CreatedAt,
	}
	if instance.Service != nil {
		item.PackageName = instance.Service.PackageNameSnapshot
		item.ServiceStatus = instance.Service.Status
		item.ExpiresAt = instance.Service.ExpiresAt
		item.RemainingDays = remainingDays(instance.Service.ExpiresAt)
	}
	if live, ok := liveMap[instance.IncusInstanceName]; ok {
		item.Status = normalizeStatus(live.Status, item.Status)
		item.LiveAvailable = true
	}
	return item
}

func buildMyVPSUsage(state *api.InstanceState) MyVPSUsage {
	return MyVPSUsage{
		CPU: MyVPSCPUUsage{UsageNS: state.CPU.Usage},
		Memory: MyVPSMemoryUsage{
			UsageBytes:   state.Memory.Usage,
			TotalBytes:   state.Memory.Total,
			UsagePercent: percentage(state.Memory.Usage, state.Memory.Total),
		},
		Disk: buildMyVPSDiskUsage(state.Disk),
	}
}

func buildMyVPSDiskUsage(disks map[string]api.InstanceStateDisk) MyVPSDiskUsage {
	if disk, ok := disks["root"]; ok {
		return MyVPSDiskUsage{
			UsageBytes:   disk.Usage,
			TotalBytes:   disk.Total,
			UsagePercent: percentage(disk.Usage, disk.Total),
		}
	}

	var usage int64
	var total int64
	for _, disk := range disks {
		usage += disk.Usage
		total += disk.Total
	}
	return MyVPSDiskUsage{
		UsageBytes:   usage,
		TotalBytes:   total,
		UsagePercent: percentage(usage, total),
	}
}

func remainingDays(expiresAt *time.Time) *int {
	if expiresAt == nil || expiresAt.IsZero() {
		return nil
	}
	remaining := int(time.Until(*expiresAt).Hours() / 24)
	if remaining < 0 {
		remaining = 0
	}
	return &remaining
}

func percentage(usage int64, total int64) *float64 {
	if total <= 0 {
		return nil
	}
	value := (float64(usage) / float64(total)) * 100
	return &value
}

func normalizeStatus(liveStatus string, fallback string) string {
	if strings.TrimSpace(liveStatus) == "" {
		return fallback
	}
	return strings.ToLower(strings.TrimSpace(liveStatus))
}

func humanizeDuration(duration time.Duration) string {
	if duration <= 0 {
		return "0m"
	}
	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	parts := []string{}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dhari", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%djam", hours))
	}
	if minutes > 0 && days == 0 {
		parts = append(parts, fmt.Sprintf("%dmenit", minutes))
	}
	if len(parts) == 0 {
		return "0m"
	}
	return strings.Join(parts, " ")
}

func stringPointer(value string) *string {
	return &value
}
