package adminops

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/lxc/incus/v6/shared/api"
)

type IncusServer interface {
	GetInstancesFull(instanceType api.InstanceType) ([]api.InstanceFull, error)
}

type DashboardCache struct {
	server   IncusServer
	interval time.Duration

	mu        sync.RWMutex
	snapshot  DashboardLiveSnapshot
	samples   map[string]dashboardInstanceSample
	lastError error
	lastRun   time.Time

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
}

type dashboardInstanceSample struct {
	CPUUsage      int64
	AllocatedTime int64
	SampledAt     time.Time
}

func NewDashboardCache(server IncusServer, interval time.Duration) *DashboardCache {
	if interval <= 0 {
		interval = 30 * time.Second
	}

	return &DashboardCache{
		server:   server,
		interval: interval,
		samples:  map[string]dashboardInstanceSample{},
		stopCh:   make(chan struct{}),
		snapshot: DashboardLiveSnapshot{
			LiveAvailable: false,
			Warning:       stringPtr("metrics not sampled yet"),
		},
	}
}

func (c *DashboardCache) Start() {
	if c == nil {
		return
	}

	c.startOnce.Do(func() {
		if c.server == nil {
			c.mu.Lock()
			warning := "incus unavailable"
			c.snapshot = DashboardLiveSnapshot{
				LiveAvailable: false,
				Warning:       &warning,
			}
			c.mu.Unlock()
			return
		}

		go c.loop()
	})
}

func (c *DashboardCache) Stop() {
	if c == nil {
		return
	}

	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

func (c *DashboardCache) Snapshot(ctx context.Context) DashboardLiveSnapshot {
	if c == nil {
		return DashboardLiveSnapshot{}
	}

	c.mu.RLock()
	snapshot := c.snapshot
	lastRun := c.lastRun
	c.mu.RUnlock()

	if snapshot.LiveAvailable && time.Since(lastRun) <= c.interval*2 {
		return snapshot
	}

	if err := c.Refresh(ctx); err != nil {
		log.Printf("[dashboard][cache] sync refresh failed: %v", err)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot
}

func (c *DashboardCache) Refresh(ctx context.Context) error {
	if c == nil {
		return nil
	}

	if c.server == nil {
		warning := "incus unavailable"
		c.mu.Lock()
		c.snapshot = DashboardLiveSnapshot{
			LiveAvailable: false,
			Warning:       &warning,
		}
		c.lastError = ErrMetricsUnavailable
		c.lastRun = time.Now().UTC()
		c.mu.Unlock()
		return ErrMetricsUnavailable
	}

	instances, err := c.server.GetInstancesFull(api.InstanceTypeContainer)
	if err != nil {
		warning := fmt.Sprintf("incus metrics unavailable: %v", err)
		c.mu.Lock()
		c.snapshot = DashboardLiveSnapshot{
			LiveAvailable: false,
			Warning:       &warning,
		}
		c.lastError = err
		c.lastRun = time.Now().UTC()
		c.mu.Unlock()
		return err
	}

	now := time.Now().UTC()
	nextSamples := make(map[string]dashboardInstanceSample, len(instances))
	var totalRAM int64
	var totalDisk int64
	var cpuPercentSum float64
	warmingUp := false
	instanceCount := int64(0)

	c.mu.RLock()
	prevSamples := c.samples
	c.mu.RUnlock()

	for _, instance := range instances {
		state := instance.State
		if state == nil || !strings.EqualFold(state.Status, "Running") {
			continue
		}

		instanceCount++
		totalRAM += state.Memory.Usage
		for _, disk := range state.Disk {
			totalDisk += disk.Usage
		}

		allocatedTime := state.CPU.AllocatedTime
		if allocatedTime <= 0 {
			allocatedTime = 1_000_000_000
		}

		prev, ok := prevSamples[instance.Name]
		if !ok || prev.AllocatedTime <= 0 || state.CPU.Usage < prev.CPUUsage {
			warmingUp = true
		} else {
			elapsed := now.Sub(prev.SampledAt).Seconds()
			if elapsed <= 0 {
				warmingUp = true
			} else {
				usageDelta := float64(state.CPU.Usage - prev.CPUUsage)
				cpuPercentSum += usageDelta / (float64(allocatedTime) * elapsed) * 100
			}
		}

		nextSamples[instance.Name] = dashboardInstanceSample{
			CPUUsage:      state.CPU.Usage,
			AllocatedTime: allocatedTime,
			SampledAt:     now,
		}
	}

	var cpuPercent *float64
	if !warmingUp {
		value := cpuPercentSum
		cpuPercent = &value
	}

	c.mu.Lock()
	c.samples = nextSamples
	c.snapshot = DashboardLiveSnapshot{
		LiveAvailable:   true,
		LastSampleAt:    &now,
		CPUUsagePercent: cpuPercent,
		RAMUsageBytes:   totalRAM,
		DiskUsageBytes:  totalDisk,
		WarmingUp:       warmingUp,
		InstanceCount:   instanceCount,
	}
	c.lastError = nil
	c.lastRun = now
	c.mu.Unlock()

	return nil
}

func (c *DashboardCache) loop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	_ = c.Refresh(context.Background())

	for {
		select {
		case <-ticker.C:
			if err := c.Refresh(context.Background()); err != nil {
				log.Printf("[dashboard][cache] refresh failed: %v", err)
			}
		case <-c.stopCh:
			return
		}
	}
}

func stringPtr(value string) *string {
	return &value
}
