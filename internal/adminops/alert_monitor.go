package adminops

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/google/uuid"
	"github.com/lxc/incus/v6/shared/api"
	"gorm.io/gorm"
)

type AlertMonitor struct {
	repo     *Repository
	server   IncusServer
	notifier AlertNotifier
	config   AlertMonitorConfig

	mu      sync.Mutex
	samples map[string]alertSampleState
	now     func() time.Time

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
}

type alertSampleState struct {
	CPUUsage      int64
	AllocatedTime int64
	SampledAt     time.Time
	CPUAboveSince *time.Time
	RAMAboveSince *time.Time
}

func NewAlertMonitor(repo *Repository, server IncusServer, notifier AlertNotifier, config AlertMonitorConfig) *AlertMonitor {
	if config.Interval <= 0 {
		config.Interval = 30 * time.Second
	}
	if config.ThresholdPercent <= 0 {
		config.ThresholdPercent = 95
	}
	if config.Duration <= 0 {
		config.Duration = 10 * time.Minute
	}

	return &AlertMonitor{
		repo:     repo,
		server:   server,
		notifier: notifier,
		config:   config,
		samples:  map[string]alertSampleState{},
		now: func() time.Time {
			return time.Now().UTC()
		},
		stopCh: make(chan struct{}),
	}
}

func (m *AlertMonitor) Start() {
	if m == nil || m.server == nil {
		return
	}

	m.startOnce.Do(func() {
		go m.loop()
	})
}

func (m *AlertMonitor) Stop() {
	if m == nil {
		return
	}

	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}

func (m *AlertMonitor) loop() {
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	_ = m.RunOnce(context.Background())

	for {
		select {
		case <-ticker.C:
			if err := m.RunOnce(context.Background()); err != nil {
				log.Printf("[alerts][monitor] run failed: %v", err)
			}
		case <-m.stopCh:
			return
		}
	}
}

func (m *AlertMonitor) RunOnce(ctx context.Context) error {
	if m == nil || m.server == nil {
		return ErrMetricsUnavailable
	}

	instances, err := m.server.GetInstancesFull(api.InstanceTypeContainer)
	if err != nil {
		return err
	}

	monitored, err := m.repo.ListMonitoredInstances(ctx)
	if err != nil {
		return err
	}

	now := m.now()
	liveMap := make(map[string]api.InstanceFull, len(instances))
	for i := range instances {
		liveMap[instances[i].Name] = instances[i]
	}

	serviceIDs := make([]string, 0, len(monitored))
	for i := range monitored {
		serviceIDs = append(serviceIDs, monitored[i].ServiceID)
	}

	openAlerts, err := m.repo.FindOpenAlertsForServices(ctx, uniqueStrings(serviceIDs))
	if err != nil {
		return err
	}
	openAlertMap := make(map[string]model.ResourceAlert, len(openAlerts))
	for i := range openAlerts {
		key := alertKey(ptrValue(openAlerts[i].ServiceID), openAlerts[i].AlertType)
		openAlertMap[key] = openAlerts[i]
	}

	nextSamples := make(map[string]alertSampleState, len(monitored))

	for i := range monitored {
		instance := monitored[i]
		previous := m.sampleForInstance(instance.IncusInstanceName)
		current := previous
		live, ok := liveMap[instance.IncusInstanceName]
		if !ok || live.State == nil || !strings.EqualFold(live.State.Status, "Running") {
			if err := m.resolveOpenAlert(ctx, instance, openAlertMap, "cpu_high", now); err != nil {
				return err
			}
			if err := m.resolveOpenAlert(ctx, instance, openAlertMap, "ram_high", now); err != nil {
				return err
			}
			current.CPUAboveSince = nil
			current.RAMAboveSince = nil
			nextSamples[instance.IncusInstanceName] = current
			continue
		}

		state := live.State
		allocatedTime := state.CPU.AllocatedTime
		if allocatedTime <= 0 {
			allocatedTime = 1_000_000_000
		}

		cpuPercent := computeCPUPercent(previous, state.CPU.Usage, allocatedTime, now)
		ramPercent := percentageFloat64(state.Memory.Usage, state.Memory.Total)

		current.CPUUsage = state.CPU.Usage
		current.AllocatedTime = allocatedTime
		current.SampledAt = now

		cpuAbove, cpuSince := updateAboveSince(current.CPUAboveSince, cpuPercent, m.config.ThresholdPercent, now, previous.SampledAt)
		current.CPUAboveSince = cpuSince
		if cpuAbove && cpuSince != nil && now.Sub(*cpuSince) >= m.config.Duration {
			if err := m.ensureOpenAlert(ctx, instance, openAlertMap, "cpu_high", *cpuPercent, now); err != nil {
				return err
			}
		} else if !cpuAbove {
			if err := m.resolveOpenAlert(ctx, instance, openAlertMap, "cpu_high", now); err != nil {
				return err
			}
		}

		ramAbove, ramSince := updateAboveSince(current.RAMAboveSince, ramPercent, m.config.ThresholdPercent, now, previous.SampledAt)
		current.RAMAboveSince = ramSince
		if ramAbove && ramSince != nil && now.Sub(*ramSince) >= m.config.Duration {
			if err := m.ensureOpenAlert(ctx, instance, openAlertMap, "ram_high", *ramPercent, now); err != nil {
				return err
			}
		} else if !ramAbove {
			if err := m.resolveOpenAlert(ctx, instance, openAlertMap, "ram_high", now); err != nil {
				return err
			}
		}

		nextSamples[instance.IncusInstanceName] = current
	}

	m.mu.Lock()
	m.samples = nextSamples
	m.mu.Unlock()
	return nil
}

func (m *AlertMonitor) sampleForInstance(name string) alertSampleState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.samples[name]
}

func (m *AlertMonitor) ensureOpenAlert(ctx context.Context, instance model.ServiceInstance, openAlertMap map[string]model.ResourceAlert, alertType string, currentPercent float64, now time.Time) error {
	key := alertKey(instance.ServiceID, alertType)
	if _, exists := openAlertMap[key]; exists {
		return nil
	}

	serviceID := instance.ServiceID
	nodeID := instance.NodeID
	alert := model.ResourceAlert{
		ID:               uuid.NewString(),
		ServiceID:        &serviceID,
		NodeID:           &nodeID,
		AlertType:        alertType,
		ThresholdPercent: m.config.ThresholdPercent,
		DurationMinutes:  int(m.config.Duration / time.Minute),
		Status:           "open",
		OpenedAt:         now,
		Metadata: map[string]any{
			"instance_name":   instance.IncusInstanceName,
			"node_id":         instance.NodeID,
			"current_percent": currentPercent,
			"sampled_at":      now,
			"private_ip":      instance.InternalIP,
			"service_status":  instance.Service.Status,
		},
	}
	logEntry := model.ActivityLog{
		ID:         uuid.NewString(),
		ActorType:  "system",
		Action:     "resource_alert.opened",
		TargetType: "resource_alert",
		TargetID:   &alert.ID,
		Metadata: map[string]any{
			"service_id":       serviceID,
			"node_id":          nodeID,
			"alert_type":       alertType,
			"threshold_percent": m.config.ThresholdPercent,
			"duration_minutes": alert.DurationMinutes,
			"current_percent":  currentPercent,
		},
		CreatedAt: now,
	}

	if err := m.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&alert).Error; err != nil {
			return err
		}
		return tx.Create(&logEntry).Error
	}); err != nil {
		return err
	}

	openAlertMap[key] = alert

	if m.notifier != nil {
		message := fmt.Sprintf(
			"Alert %s terbuka untuk service %s (%s) pada node %s, usage %.2f%% selama >= %d menit",
			alertType,
			instance.ServiceID,
			instance.IncusInstanceName,
			instance.NodeID,
			currentPercent,
			alert.DurationMinutes,
		)
		if err := m.notifier.NotifyAlert(ctx, message); err != nil {
			log.Printf("[alerts][monitor] notify failed alert_id=%s: %v", alert.ID, err)
			_ = m.repo.CreateActivityLog(ctx, &model.ActivityLog{
				ID:         uuid.NewString(),
				ActorType:  "system",
				Action:     "resource_alert.notify_failed",
				TargetType: "resource_alert",
				TargetID:   &alert.ID,
				Metadata: map[string]any{
					"error": err.Error(),
				},
				CreatedAt: now,
			})
		}
	}

	return nil
}

func (m *AlertMonitor) resolveOpenAlert(ctx context.Context, instance model.ServiceInstance, openAlertMap map[string]model.ResourceAlert, alertType string, now time.Time) error {
	key := alertKey(instance.ServiceID, alertType)
	alert, exists := openAlertMap[key]
	if !exists {
		return nil
	}

	if err := m.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ResourceAlert{}).
			Where("id = ?", alert.ID).
			Updates(map[string]any{
				"status":      "resolved",
				"resolved_at": now,
			}).Error; err != nil {
			return err
		}
		return tx.Create(&model.ActivityLog{
			ID:         uuid.NewString(),
			ActorType:  "system",
			Action:     "resource_alert.resolved",
			TargetType: "resource_alert",
			TargetID:   &alert.ID,
			Metadata: map[string]any{
				"service_id":  instance.ServiceID,
				"node_id":     instance.NodeID,
				"alert_type":  alertType,
				"resolved_at": now,
			},
			CreatedAt: now,
		}).Error
	}); err != nil {
		return err
	}

	delete(openAlertMap, key)
	return nil
}

func alertKey(serviceID string, alertType string) string {
	return serviceID + ":" + alertType
}

func ptrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func computeCPUPercent(previous alertSampleState, currentUsage int64, allocatedTime int64, now time.Time) *float64 {
	if previous.SampledAt.IsZero() || previous.AllocatedTime <= 0 || currentUsage < previous.CPUUsage {
		return nil
	}
	elapsed := now.Sub(previous.SampledAt).Seconds()
	if elapsed <= 0 {
		return nil
	}
	usageDelta := float64(currentUsage - previous.CPUUsage)
	value := usageDelta / (float64(allocatedTime) * elapsed) * 100
	return &value
}

func percentageFloat64(usage int64, total int64) *float64 {
	if total <= 0 {
		return nil
	}
	value := (float64(usage) / float64(total)) * 100
	return &value
}

func updateAboveSince(current *time.Time, percent *float64, threshold int, now time.Time, fallbackStart time.Time) (bool, *time.Time) {
	if percent == nil || *percent < float64(threshold) {
		return false, nil
	}
	if current != nil {
		return true, current
	}
	if !fallbackStart.IsZero() {
		t := fallbackStart
		return true, &t
	}
	t := now
	return true, &t
}
