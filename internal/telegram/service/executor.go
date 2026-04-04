package service

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/DioSaputra28/vps-nat/internal/incus"
	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

const DefaultSSHUsername = "root"

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
	command := []string{"sh", "-lc", fmt.Sprintf("echo %s:%s | chpasswd", shellQuote(DefaultSSHUsername), shellQuote(password))}
	op, err := server.ExecInstance(instanceName, api.InstanceExecPost{
		Command:      command,
		Interactive:  false,
		RecordOutput: true,
	}, &incusclient.InstanceExecArgs{
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

	op, err := server.RebuildInstance(instanceName, api.InstanceRebuildPost{
		Source: api.InstanceSource{
			Type:  "image",
			Alias: imageAlias,
		},
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

	op, err := server.DeleteInstance(instanceName)
	if err != nil {
		return "", err
	}
	if err := op.Wait(); err != nil {
		return "", err
	}

	return op.Get().ID, nil
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

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
