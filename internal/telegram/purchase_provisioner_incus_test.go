package telegram

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lxc/incus/v6/shared/api"
)

func TestInstanceSourceFromImageAliasRemoteSimpleStreams(t *testing.T) {
	t.Parallel()

	source, err := instanceSourceFromImageAlias("images:ubuntu/24.04")
	if err != nil {
		t.Fatalf("instanceSourceFromImageAlias returned error: %v", err)
	}

	if source.Type != "image" {
		t.Fatalf("expected type image, got %q", source.Type)
	}
	if source.Alias != "ubuntu/24.04" {
		t.Fatalf("expected alias ubuntu/24.04, got %q", source.Alias)
	}
	if source.Server != "https://images.linuxcontainers.org" {
		t.Fatalf("expected images server, got %q", source.Server)
	}
	if source.Protocol != "simplestreams" {
		t.Fatalf("expected simplestreams protocol, got %q", source.Protocol)
	}
}

func TestInstanceSourceFromImageAliasLocalAlias(t *testing.T) {
	t.Parallel()

	source, err := instanceSourceFromImageAlias("ubuntu-local")
	if err != nil {
		t.Fatalf("instanceSourceFromImageAlias returned error: %v", err)
	}

	if source.Type != "image" {
		t.Fatalf("expected type image, got %q", source.Type)
	}
	if source.Alias != "ubuntu-local" {
		t.Fatalf("expected alias ubuntu-local, got %q", source.Alias)
	}
	if source.Server != "" {
		t.Fatalf("expected empty server, got %q", source.Server)
	}
	if source.Protocol != "" {
		t.Fatalf("expected empty protocol, got %q", source.Protocol)
	}
}

func TestInstanceSourceFromImageAliasUnsupportedRemote(t *testing.T) {
	t.Parallel()

	_, err := instanceSourceFromImageAlias("foo:bar")
	if err == nil {
		t.Fatalf("expected error for unsupported remote")
	}
}

func TestWaitForPrivateIPv4RetriesUntilAddressAvailable(t *testing.T) {
	t.Parallel()

	states := []*api.InstanceState{
		{Network: map[string]api.InstanceStateNetwork{"eth0": {Addresses: []api.InstanceStateNetworkAddress{}}}},
		{Network: map[string]api.InstanceStateNetwork{"eth0": {Addresses: []api.InstanceStateNetworkAddress{
			{Family: "inet", Scope: "global", Address: "10.228.49.134"},
		}}}},
	}
	calls := 0

	state, ip, err := waitForPrivateIPv4(func() (*api.InstanceState, error) {
		if calls >= len(states) {
			return states[len(states)-1], nil
		}
		current := states[calls]
		calls++
		return current, nil
	}, 3, 0)
	if err != nil {
		t.Fatalf("waitForPrivateIPv4 returned error: %v", err)
	}
	if state == nil {
		t.Fatalf("expected state to be returned")
	}
	if ip != "10.228.49.134" {
		t.Fatalf("expected ip 10.228.49.134, got %q", ip)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestWaitForPrivateIPv4ReturnsFetchError(t *testing.T) {
	t.Parallel()

	expected := errors.New("boom")
	_, _, err := waitForPrivateIPv4(func() (*api.InstanceState, error) {
		return nil, expected
	}, 3, time.Millisecond)
	if !errors.Is(err, expected) {
		t.Fatalf("expected fetch error to be returned, got %v", err)
	}
}

func TestPasswordExecPostDoesNotRecordOutput(t *testing.T) {
	t.Parallel()

	post := passwordExecPost("secret123")

	if post.RecordOutput {
		t.Fatalf("expected RecordOutput to be false")
	}

	expected := []string{"sh", "-lc", "echo 'root':'secret123' | chpasswd"}
	if !reflect.DeepEqual(post.Command, expected) {
		t.Fatalf("unexpected command: %#v", post.Command)
	}
}
