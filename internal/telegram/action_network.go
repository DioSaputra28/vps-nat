package telegram

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/activitylog"
	"github.com/DioSaputra28/vps-nat/internal/model"
	telegramservice "github.com/DioSaputra28/vps-nat/internal/telegram/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Service) DomainPreview(ctx context.Context, input DomainPreviewInput) (*DomainPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	domain, ok := telegramservice.NormalizeDomain(input.Domain)
	if !ok {
		return nil, ErrInvalidDomain
	}
	if !telegramservice.IsValidProxyMode(input.ProxyMode) || input.TargetPort < 1 || input.TargetPort > 65535 {
		return nil, ErrInvalidActionRequest
	}

	exists, err := s.repo.DomainExists(ctx, domain)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDomainAlreadyExists
	}

	expectedPublicIP := ms.Instance.MainPublicIP
	upstreamTarget := ""
	dnsReady := false
	if ms.Instance.InternalIP != nil {
		upstreamTarget = fmt.Sprintf("%s:%d", *ms.Instance.InternalIP, input.TargetPort)
	}
	if s.dnsResolver != nil && expectedPublicIP != nil {
		records, err := s.dnsResolver.LookupIPs(ctx, domain)
		if err == nil {
			for _, record := range records {
				if record == *expectedPublicIP {
					dnsReady = true
					break
				}
			}
		}
	}

	return &DomainPreviewResult{
		ContainerID:      ms.Instance.ID,
		ServiceID:        ms.Service.ID,
		Domain:           domain,
		TargetPort:       input.TargetPort,
		ProxyMode:        input.ProxyMode,
		Available:        true,
		ExpectedPublicIP: expectedPublicIP,
		DNSReady:         dnsReady,
		UpstreamTarget:   upstreamTarget,
	}, nil
}

func (s *Service) DomainSubmit(ctx context.Context, input DomainSubmitInput) (*DomainSubmitResult, error) {
	preview, err := s.DomainPreview(ctx, DomainPreviewInput(input))
	if err != nil {
		return nil, err
	}

	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}
	if !preview.DNSReady {
		return nil, ErrDomainDNSMismatch
	}
	if s.reverseProxy == nil {
		return nil, ErrReverseProxyUnavailable
	}
	if ms.Instance.InternalIP == nil {
		return nil, ErrInvalidActionRequest
	}

	now := time.Now().UTC()
	record := model.ServiceDomain{
		ID:         uuid.NewString(),
		ServiceID:  ms.Service.ID,
		Domain:     preview.Domain,
		TargetPort: preview.TargetPort,
		ProxyMode:  preview.ProxyMode,
		Status:     "pending",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(&record).Error
	}); err != nil {
		return nil, err
	}

	routeID := "service-domain-" + record.ID
	upstreamTarget := fmt.Sprintf("%s:%d", *ms.Instance.InternalIP, preview.TargetPort)
	if err := s.reverseProxy.EnsureRoute(ctx, ReverseProxyRouteInput{
		RouteID:      routeID,
		Domain:       record.Domain,
		UpstreamDial: upstreamTarget,
		ProxyMode:    record.ProxyMode,
	}); err != nil {
		failTime := time.Now().UTC()
		_ = s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.ServiceDomain{}).
				Where("id = ?", record.ID).
				Updates(map[string]any{
					"status":     "failed",
					"updated_at": failTime,
				}).Error; err != nil {
				return err
			}
			event := model.ServiceEvent{
				ID:        uuid.NewString(),
				ServiceID: ms.Service.ID,
				EventType: "domain_setup_failed",
				ActorType: actorTypeTelegramUser,
				ActorID:   &ms.User.ID,
				Summary:   "setup domain gagal",
				Payload: map[string]any{
					"domain":        record.Domain,
					"target_port":   record.TargetPort,
					"proxy_mode":    record.ProxyMode,
					"upstream":      upstreamTarget,
					"expected_ip":   preview.ExpectedPublicIP,
					"failure_reason": err.Error(),
				},
				CreatedAt: failTime,
			}
			if err := tx.Create(&event).Error; err != nil {
				return err
			}
			return activitylog.Write(ctx, tx, activitylog.Entry{
				ActorType:  actorTypeSystem,
				Action:     "domain.provision_failed",
				TargetType: "service_domain",
				TargetID:   &record.ID,
				Metadata: map[string]any{
					"service_id":      ms.Service.ID,
					"container_id":    ms.Instance.ID,
					"domain":          record.Domain,
					"proxy_mode":      record.ProxyMode,
					"target_port":     record.TargetPort,
					"upstream_target": upstreamTarget,
					"error":           err.Error(),
				},
				CreatedAt: failTime,
			})
		})
		return nil, err
	}

	successTime := time.Now().UTC()
	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ServiceDomain{}).
			Where("id = ?", record.ID).
			Updates(map[string]any{
				"status":     "active",
				"updated_at": successTime,
			}).Error; err != nil {
			return err
		}
		event := model.ServiceEvent{
			ID:        uuid.NewString(),
			ServiceID: ms.Service.ID,
			EventType: "domain_setup_completed",
			ActorType: actorTypeTelegramUser,
			ActorID:   &ms.User.ID,
			Summary:   "setup domain berhasil",
			Payload: map[string]any{
				"domain":          record.Domain,
				"target_port":     record.TargetPort,
				"proxy_mode":      record.ProxyMode,
				"expected_ip":     preview.ExpectedPublicIP,
				"upstream_target": upstreamTarget,
			},
			CreatedAt: successTime,
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		return activitylog.Write(ctx, tx, activitylog.Entry{
			ActorType:  actorTypeSystem,
			Action:     "domain.provisioned",
			TargetType: "service_domain",
			TargetID:   &record.ID,
			Metadata: map[string]any{
				"service_id":      ms.Service.ID,
				"container_id":    ms.Instance.ID,
				"domain":          record.Domain,
				"proxy_mode":      record.ProxyMode,
				"target_port":     record.TargetPort,
				"expected_ip":     ptrStringValue(preview.ExpectedPublicIP),
				"upstream_target": upstreamTarget,
			},
			CreatedAt: successTime,
		})
	}); err != nil {
		_ = s.reverseProxy.DeleteRoute(ctx, routeID, record.ProxyMode)
		return nil, err
	}
	record.Status = "active"
	record.UpdatedAt = successTime

	return &DomainSubmitResult{
		ContainerID: ms.Instance.ID,
		ServiceID:   ms.Service.ID,
		Domain:      record,
		Status:      "active",
	}, nil
}

func (s *Service) ReconfigIPPreview(ctx context.Context, input ReconfigIPPreviewInput) (*ReconfigIPPreviewResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if ms.Instance.MainPublicIP == nil || ms.Instance.InternalIP == nil {
		return nil, ErrInvalidActionRequest
	}
	currentMappings := activeMappingsSorted(ms.Service.PortMappings)
	proposed, err := s.allocateRotatedMappings(ctx, *ms.Instance.MainPublicIP, currentMappings, input.ContainerID, nil)
	if err != nil {
		return nil, err
	}
	proposedSSHPort := 0
	for _, mapping := range proposed {
		if mapping.MappingType == "ssh" {
			proposedSSHPort = mapping.PublicPort
			break
		}
	}

	return &ReconfigIPPreviewResult{
		ContainerID:      ms.Instance.ID,
		ServiceID:        ms.Service.ID,
		MainPublicIP:     ms.Instance.MainPublicIP,
		CurrentSSHPort:   ms.Instance.SSHPort,
		ProposedSSHPort:  proposedSSHPort,
		CurrentMappings:  mappingsToPreview(currentMappings),
		ProposedMappings: requestedMappingsToPreview(*ms.Instance.MainPublicIP, *ms.Instance.InternalIP, proposed),
	}, nil
}

func (s *Service) ReconfigIPSubmit(ctx context.Context, input ReconfigIPSubmitInput) (*ReconfigIPSubmitResult, error) {
	ms, err := s.loadManagedService(ctx, input.TelegramID, input.ContainerID)
	if err != nil {
		return nil, err
	}

	if ms.Instance.MainPublicIP == nil || ms.Instance.InternalIP == nil {
		return nil, ErrInvalidActionRequest
	}
	if s.networkManager == nil {
		return nil, ErrIncusUnavailable
	}
	currentMappings := activeMappingsSorted(ms.Service.PortMappings)
	if len(currentMappings) == 0 {
		return nil, ErrNoActiveSSHMapping
	}
	proposedMappings, err := s.allocateRotatedMappings(ctx, *ms.Instance.MainPublicIP, currentMappings, input.ContainerID, input.RequestedSSHPort)
	if err != nil {
		return nil, err
	}
	var newSSHPort int
	for _, mapping := range proposedMappings {
		if mapping.MappingType == "ssh" {
			newSSHPort = mapping.PublicPort
			break
		}
	}
	if newSSHPort == 0 {
		return nil, ErrNoActiveSSHMapping
	}

	currentAdapters := make([]modelServicePortMappingLike, 0, len(currentMappings))
	for i := range currentMappings {
		currentAdapters = append(currentAdapters, servicePortMappingAdapter{ServicePortMapping: currentMappings[i]})
	}
	rollback, err := s.networkManager.ReplaceServiceMappings(ctx, *ms.Instance.MainPublicIP, *ms.Instance.InternalIP, currentAdapters, proposedMappings)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	job := telegramservice.SuccessJob(ms.Service.ID, nil, "reassign_port", actorTypeTelegramUser, ms.User.ID, "", map[string]any{
		"container_id":     ms.Instance.ID,
		"old_ssh_port":     ms.Instance.SSHPort,
		"new_ssh_port":     newSSHPort,
		"main_public_ip":   ms.Instance.MainPublicIP,
		"old_mapping_ids":  activeMappingIDs(currentMappings),
		"new_mapping_count": len(proposedMappings),
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: ms.Service.ID,
		EventType: "ip_reconfigured",
		ActorType: actorTypeTelegramUser,
		ActorID:   &ms.User.ID,
		Summary:   "akses NAT utama berhasil diubah",
		Payload: map[string]any{
			"container_id":      ms.Instance.ID,
			"new_ssh_port":      newSSHPort,
			"port_mapping_count": len(proposedMappings),
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ServiceInstance{}).
			Where("id = ?", ms.Instance.ID).
			Updates(map[string]any{
				"ssh_port":   newSSHPort,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		if len(currentMappings) > 0 {
			if err := tx.Model(&model.ServicePortMapping{}).
				Where("id IN ?", activeMappingIDs(currentMappings)).
				Updates(map[string]any{
					"is_active":  false,
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
		}

		newRows := make([]model.ServicePortMapping, 0, len(proposedMappings))
		for _, mapping := range proposedMappings {
			targetIP := *ms.Instance.InternalIP
			description := mapping.Description
			if strings.TrimSpace(description) == "" {
				description = mapping.MappingType
			}
			newRows = append(newRows, model.ServicePortMapping{
				ID:          uuid.NewString(),
				ServiceID:   ms.Service.ID,
				MappingType: mapping.MappingType,
				PublicIP:    *ms.Instance.MainPublicIP,
				PublicPort:  mapping.PublicPort,
				Protocol:    mapping.Protocol,
				TargetIP:    targetIP,
				TargetPort:  mapping.TargetPort,
				Description: strPtr(description),
				IsActive:    true,
				CreatedAt:   now,
				UpdatedAt:   now,
			})
		}
		if len(newRows) > 0 {
			if err := tx.Create(&newRows).Error; err != nil {
				return err
			}
		}

		if err := tx.Create(job).Error; err != nil {
			return err
		}
		if err := tx.Create(event).Error; err != nil {
			return err
		}
		return activitylog.Write(ctx, tx, activitylog.Entry{
			ActorType:  actorTypeTelegramUser,
			ActorID:    &ms.User.ID,
			Action:     "port_mapping.reassigned",
			TargetType: "service",
			TargetID:   &ms.Service.ID,
			Metadata: map[string]any{
				"container_id":       ms.Instance.ID,
				"old_ssh_port":       derefInt(ms.Instance.SSHPort),
				"new_ssh_port":       newSSHPort,
				"mapping_count":      len(proposedMappings),
				"main_public_ip":     ptrStringValue(ms.Instance.MainPublicIP),
				"current_mappings":   mappingsToPreview(currentMappings),
				"proposed_mappings":  requestedMappingsToPreview(*ms.Instance.MainPublicIP, *ms.Instance.InternalIP, proposedMappings),
			},
			CreatedAt: now,
		})
	}); err != nil {
		if rollback != nil {
			_ = rollback(ctx)
		}
		return nil, err
	}

	return &ReconfigIPSubmitResult{
		ActionAcceptedResult: ActionAcceptedResult{
			ContainerID: ms.Instance.ID,
			ServiceID:   ms.Service.ID,
			Action:      "reconfig_ip",
			Accepted:    true,
			Status:      "completed",
		},
		MainPublicIP: ms.Instance.MainPublicIP,
		SSHPort:      &newSSHPort,
		PortMappings: requestedMappingsToPreview(*ms.Instance.MainPublicIP, *ms.Instance.InternalIP, proposedMappings),
	}, nil
}

func activeMappingsSorted(mappings []model.ServicePortMapping) []model.ServicePortMapping {
	active := make([]model.ServicePortMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.IsActive {
			active = append(active, mapping)
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].MappingType != active[j].MappingType {
			if active[i].MappingType == "ssh" {
				return true
			}
			if active[j].MappingType == "ssh" {
				return false
			}
		}
		if active[i].Protocol != active[j].Protocol {
			return active[i].Protocol < active[j].Protocol
		}
		return active[i].TargetPort < active[j].TargetPort
	})
	return active
}

func (s *Service) allocateRotatedMappings(ctx context.Context, publicIP string, currentMappings []model.ServicePortMapping, containerID string, requestedSSHPort *int) ([]RequestedPortMapping, error) {
	used := map[string]map[int]struct{}{
		"tcp": {},
		"udp": {},
	}
	result := make([]RequestedPortMapping, 0, len(currentMappings))

	for _, current := range currentMappings {
		nextPort := 0
		if current.MappingType == "ssh" && requestedSSHPort != nil {
			if *requestedSSHPort < 1 || *requestedSSHPort > 65535 {
				return nil, ErrInvalidActionRequest
			}
			available, err := s.repo.IsPublicPortAvailable(ctx, publicIP, current.Protocol, *requestedSSHPort)
			if err != nil {
				return nil, err
			}
			if !available {
				return nil, ErrActionConflict
			}
			nextPort = *requestedSSHPort
		} else {
			port, err := s.repo.FindNextAvailablePublicPort(ctx, publicIP, current.Protocol, used[current.Protocol])
			if err != nil {
				return nil, err
			}
			nextPort = port
		}

		used[current.Protocol][nextPort] = struct{}{}
		description := current.MappingType
		if current.Description != nil && strings.TrimSpace(*current.Description) != "" {
			description = strings.TrimSpace(*current.Description)
		}
		result = append(result, RequestedPortMapping{
			MappingType: current.MappingType,
			Protocol:    current.Protocol,
			TargetPort:  current.TargetPort,
			PublicPort:  nextPort,
			Description: description,
		})
	}

	return result, nil
}

func mappingsToPreview(mappings []model.ServicePortMapping) []ReconfigPortMapping {
	result := make([]ReconfigPortMapping, 0, len(mappings))
	for _, mapping := range mappings {
		result = append(result, ReconfigPortMapping{
			MappingType: mapping.MappingType,
			PublicIP:    mapping.PublicIP,
			PublicPort:  mapping.PublicPort,
			Protocol:    mapping.Protocol,
			TargetIP:    mapping.TargetIP,
			TargetPort:  mapping.TargetPort,
			Description: mapping.Description,
			IsActive:    mapping.IsActive,
		})
	}
	return result
}

func requestedMappingsToPreview(publicIP string, privateIP string, mappings []RequestedPortMapping) []ReconfigPortMapping {
	result := make([]ReconfigPortMapping, 0, len(mappings))
	for _, mapping := range mappings {
		description := mapping.Description
		result = append(result, ReconfigPortMapping{
			MappingType: mapping.MappingType,
			PublicIP:    publicIP,
			PublicPort:  mapping.PublicPort,
			Protocol:    mapping.Protocol,
			TargetIP:    privateIP,
			TargetPort:  mapping.TargetPort,
			Description: strPtr(description),
			IsActive:    true,
		})
	}
	return result
}

func activeMappingIDs(mappings []model.ServicePortMapping) []string {
	ids := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.IsActive {
			ids = append(ids, mapping.ID)
		}
	}
	return ids
}
