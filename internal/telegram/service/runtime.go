package service

import (
	"fmt"
	"strings"
)

func NormalizeRuntimeAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "start":
		return "start"
	case "stop":
		return "stop"
	case "reboot", "restart":
		return "restart"
	default:
		return ""
	}
}

func RuntimeStatuses(action string) (string, string) {
	switch action {
	case "start":
		return "running", "active"
	case "stop":
		return "stopped", "stopped"
	case "restart":
		return "running", "active"
	default:
		return "pending", "pending"
	}
}

func RuntimeEventType(action string) string {
	switch action {
	case "start":
		return "container_started"
	case "stop":
		return "container_stopped"
	default:
		return "container_restarted"
	}
}

func RuntimeSummary(action string, instanceName string) string {
	switch action {
	case "start":
		return fmt.Sprintf("container %s berhasil dijalankan", instanceName)
	case "stop":
		return fmt.Sprintf("container %s berhasil dihentikan", instanceName)
	default:
		return fmt.Sprintf("container %s berhasil direstart", instanceName)
	}
}

func ActionToJobType(action string) string {
	switch action {
	case "restart":
		return "restart"
	default:
		return action
	}
}

func IsRunningStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "running")
}
