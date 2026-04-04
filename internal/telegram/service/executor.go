package service

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/DioSaputra28/vps-nat/internal/incus"
	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

const DefaultSSHUsername = "root"
const simplestreamsImagesServer = "https://images.linuxcontainers.org"

type ActionExecutor interface {
	ChangeState(instanceName string, action string) (string, error)
	ChangePassword(instanceName string, password string) (string, error)
	ResetSSH(instanceName string) (string, string, error)
	Reinstall(instanceName string, imageAlias string) (string, error)
	ApplyResourceLimits(instanceName string, cpu int, ramMB int, diskGB int) (string, error)
	DeleteInstance(instanceName string) (string, error)
}

type IncusUnavailableError struct{}

func (IncusUnavailableError) Error() string {
	return "incus unavailable"
}

type incusActionExecutor struct {
	client *incus.Client
}

type instanceDeletionServer interface {
	GetInstance(name string) (*api.Instance, string, error)
	UpdateInstanceState(name string, state api.InstanceStatePut, etag string) (incusclient.Operation, error)
	DeleteInstance(name string) (incusclient.Operation, error)
}

func NewActionExecutor(client *incus.Client) ActionExecutor {
	if client == nil {
		return nil
	}
	return &incusActionExecutor{client: client}
}

func (e *incusActionExecutor) ChangeState(instanceName string, action string) (string, error) {
	server, err := e.server()
	if err != nil {
		return "", err
	}

	op, err := server.UpdateInstanceState(instanceName, api.InstanceStatePut{Action: action}, "")
	if err != nil {
		return "", err
	}
	if err := op.Wait(); err != nil {
		return "", err
	}
	return op.Get().ID, nil
}

func (e *incusActionExecutor) ChangePassword(instanceName string, password string) (string, error) {
	server, err := e.server()
	if err != nil {
		return "", err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	op, err := server.ExecInstance(instanceName, passwordExecPost(password), &incusclient.InstanceExecArgs{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", err
	}
	if err := op.Wait(); err != nil {
		return "", err
	}

	return op.Get().ID, nil
}

func (e *incusActionExecutor) ResetSSH(instanceName string) (string, string, error) {
	password := randomPassword(18)
	opID, err := e.ChangePassword(instanceName, password)
	if err != nil {
		return "", "", err
	}

	return opID, password, nil
}

func (e *incusActionExecutor) Reinstall(instanceName string, imageAlias string) (string, error) {
	server, err := e.server()
	if err != nil {
		return "", err
	}

	source, err := instanceSourceFromImageAlias(imageAlias)
	if err != nil {
		return "", err
	}

	op, err := server.RebuildInstance(instanceName, api.InstanceRebuildPost{
		Source: source,
	})
	if err != nil {
		return "", err
	}
	if err := op.Wait(); err != nil {
		return "", err
	}

	return op.Get().ID, nil
}

func (e *incusActionExecutor) ApplyResourceLimits(instanceName string, cpu int, ramMB int, diskGB int) (string, error) {
	server, err := e.server()
	if err != nil {
		return "", err
	}

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

func (e *incusActionExecutor) DeleteInstance(instanceName string) (string, error) {
	server, err := e.server()
	if err != nil {
		return "", err
	}

	return deleteInstanceWithStopIfNeeded(server, instanceName)
}

func (e *incusActionExecutor) server() (incusclient.InstanceServer, error) {
	if e == nil || e.client == nil || e.client.Server() == nil {
		return nil, IncusUnavailableError{}
	}
	return e.client.Server(), nil
}

func randomPassword(length int) string {
	const chars = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%^&*"
	var builder strings.Builder
	for i := 0; i < length; i++ {
		builder.WriteByte(chars[rand.IntN(len(chars))])
	}
	return builder.String()
}

func passwordExecPost(password string) api.InstanceExecPost {
	command := []string{"sh", "-lc", fmt.Sprintf("echo %s:%s | chpasswd", shellQuote(DefaultSSHUsername), shellQuote(password))}
	return api.InstanceExecPost{
		Command:      command,
		Interactive:  false,
		RecordOutput: false,
	}
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

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func deleteInstanceWithStopIfNeeded(server instanceDeletionServer, instanceName string) (string, error) {
	instance, _, err := server.GetInstance(instanceName)
	if err != nil {
		return "", err
	}

	if instance != nil && strings.EqualFold(instance.Status, "running") {
		stopOp, err := server.UpdateInstanceState(instanceName, api.InstanceStatePut{
			Action: "stop",
			Force:  true,
		}, "")
		if err != nil {
			return "", err
		}
		if err := stopOp.Wait(); err != nil {
			return "", err
		}
	}

	deleteOp, err := server.DeleteInstance(instanceName)
	if err != nil {
		return "", err
	}
	if err := deleteOp.Wait(); err != nil {
		return "", err
	}

	return deleteOp.Get().ID, nil
}
