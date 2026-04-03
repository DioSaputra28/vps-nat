package incus

import (
	"fmt"
	"os"

	"github.com/DioSaputra28/vps-nat/internal/config"
	incusclient "github.com/lxc/incus/v6/client"
)

type Client struct {
	server incusclient.InstanceServer
	mode   string
}

func New(cfg config.IncusConfig) (*Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	args, err := connectionArgs(cfg)
	if err != nil {
		return nil, err
	}

	var server incusclient.InstanceServer

	switch cfg.Mode {
	case "unix":
		server, err = incusclient.ConnectIncusUnix(cfg.Socket, args)
	case "remote":
		server, err = incusclient.ConnectIncus(cfg.RemoteAddr, args)
	default:
		return nil, fmt.Errorf("unsupported incus mode %q", cfg.Mode)
	}
	if err != nil {
		return nil, fmt.Errorf("connect to incus: %w", err)
	}

	return &Client{
		server: server,
		mode:   cfg.Mode,
	}, nil
}

func (c *Client) Server() incusclient.InstanceServer {
	if c == nil {
		return nil
	}

	return c.server
}

func (c *Client) Mode() string {
	if c == nil {
		return ""
	}

	return c.mode
}

func connectionArgs(cfg config.IncusConfig) (*incusclient.ConnectionArgs, error) {
	args := &incusclient.ConnectionArgs{
		UserAgent:          cfg.UserAgent,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	if cfg.Mode != "remote" {
		return args, nil
	}

	clientCert, err := readOptionalFile(cfg.TLSClientCertPath)
	if err != nil {
		return nil, fmt.Errorf("read INCUS_TLS_CLIENT_CERT_PATH: %w", err)
	}

	clientKey, err := readOptionalFile(cfg.TLSClientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read INCUS_TLS_CLIENT_KEY_PATH: %w", err)
	}

	ca, err := readOptionalFile(cfg.TLSCAPath)
	if err != nil {
		return nil, fmt.Errorf("read INCUS_TLS_CA_PATH: %w", err)
	}

	serverCert, err := readOptionalFile(cfg.TLSServerCertPath)
	if err != nil {
		return nil, fmt.Errorf("read INCUS_TLS_SERVER_CERT_PATH: %w", err)
	}

	args.TLSClientCert = clientCert
	args.TLSClientKey = clientKey
	args.TLSCA = ca
	args.TLSServerCert = serverCert

	return args, nil
}

func readOptionalFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}
