package containers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/incus"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/api"
	"gorm.io/gorm"
)

const (
	defaultPage  = 1
	defaultLimit = 10
	maxLimit     = 100
)

var (
	ErrContainerNotFound = errors.New("container not found")
	ErrInvalidPagination = errors.New("invalid pagination")
	ErrIncusUnavailable  = errors.New("incus unavailable")
	ErrContainerAction   = errors.New("container action failed")
	ErrUnsupportedAction = errors.New("unsupported container action")
)

type Service struct {
	repo  *Repository
	incus *incus.Client
}

type ListInput struct {
	Page   int
	Limit  int
	Search string
}

type ListResult struct {
	Items      []ContainerSummary
	Page       int
	Limit      int
	TotalItems int64
	TotalPages int
	Search     string
}

type ContainerSummary struct {
	ID                string       `json:"id"`
	ServiceID         string       `json:"service_id"`
	OwnerUserID       string       `json:"owner_user_id"`
	OwnerDisplayName  string       `json:"owner_display_name"`
	OwnerTelegramID   int64        `json:"owner_telegram_id"`
	OwnerUsername     *string      `json:"owner_username,omitempty"`
	NodeID            string       `json:"node_id"`
	NodeName          string       `json:"node_name"`
	NodeDisplayName   string       `json:"node_display_name"`
	IncusInstanceName string       `json:"incus_instance_name"`
	ImageAlias        string       `json:"image_alias"`
	OSFamily          *string      `json:"os_family,omitempty"`
	InternalIP        *string      `json:"internal_ip,omitempty"`
	MainPublicIP      *string      `json:"main_public_ip,omitempty"`
	SSHPort           *int         `json:"ssh_port,omitempty"`
	Status            string       `json:"status"`
	ServiceStatus     string       `json:"service_status"`
	PackageName       string       `json:"package_name"`
	CPU               int          `json:"cpu"`
	RAMMB             int          `json:"ram_mb"`
	DiskGB            int          `json:"disk_gb"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	Live              *LiveSummary `json:"live,omitempty"`
}

type LiveSummary struct {
	Available  bool    `json:"available"`
	Status     string  `json:"status,omitempty"`
	StatusCode int     `json:"status_code,omitempty"`
	Error      *string `json:"error,omitempty"`
}

type ContainerDetail struct {
	ContainerSummary
	PortMappings []model.ServicePortMapping `json:"port_mappings"`
	Domains      []model.ServiceDomain      `json:"domains"`
	LiveDetails  *LiveDetails               `json:"live_details,omitempty"`
}

type LiveDetails struct {
	Available     bool           `json:"available"`
	Status        string         `json:"status,omitempty"`
	StatusCode    int            `json:"status_code,omitempty"`
	Type          string         `json:"type,omitempty"`
	Location      string         `json:"location,omitempty"`
	Architecture  string         `json:"architecture,omitempty"`
	CreatedAt     *time.Time     `json:"created_at,omitempty"`
	LastUsedAt    *time.Time     `json:"last_used_at,omitempty"`
	ResourceUsage *ResourceUsage `json:"resource_usage,omitempty"`
	Error         *string        `json:"error,omitempty"`
}

type ResourceUsage struct {
	CPU    CPUUsage    `json:"cpu"`
	Memory MemoryUsage `json:"memory"`
	Disk   DiskUsage   `json:"disk"`
}

type CPUUsage struct {
	UsageNS         int64 `json:"usage_ns"`
	AllocatedTimeNS int64 `json:"allocated_time_ns"`
}

type MemoryUsage struct {
	UsageBytes     int64    `json:"usage_bytes"`
	PeakBytes      int64    `json:"peak_bytes"`
	TotalBytes     int64    `json:"total_bytes"`
	UsagePercent   *float64 `json:"usage_percent,omitempty"`
	SwapUsageBytes int64    `json:"swap_usage_bytes"`
	SwapPeakBytes  int64    `json:"swap_peak_bytes"`
}

type DiskUsage struct {
	RootDevice     string            `json:"root_device,omitempty"`
	UsageBytes     int64             `json:"usage_bytes"`
	TotalBytes     int64             `json:"total_bytes"`
	UsagePercent   *float64          `json:"usage_percent,omitempty"`
	Devices        []DiskDeviceUsage `json:"devices"`
	AggregateUsage int64             `json:"aggregate_usage"`
	AggregateTotal int64             `json:"aggregate_total"`
}

type DiskDeviceUsage struct {
	Name         string   `json:"name"`
	UsageBytes   int64    `json:"usage_bytes"`
	TotalBytes   int64    `json:"total_bytes"`
	UsagePercent *float64 `json:"usage_percent,omitempty"`
}

type ActionInput struct {
	ContainerID string
	Action      string
	Admin       model.AdminUser
}

type ActionResult struct {
	Container   ContainerDetail
	OperationID string
	Action      string
}

func NewService(repo *Repository, incusClient *incus.Client) *Service {
	return &Service{
		repo:  repo,
		incus: incusClient,
	}
}

func (s *Service) List(ctx context.Context, input ListInput) (*ListResult, error) {
	page := input.Page
	limit := input.Limit
	if page == 0 {
		page = defaultPage
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if page <= 0 || limit <= 0 || limit > maxLimit {
		return nil, ErrInvalidPagination
	}

	instances, totalItems, err := s.repo.FindAll(ctx, ListParams{
		Page:   page,
		Limit:  limit,
		Search: strings.TrimSpace(input.Search),
	})
	if err != nil {
		return nil, err
	}

	liveMap, liveErr := s.listLiveState()

	items := make([]ContainerSummary, 0, len(instances))
	for i := range instances {
		items = append(items, toContainerSummary(&instances[i], liveMap, liveErr))
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = int((totalItems + int64(limit) - 1) / int64(limit))
	}

	return &ListResult{
		Items:      items,
		Page:       page,
		Limit:      limit,
		TotalItems: totalItems,
		TotalPages: totalPages,
		Search:     strings.TrimSpace(input.Search),
	}, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*ContainerDetail, error) {
	instance, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContainerNotFound
		}

		return nil, err
	}

	summary := toContainerSummary(instance, nil, nil)
	detail := &ContainerDetail{
		ContainerSummary: summary,
		PortMappings:     instance.Service.PortMappings,
		Domains:          instance.Service.Domains,
	}

	if s.incus == nil || s.incus.Server() == nil {
		detail.LiveDetails = &LiveDetails{
			Available: false,
			Error:     pointer("incus client is not configured"),
		}
		return detail, nil
	}

	live, err := s.fetchLiveDetails(instance.IncusInstanceName)
	if err != nil {
		detail.LiveDetails = &LiveDetails{
			Available: false,
			Error:     pointer(err.Error()),
		}
		return detail, nil
	}

	detail.Live = &LiveSummary{
		Available:  true,
		Status:     live.Status,
		StatusCode: int(live.StatusCode),
	}
	detail.LiveDetails = live

	return detail, nil
}

func (s *Service) Act(ctx context.Context, input ActionInput) (*ActionResult, error) {
	requestedAction := normalizeRequestedAction(input.Action)
	if requestedAction == "" {
		return nil, ErrUnsupportedAction
	}
	incusAction := requestedToIncusAction(requestedAction)
	logAction := actionLogAction(requestedAction)

	if s.incus == nil || s.incus.Server() == nil {
		return nil, ErrIncusUnavailable
	}

	instance, err := s.repo.FindByID(ctx, input.ContainerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContainerNotFound
		}

		return nil, err
	}

	op, err := s.incus.Server().UpdateInstanceState(instance.IncusInstanceName, api.InstanceStatePut{
		Action: incusAction,
	}, "")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrContainerAction, err)
	}

	operationID := op.Get().ID
	if err := op.Wait(); err != nil {
		now := time.Now().UTC()
		instance.LastIncusOperationID = &operationID
		job := failedActionJob(instance, input.Admin, logAction, operationID, err, now)
		event := failedActionEvent(instance, input.Admin, requestedAction, operationID, err, now)
		_ = s.repo.SaveFailedAction(ctx, instance, job, event)
		return nil, fmt.Errorf("%w: %v", ErrContainerAction, err)
	}

	now := time.Now().UTC()
	instanceStatus, serviceStatus, suspendedAt := statusAfterAction(requestedAction, now)

	instance.Status = instanceStatus
	instance.UpdatedAt = now
	instance.LastIncusOperationID = &operationID

	service := instance.Service
	service.Status = serviceStatus
	service.UpdatedAt = now
	service.SuspendedAt = suspendedAt

	job := &model.ProvisioningJob{
		ID:               uuid.NewString(),
		ServiceID:        &instance.ServiceID,
		OrderID:          nil,
		JobType:          actionToJobType(requestedAction),
		Status:           "success",
		IncusOperationID: &operationID,
		RequestedByType:  "admin",
		RequestedByID:    &input.Admin.ID,
		AttemptCount:     1,
		Payload: map[string]any{
			"action":              logAction,
			"incus_action":        incusAction,
			"container_id":        instance.ID,
			"incus_instance_name": instance.IncusInstanceName,
		},
		StartedAt:  &now,
		FinishedAt: &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: service.ID,
		EventType: actionToEventType(requestedAction),
		ActorType: "admin",
		ActorID:   &input.Admin.ID,
		Summary:   actionSummary(requestedAction, instance.IncusInstanceName),
		Payload: map[string]any{
			"action":              requestedAction,
			"incus_action":        incusAction,
			"operation_id":        operationID,
			"container_id":        instance.ID,
			"incus_instance_name": instance.IncusInstanceName,
		},
		CreatedAt: now,
	}

	if err := s.repo.SaveActionResult(ctx, instance, service, job, event); err != nil {
		return nil, err
	}

	detail, err := s.GetByID(ctx, instance.ID)
	if err != nil {
		return nil, err
	}

	return &ActionResult{
		Container:   *detail,
		OperationID: operationID,
		Action:      logAction,
	}, nil
}

func (s *Service) listLiveState() (map[string]api.InstanceFull, error) {
	if s.incus == nil || s.incus.Server() == nil {
		return nil, ErrIncusUnavailable
	}

	instances, err := s.incus.Server().GetInstancesFull(api.InstanceTypeContainer)
	if err != nil {
		return nil, err
	}

	result := make(map[string]api.InstanceFull, len(instances))
	for i := range instances {
		result[instances[i].Name] = instances[i]
	}

	return result, nil
}

func (s *Service) fetchLiveDetails(instanceName string) (*LiveDetails, error) {
	server := s.incus.Server()
	if server == nil {
		return nil, ErrIncusUnavailable
	}

	instance, _, err := server.GetInstanceFull(instanceName)
	if err != nil {
		return nil, err
	}

	live := &LiveDetails{
		Available:    true,
		Status:       instance.Status,
		StatusCode:   int(instance.StatusCode),
		Type:         instance.Type,
		Location:     instance.Location,
		Architecture: instance.Architecture,
		CreatedAt:    &instance.CreatedAt,
		LastUsedAt:   &instance.LastUsedAt,
	}

	if instance.State != nil {
		resourceUsage := buildResourceUsage(instance.State)
		live.ResourceUsage = &resourceUsage
	}

	return live, nil
}

func toContainerSummary(instance *model.ServiceInstance, liveMap map[string]api.InstanceFull, liveErr error) ContainerSummary {
	summary := ContainerSummary{
		ID:                instance.ID,
		ServiceID:         instance.ServiceID,
		NodeID:            instance.NodeID,
		IncusInstanceName: instance.IncusInstanceName,
		ImageAlias:        instance.ImageAlias,
		OSFamily:          instance.OSFamily,
		InternalIP:        instance.InternalIP,
		MainPublicIP:      instance.MainPublicIP,
		SSHPort:           instance.SSHPort,
		Status:            normalizeLegacyStatus(instance.Status),
		CreatedAt:         instance.CreatedAt,
		UpdatedAt:         instance.UpdatedAt,
	}

	if instance.Node != nil {
		summary.NodeName = instance.Node.Name
		summary.NodeDisplayName = instance.Node.DisplayName
	}

	if instance.Service != nil {
		summary.ServiceStatus = normalizeLegacyStatus(instance.Service.Status)
		summary.PackageName = instance.Service.PackageNameSnapshot
		summary.CPU = instance.Service.CPUSnapshot
		summary.RAMMB = instance.Service.RAMMBSnapshot
		summary.DiskGB = instance.Service.DiskGBSnapshot
		summary.OwnerUserID = instance.Service.OwnerUserID

		if instance.Service.OwnerUser != nil {
			summary.OwnerDisplayName = instance.Service.OwnerUser.DisplayName
			summary.OwnerTelegramID = instance.Service.OwnerUser.TelegramID
			summary.OwnerUsername = instance.Service.OwnerUser.TelegramUsername
		}
	}

	if liveErr != nil {
		summary.Live = &LiveSummary{
			Available: false,
			Error:     pointer(liveErr.Error()),
		}
		return summary
	}

	if liveMap == nil {
		return summary
	}

	live, ok := liveMap[instance.IncusInstanceName]
	if !ok {
		summary.Live = &LiveSummary{
			Available: false,
			Error:     pointer("container not found on incus server"),
		}
		return summary
	}

	summary.Live = &LiveSummary{
		Available:  true,
		Status:     live.Status,
		StatusCode: int(live.StatusCode),
	}

	return summary
}

func buildResourceUsage(state *api.InstanceState) ResourceUsage {
	return ResourceUsage{
		CPU: CPUUsage{
			UsageNS:         state.CPU.Usage,
			AllocatedTimeNS: state.CPU.AllocatedTime,
		},
		Memory: buildMemoryUsage(state.Memory),
		Disk:   buildDiskUsage(state.Disk),
	}
}

func buildMemoryUsage(memory api.InstanceStateMemory) MemoryUsage {
	usagePercent := percentage(memory.Usage, memory.Total)
	return MemoryUsage{
		UsageBytes:     memory.Usage,
		PeakBytes:      memory.UsagePeak,
		TotalBytes:     memory.Total,
		UsagePercent:   usagePercent,
		SwapUsageBytes: memory.SwapUsage,
		SwapPeakBytes:  memory.SwapUsagePeak,
	}
}

func buildDiskUsage(disks map[string]api.InstanceStateDisk) DiskUsage {
	result := DiskUsage{
		Devices: make([]DiskDeviceUsage, 0, len(disks)),
	}

	for name, disk := range disks {
		device := DiskDeviceUsage{
			Name:         name,
			UsageBytes:   disk.Usage,
			TotalBytes:   disk.Total,
			UsagePercent: percentage(disk.Usage, disk.Total),
		}
		result.Devices = append(result.Devices, device)
		result.AggregateUsage += disk.Usage
		result.AggregateTotal += disk.Total

		if result.RootDevice == "" || name == "root" {
			result.RootDevice = name
			result.UsageBytes = disk.Usage
			result.TotalBytes = disk.Total
			result.UsagePercent = percentage(disk.Usage, disk.Total)
		}
	}

	if result.RootDevice == "" {
		result.UsageBytes = result.AggregateUsage
		result.TotalBytes = result.AggregateTotal
		result.UsagePercent = percentage(result.AggregateUsage, result.AggregateTotal)
	}

	return result
}

func percentage(usage int64, total int64) *float64 {
	if total <= 0 {
		return nil
	}

	value := (float64(usage) / float64(total)) * 100
	return &value
}

func normalizeRequestedAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "start":
		return "start"
	case "stop":
		return "stop"
	case "suspend":
		return "suspend"
	default:
		return ""
	}
}

func requestedToIncusAction(action string) string {
	if action == "suspend" {
		return "stop"
	}

	return action
}

func statusAfterAction(action string, now time.Time) (string, string, *time.Time) {
	switch action {
	case "start":
		return "running", "active", nil
	case "stop":
		return "stopped", "stopped", nil
	case "suspend":
		return "stopped", "stopped", nil
	default:
		return "failed", "failed", nil
	}
}

func actionToJobType(action string) string {
	switch action {
	case "start":
		return "start"
	case "stop":
		return "stop"
	case "suspend":
		return "stop"
	default:
		return "stop"
	}
}

func actionToEventType(action string) string {
	switch action {
	case "start":
		return "container_started"
	case "stop":
		return "container_stopped"
	case "suspend":
		return "container_stopped"
	default:
		return "container_action"
	}
}

func actionLogAction(action string) string {
	if action == "suspend" {
		return "stop"
	}
	return action
}

func actionSummary(action string, instanceName string) string {
	switch action {
	case "start":
		return fmt.Sprintf("Container %s started", instanceName)
	case "stop":
		return fmt.Sprintf("Container %s stopped", instanceName)
	case "suspend":
		return fmt.Sprintf("Container %s stopped", instanceName)
	default:
		return fmt.Sprintf("Container %s updated", instanceName)
	}
}

func pointer(value string) *string {
	return &value
}

func normalizeLegacyStatus(status string) string {
	if strings.EqualFold(strings.TrimSpace(status), "suspended") {
		return "stopped"
	}

	return status
}

func failedActionJob(instance *model.ServiceInstance, admin model.AdminUser, action string, operationID string, actionErr error, now time.Time) *model.ProvisioningJob {
	errorMessage := actionErr.Error()

	return &model.ProvisioningJob{
		ID:               uuid.NewString(),
		ServiceID:        &instance.ServiceID,
		JobType:          actionToJobType(action),
		Status:           "failed",
		IncusOperationID: &operationID,
		RequestedByType:  "admin",
		RequestedByID:    &admin.ID,
		AttemptCount:     1,
		ErrorMessage:     &errorMessage,
		Payload: map[string]any{
			"action":              action,
			"container_id":        instance.ID,
			"incus_instance_name": instance.IncusInstanceName,
		},
		StartedAt:  &now,
		FinishedAt: &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func failedActionEvent(instance *model.ServiceInstance, admin model.AdminUser, action string, operationID string, actionErr error, now time.Time) *model.ServiceEvent {
	return &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: instance.ServiceID,
		EventType: actionToEventType(action) + "_failed",
		ActorType: "admin",
		ActorID:   &admin.ID,
		Summary:   fmt.Sprintf("Failed to %s container %s", action, instance.IncusInstanceName),
		Payload: map[string]any{
			"action":              action,
			"operation_id":        operationID,
			"error":               actionErr.Error(),
			"container_id":        instance.ID,
			"incus_instance_name": instance.IncusInstanceName,
		},
		CreatedAt: now,
	}
}
