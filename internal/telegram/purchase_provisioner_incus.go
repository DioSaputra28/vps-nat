package telegram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/incus"
	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

const simplestreamsImagesServer = "https://images.linuxcontainers.org"

type incusPurchaseProvisioner struct {
	client      *incus.Client
	networkName string
}

func NewIncusPurchaseProvisioner(client *incus.Client, networkName string) PurchaseProvisioner {
	if client == nil || strings.TrimSpace(networkName) == "" {
		return nil
	}

	return &incusPurchaseProvisioner{
		client:      client,
		networkName: strings.TrimSpace(networkName),
	}
}

func (p *incusPurchaseProvisioner) HostnameExists(ctx context.Context, hostname string) (bool, error) {
	server, err := p.server()
	if err != nil {
		return false, err
	}

	_, _, err = server.GetInstance(hostname)
	if err == nil {
		return true, nil
	}
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return false, nil
	}
	return false, err
}

func (p *incusPurchaseProvisioner) Provision(ctx context.Context, req PurchaseProvisionRequest) (*PurchaseProvisionResult, error) {
	server, err := p.server()
	if err != nil {
		return nil, err
	}

	source, err := instanceSourceFromImageAlias(req.ImageAlias)
	if err != nil {
		return nil, err
	}

	log.Printf("[purchase][provision][hostname=%s] create instance started image_alias=%q source_alias=%q source_server=%q source_protocol=%q", req.Hostname, req.ImageAlias, source.Alias, source.Server, source.Protocol)
	instanceReq := api.InstancesPost{
		Name:   req.Hostname,
		Type:   api.InstanceTypeContainer,
		Source: source,
	}

	op, err := server.CreateInstance(instanceReq)
	if err != nil {
		log.Printf("[purchase][provision][hostname=%s] create instance request failed: %v", req.Hostname, err)
		return nil, err
	}
	if err := op.Wait(); err != nil {
		log.Printf("[purchase][provision][hostname=%s] create instance operation failed operation_id=%s: %v", req.Hostname, op.Get().ID, err)
		return nil, err
	}
	log.Printf("[purchase][provision][hostname=%s] create instance completed operation_id=%s", req.Hostname, op.Get().ID)

	var createdForward bool
	cleanup := func() {
		log.Printf("[purchase][provision][hostname=%s] cleanup started created_forward=%t", req.Hostname, createdForward)
		if createdForward {
			if err := p.removeForwardPorts(req.Node.PublicIP, req.PortMappings); err != nil {
				log.Printf("[purchase][provision][hostname=%s] cleanup remove forward ports failed: %v", req.Hostname, err)
			}
		}
		if _, err := p.deleteInstance(req.Hostname); err != nil {
			log.Printf("[purchase][provision][hostname=%s] cleanup delete instance failed: %v", req.Hostname, err)
		}
		log.Printf("[purchase][provision][hostname=%s] cleanup finished", req.Hostname)
	}

	updateOpID, err := p.applyResourceLimits(server, req.Hostname, req.Package.CPU, req.Package.RAMMB, req.Package.DiskGB)
	if err != nil {
		log.Printf("[purchase][provision][hostname=%s] apply resource limits failed: %v", req.Hostname, err)
		cleanup()
		return nil, err
	}
	log.Printf("[purchase][provision][hostname=%s] apply resource limits completed operation_id=%s", req.Hostname, updateOpID)

	startOp, err := server.UpdateInstanceState(req.Hostname, api.InstanceStatePut{Action: "start", Timeout: -1}, "")
	if err != nil {
		log.Printf("[purchase][provision][hostname=%s] start instance request failed: %v", req.Hostname, err)
		cleanup()
		return nil, err
	}
	if err := startOp.Wait(); err != nil {
		log.Printf("[purchase][provision][hostname=%s] start instance operation failed operation_id=%s: %v", req.Hostname, startOp.Get().ID, err)
		cleanup()
		return nil, err
	}
	log.Printf("[purchase][provision][hostname=%s] start instance completed operation_id=%s", req.Hostname, startOp.Get().ID)

	privateIP, osFamily, err := p.fetchInstanceState(server, req.Hostname)
	if err != nil {
		log.Printf("[purchase][provision][hostname=%s] fetch instance state failed: %v", req.Hostname, err)
		cleanup()
		return nil, err
	}
	log.Printf("[purchase][provision][hostname=%s] instance state fetched private_ip=%s os_family=%v", req.Hostname, privateIP, osFamily)

	password := randomPassword(18)
	passwordOpID, err := p.execPassword(server, req.Hostname, password)
	if err != nil {
		log.Printf("[purchase][provision][hostname=%s] set root password failed: %v", req.Hostname, err)
		cleanup()
		return nil, err
	}
	log.Printf("[purchase][provision][hostname=%s] set root password completed operation_id=%s", req.Hostname, passwordOpID)

	if err := p.ensureForwardPorts(server, req.Node.PublicIP, privateIP, req.PortMappings); err != nil {
		log.Printf("[purchase][provision][hostname=%s] ensure forward ports failed: %v", req.Hostname, err)
		cleanup()
		return nil, err
	}
	createdForward = true
	log.Printf("[purchase][provision][hostname=%s] ensure forward ports completed mapping_count=%d", req.Hostname, len(req.PortMappings))

	mappings := make([]ProvisionedPortMapping, 0, len(req.PortMappings))
	for _, mapping := range req.PortMappings {
		mappings = append(mappings, ProvisionedPortMapping{
			MappingType: mapping.MappingType,
			PublicIP:    req.Node.PublicIP,
			PublicPort:  mapping.PublicPort,
			Protocol:    mapping.Protocol,
			TargetIP:    privateIP,
			TargetPort:  mapping.TargetPort,
		})
	}

	return &PurchaseProvisionResult{
		OperationID:  updateOpID + "," + startOp.Get().ID + "," + passwordOpID,
		Hostname:     req.Hostname,
		PublicIP:     req.Node.PublicIP,
		PrivateIP:    privateIP,
		SSHUsername:  "root",
		SSHPassword:  password,
		SSHPort:      req.PortMappings[0].PublicPort,
		OSFamily:     osFamily,
		PortMappings: mappings,
	}, nil
}

func instanceSourceFromImageAlias(imageAlias string) (api.InstanceSource, error) {
	trimmed := strings.TrimSpace(imageAlias)
	if trimmed == "" {
		return api.InstanceSource{}, errors.New("image alias is required")
	}

	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) == 1 {
		return api.InstanceSource{
			Type:  "image",
			Alias: trimmed,
		}, nil
	}

	remote := strings.TrimSpace(parts[0])
	alias := strings.TrimSpace(parts[1])
	if alias == "" {
		return api.InstanceSource{}, fmt.Errorf("invalid image alias %q", imageAlias)
	}

	switch remote {
	case "images":
		return api.InstanceSource{
			Type:     "image",
			Alias:    alias,
			Server:   simplestreamsImagesServer,
			Protocol: "simplestreams",
		}, nil
	default:
		return api.InstanceSource{}, fmt.Errorf("unsupported image remote %q", remote)
	}
}

func (p *incusPurchaseProvisioner) server() (incusclient.InstanceServer, error) {
	if p == nil || p.client == nil || p.client.Server() == nil {
		return nil, ErrIncusUnavailable
	}
	return p.client.Server(), nil
}

func (p *incusPurchaseProvisioner) applyResourceLimits(server incusclient.InstanceServer, instanceName string, cpu int, ramMB int, diskGB int) (string, error) {
	instance, etag, err := server.GetInstance(instanceName)
	if err != nil {
		return "", err
	}

	writable := instance.Writable()
	if writable.Config == nil {
		writable.Config = api.ConfigMap{}
	}
	writable.Config["limits.cpu"] = strconv.Itoa(cpu)
	writable.Config["limits.memory"] = fmt.Sprintf("%dMiB", ramMB)
	for name, device := range writable.Devices {
		if device["type"] == "disk" && device["path"] == "/" {
			device["size"] = fmt.Sprintf("%dGiB", diskGB)
			writable.Devices[name] = device
			break
		}
	}

	op, err := server.UpdateInstance(instanceName, writable, etag)
	if err != nil {
		return "", err
	}
	if err := op.Wait(); err != nil {
		return "", err
	}
	return op.Get().ID, nil
}

func (p *incusPurchaseProvisioner) fetchInstanceState(server incusclient.InstanceServer, instanceName string) (string, *string, error) {
	state, privateIP, err := waitForPrivateIPv4(func() (*api.InstanceState, error) {
		current, _, err := server.GetInstanceState(instanceName)
		return current, err
	}, 10, 2*time.Second)
	if err != nil {
		return "", nil, err
	}

	var osFamily *string
	if state.OSInfo != nil && state.OSInfo.OS != "" {
		name := state.OSInfo.OS
		osFamily = &name
	}

	return privateIP, osFamily, nil
}

func waitForPrivateIPv4(fetch func() (*api.InstanceState, error), attempts int, delay time.Duration) (*api.InstanceState, string, error) {
	if attempts <= 0 {
		attempts = 1
	}

	var lastState *api.InstanceState
	for attempt := 1; attempt <= attempts; attempt++ {
		state, err := fetch()
		if err != nil {
			return nil, "", err
		}
		lastState = state

		privateIP := privateIPv4FromState(state)
		if privateIP != "" {
			return state, privateIP, nil
		}

		if attempt < attempts && delay > 0 {
			time.Sleep(delay)
		}
	}

	return lastState, "", fmt.Errorf("instance private ip not found")
}

func privateIPv4FromState(state *api.InstanceState) string {
	if state == nil {
		return ""
	}

	for _, network := range state.Network {
		for _, address := range network.Addresses {
			if address.Family == "inet" && strings.EqualFold(address.Scope, "global") {
				return address.Address
			}
		}
	}

	return ""
}

func (p *incusPurchaseProvisioner) execPassword(server incusclient.InstanceServer, instanceName string, password string) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	post := passwordExecPost(password)
	op, err := server.ExecInstance(instanceName, post, &incusclient.InstanceExecArgs{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", err
	}
	if err := op.Wait(); err != nil {
		log.Printf("[purchase][provision][hostname=%s] set root password operation failed stdout=%q stderr=%q", instanceName, stdout.String(), stderr.String())
		return "", err
	}
	return op.Get().ID, nil
}

func passwordExecPost(password string) api.InstanceExecPost {
	command := []string{"sh", "-lc", fmt.Sprintf("echo %s:%s | chpasswd", shellQuote("root"), shellQuote(password))}
	return api.InstanceExecPost{
		Command:      command,
		Interactive:  false,
		RecordOutput: false,
	}
}

func (p *incusPurchaseProvisioner) ensureForwardPorts(server incusclient.InstanceServer, publicIP string, privateIP string, mappings []RequestedPortMapping) error {
	forward, etag, err := server.GetNetworkForward(p.networkName, publicIP)
	if err != nil && !api.StatusErrorCheck(err, http.StatusNotFound) {
		return err
	}

	ports := make([]api.NetworkForwardPort, 0, len(mappings))
	if err == nil && forward != nil {
		ports = append(ports, forward.Ports...)
	}

	for _, mapping := range mappings {
		ports = append(ports, api.NetworkForwardPort{
			Description:   mapping.Description,
			Protocol:      mapping.Protocol,
			ListenPort:    strconv.Itoa(mapping.PublicPort),
			TargetPort:    strconv.Itoa(mapping.TargetPort),
			TargetAddress: privateIP,
		})
	}

	if err != nil && api.StatusErrorCheck(err, http.StatusNotFound) {
		return server.CreateNetworkForward(p.networkName, api.NetworkForwardsPost{
			ListenAddress: publicIP,
			NetworkForwardPut: api.NetworkForwardPut{
				Ports: ports,
			},
		})
	}

	return server.UpdateNetworkForward(p.networkName, publicIP, api.NetworkForwardPut{
		Config:      forward.Config,
		Description: forward.Description,
		Ports:       ports,
	}, etag)
}

func (p *incusPurchaseProvisioner) removeForwardPorts(publicIP string, mappings []RequestedPortMapping) error {
	server, err := p.server()
	if err != nil {
		return err
	}

	forward, etag, err := server.GetNetworkForward(p.networkName, publicIP)
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return nil
		}
		return err
	}

	remaining := make([]api.NetworkForwardPort, 0, len(forward.Ports))
outer:
	for _, existing := range forward.Ports {
		for _, mapping := range mappings {
			if existing.Protocol == mapping.Protocol && existing.ListenPort == strconv.Itoa(mapping.PublicPort) {
				continue outer
			}
		}
		remaining = append(remaining, existing)
	}

	if len(remaining) == 0 {
		return server.DeleteNetworkForward(p.networkName, publicIP)
	}

	return server.UpdateNetworkForward(p.networkName, publicIP, api.NetworkForwardPut{
		Config:      forward.Config,
		Description: forward.Description,
		Ports:       remaining,
	}, etag)
}

func (p *incusPurchaseProvisioner) deleteInstance(instanceName string) (string, error) {
	server, err := p.server()
	if err != nil {
		return "", err
	}

	state, _, err := server.GetInstanceState(instanceName)
	if err == nil && strings.EqualFold(state.Status, "Running") {
		stopOp, stopErr := server.UpdateInstanceState(instanceName, api.InstanceStatePut{
			Action:  "stop",
			Timeout: -1,
			Force:   true,
		}, "")
		if stopErr != nil {
			return "", stopErr
		}
		if stopErr = stopOp.Wait(); stopErr != nil {
			return "", stopErr
		}
	}
	if err != nil && !api.StatusErrorCheck(err, http.StatusNotFound) {
		return "", err
	}

	op, err := server.DeleteInstance(instanceName)
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return "", nil
		}
		return "", err
	}
	if err := op.Wait(); err != nil {
		return "", err
	}
	return op.Get().ID, nil
}

func randomPassword(length int) string {
	const chars = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%^&*"
	var builder strings.Builder
	for i := 0; i < length; i++ {
		builder.WriteByte(chars[rand.IntN(len(chars))])
	}
	return builder.String()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
