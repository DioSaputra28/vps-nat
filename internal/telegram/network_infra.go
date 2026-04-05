package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/incus"
	"github.com/DioSaputra28/vps-nat/internal/model"
	"github.com/lxc/incus/v6/shared/api"
)

type DNSResolver interface {
	LookupIPs(ctx context.Context, host string) ([]string, error)
}

type ReverseProxyClient interface {
	EnsureRoute(ctx context.Context, input ReverseProxyRouteInput) error
	DeleteRoute(ctx context.Context, routeID string, proxyMode string) error
}

type NetworkForwardManager interface {
	ReplaceServiceMappings(ctx context.Context, publicIP string, privateIP string, current []modelServicePortMappingLike, next []RequestedPortMapping) (func(context.Context) error, error)
}

type ReverseProxyRouteInput struct {
	RouteID      string
	Domain       string
	UpstreamDial string
	ProxyMode    string
}

type modelServicePortMappingLike interface {
	GetMappingType() string
	GetProtocol() string
	GetPublicPort() int
	GetTargetPort() int
	GetDescription() *string
}

type netDNSResolver struct {
	resolver *net.Resolver
}

func NewNetDNSResolver() DNSResolver {
	return &netDNSResolver{resolver: net.DefaultResolver}
}

func (r *netDNSResolver) LookupIPs(ctx context.Context, host string) ([]string, error) {
	if r == nil || r.resolver == nil {
		return nil, errors.New("dns resolver is unavailable")
	}

	addrs, err := r.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		result = append(result, addr.IP.String())
	}
	return result, nil
}

type caddyReverseProxyClient struct {
	baseURL string
	token   string
	client  *http.Client
}

const (
	caddyHTTPServerID  = "vps-nat-http"
	caddyHTTPSServerID = "vps-nat-https"
)

func NewCaddyReverseProxyClient(baseURL string, token string) ReverseProxyClient {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return nil
	}

	return &caddyReverseProxyClient{
		baseURL: strings.TrimRight(trimmed, "/"),
		token:   strings.TrimSpace(token),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *caddyReverseProxyClient) EnsureRoute(ctx context.Context, input ReverseProxyRouteInput) error {
	if c == nil {
		return ErrReverseProxyUnavailable
	}

	httpRoute := caddyProxyRoute(input.RouteID+"-http", input.Domain, input.UpstreamDial)
	httpsRoute := caddyProxyRoute(input.RouteID+"-https", input.Domain, input.UpstreamDial)

	switch strings.TrimSpace(input.ProxyMode) {
	case "http":
		if err := c.upsertServerRoute(ctx, caddyHTTPServerID, caddyHTTPServer(false), httpRoute); err != nil {
			return err
		}
		return c.deleteRouteFromServer(ctx, caddyHTTPSServerID, input.RouteID+"-https")
	case "https":
		if err := c.upsertServerRoute(ctx, caddyHTTPSServerID, caddyHTTPSServer(), httpsRoute); err != nil {
			return err
		}
		return c.deleteRouteFromServer(ctx, caddyHTTPServerID, input.RouteID+"-http")
	case "http_and_https":
		if err := c.upsertServerRoute(ctx, caddyHTTPServerID, caddyHTTPServer(false), httpRoute); err != nil {
			return err
		}
		return c.upsertServerRoute(ctx, caddyHTTPSServerID, caddyHTTPSServer(), httpsRoute)
	default:
		return ErrInvalidActionRequest
	}
}

func (c *caddyReverseProxyClient) DeleteRoute(ctx context.Context, routeID string, proxyMode string) error {
	if c == nil {
		return ErrReverseProxyUnavailable
	}

	httpErr := c.deleteRouteFromServer(ctx, caddyHTTPServerID, routeID+"-http")
	httpsErr := c.deleteRouteFromServer(ctx, caddyHTTPSServerID, routeID+"-https")
	if httpErr != nil && httpsErr != nil {
		return httpErr
	}
	return nil
}

type caddyHTTPServerConfig struct {
	Listen []string          `json:"listen"`
	Routes []caddyRouteEntry `json:"routes"`
}

type caddyRouteEntry struct {
	ID      string                 `json:"@id,omitempty"`
	Match   []map[string][]string  `json:"match,omitempty"`
	Handle  []map[string]any       `json:"handle"`
	Terminal bool                  `json:"terminal,omitempty"`
}

func caddyHTTPServer(_ bool) caddyHTTPServerConfig {
	return caddyHTTPServerConfig{
		Listen: []string{":80"},
		Routes: []caddyRouteEntry{},
	}
}

func caddyHTTPSServer() caddyHTTPServerConfig {
	return caddyHTTPServerConfig{
		Listen: []string{":443"},
		Routes: []caddyRouteEntry{},
	}
}

func caddyProxyRoute(id string, domain string, upstreamDial string) caddyRouteEntry {
	return caddyRouteEntry{
		ID: id,
		Match: []map[string][]string{
			{"host": {domain}},
		},
		Handle: []map[string]any{
			{
				"handler": "reverse_proxy",
				"upstreams": []map[string]string{
					{"dial": upstreamDial},
				},
			},
		},
		Terminal: true,
	}
}

func (c *caddyReverseProxyClient) upsertServerRoute(ctx context.Context, serverID string, fallback caddyHTTPServerConfig, route caddyRouteEntry) error {
	server, exists, err := c.getServer(ctx, serverID)
	if err != nil {
		return err
	}
	if !exists {
		server = fallback
	}

	replaced := false
	for i := range server.Routes {
		if server.Routes[i].ID == route.ID {
			server.Routes[i] = route
			replaced = true
			break
		}
	}
	if !replaced {
		server.Routes = append(server.Routes, route)
	}

	sort.SliceStable(server.Routes, func(i, j int) bool {
		return server.Routes[i].ID < server.Routes[j].ID
	})

	return c.putServer(ctx, serverID, server)
}

func (c *caddyReverseProxyClient) deleteRouteFromServer(ctx context.Context, serverID string, routeID string) error {
	server, exists, err := c.getServer(ctx, serverID)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	filtered := make([]caddyRouteEntry, 0, len(server.Routes))
	for _, route := range server.Routes {
		if route.ID == routeID {
			continue
		}
		filtered = append(filtered, route)
	}
	server.Routes = filtered

	return c.putServer(ctx, serverID, server)
}

func (c *caddyReverseProxyClient) getServer(ctx context.Context, serverID string) (caddyHTTPServerConfig, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/config/apps/http/servers/"+serverID, nil)
	if err != nil {
		return caddyHTTPServerConfig{}, false, err
	}
	c.decorateRequest(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return caddyHTTPServerConfig{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return caddyHTTPServerConfig{}, false, nil
	}
	if resp.StatusCode >= 300 {
		return caddyHTTPServerConfig{}, false, fmt.Errorf("caddy get server failed with status %d", resp.StatusCode)
	}

	var server caddyHTTPServerConfig
	if err := json.NewDecoder(resp.Body).Decode(&server); err != nil {
		return caddyHTTPServerConfig{}, false, err
	}
	return server, true, nil
}

func (c *caddyReverseProxyClient) putServer(ctx context.Context, serverID string, server caddyHTTPServerConfig) error {
	body, err := json.Marshal(server)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/config/apps/http/servers/"+serverID, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.decorateRequest(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("caddy put server failed with status %d", resp.StatusCode)
	}
	return nil
}

func (c *caddyReverseProxyClient) decorateRequest(req *http.Request) {
	if strings.TrimSpace(c.token) != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

type incusNetworkForwardManager struct {
	client      *incus.Client
	networkName string
}

func NewIncusNetworkForwardManager(client *incus.Client, networkName string) NetworkForwardManager {
	if client == nil || client.Server() == nil || strings.TrimSpace(networkName) == "" {
		return nil
	}

	return &incusNetworkForwardManager{
		client:      client,
		networkName: strings.TrimSpace(networkName),
	}
}

func (m *incusNetworkForwardManager) ReplaceServiceMappings(ctx context.Context, publicIP string, privateIP string, current []modelServicePortMappingLike, next []RequestedPortMapping) (func(context.Context) error, error) {
	server := m.client.Server()
	forward, etag, err := server.GetNetworkForward(m.networkName, publicIP)
	notFound := err != nil && api.StatusErrorCheck(err, http.StatusNotFound)
	if err != nil && !notFound {
		return nil, err
	}

	originalExists := !notFound
	var original api.NetworkForward
	var originalETag string
	if originalExists && forward != nil {
		original = *forward
		originalETag = etag
	}

	var remaining []api.NetworkForwardPort
	if originalExists && forward != nil {
		remaining = append(remaining, forward.Ports...)
	}

currentLoop:
	for _, existing := range remaining {
		for _, cur := range current {
			if existing.Protocol == cur.GetProtocol() && existing.ListenPort == strconv.Itoa(cur.GetPublicPort()) {
				continue currentLoop
			}
		}
	}

	filtered := make([]api.NetworkForwardPort, 0, len(remaining)+len(next))
	for _, existing := range remaining {
		shouldSkip := false
		for _, cur := range current {
			if existing.Protocol == cur.GetProtocol() && existing.ListenPort == strconv.Itoa(cur.GetPublicPort()) {
				shouldSkip = true
				break
			}
		}
		if !shouldSkip {
			filtered = append(filtered, existing)
		}
	}
	for _, mapping := range next {
		filtered = append(filtered, api.NetworkForwardPort{
			Description:   mapping.Description,
			Protocol:      mapping.Protocol,
			ListenPort:    strconv.Itoa(mapping.PublicPort),
			TargetPort:    strconv.Itoa(mapping.TargetPort),
			TargetAddress: privateIP,
		})
	}

	if originalExists {
		if err := server.UpdateNetworkForward(m.networkName, publicIP, api.NetworkForwardPut{
			Config:      forward.Config,
			Description: forward.Description,
			Ports:       filtered,
		}, etag); err != nil {
			return nil, err
		}
	} else {
		if err := server.CreateNetworkForward(m.networkName, api.NetworkForwardsPost{
			ListenAddress: publicIP,
			NetworkForwardPut: api.NetworkForwardPut{
				Ports: filtered,
			},
		}); err != nil {
			return nil, err
		}
	}

	return func(ctx context.Context) error {
		if originalExists {
			return server.UpdateNetworkForward(m.networkName, publicIP, original.Writable(), originalETag)
		}
		return server.DeleteNetworkForward(m.networkName, publicIP)
	}, nil
}

type servicePortMappingAdapter struct {
	model.ServicePortMapping
}

func (a servicePortMappingAdapter) GetMappingType() string { return a.MappingType }
func (a servicePortMappingAdapter) GetProtocol() string    { return a.Protocol }
func (a servicePortMappingAdapter) GetPublicPort() int     { return a.PublicPort }
func (a servicePortMappingAdapter) GetTargetPort() int     { return a.TargetPort }
func (a servicePortMappingAdapter) GetDescription() *string {
	return a.Description
}
