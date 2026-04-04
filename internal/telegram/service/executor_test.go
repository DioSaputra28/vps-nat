package service

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/gorilla/websocket"
	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

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

func TestDeleteInstanceWithStopIfNeededStopsRunningInstanceFirst(t *testing.T) {
	t.Parallel()

	server := &fakeDeleteServer{
		instance: &api.Instance{Status: "Running"},
		stopOp:   fakeOperation{id: "op-stop"},
		deleteOp: fakeOperation{id: "op-delete"},
	}

	opID, err := deleteInstanceWithStopIfNeeded(server, "inst-one")
	if err != nil {
		t.Fatalf("deleteInstanceWithStopIfNeeded returned error: %v", err)
	}

	if opID != "op-delete" {
		t.Fatalf("expected delete operation id op-delete, got %q", opID)
	}
	if len(server.actions) != 2 {
		t.Fatalf("expected two actions, got %#v", server.actions)
	}
	if server.actions[0] != "stop:inst-one:true" {
		t.Fatalf("unexpected first action: %s", server.actions[0])
	}
	if server.actions[1] != "delete:inst-one" {
		t.Fatalf("unexpected second action: %s", server.actions[1])
	}
}

func TestDeleteInstanceWithStopIfNeededDeletesStoppedInstanceDirectly(t *testing.T) {
	t.Parallel()

	server := &fakeDeleteServer{
		instance: &api.Instance{Status: "Stopped"},
		deleteOp: fakeOperation{id: "op-delete"},
	}

	_, err := deleteInstanceWithStopIfNeeded(server, "inst-one")
	if err != nil {
		t.Fatalf("deleteInstanceWithStopIfNeeded returned error: %v", err)
	}

	if len(server.actions) != 1 || server.actions[0] != "delete:inst-one" {
		t.Fatalf("unexpected actions: %#v", server.actions)
	}
}

type fakeDeleteServer struct {
	instance *api.Instance
	stopOp   incusclient.Operation
	deleteOp incusclient.Operation
	actions  []string
}

func (f *fakeDeleteServer) GetInstance(name string) (*api.Instance, string, error) {
	return f.instance, "", nil
}

func (f *fakeDeleteServer) UpdateInstanceState(name string, state api.InstanceStatePut, etag string) (incusclient.Operation, error) {
	f.actions = append(f.actions, "stop:"+name+":"+strconv.FormatBool(state.Force))
	return f.stopOp, nil
}

func (f *fakeDeleteServer) DeleteInstance(name string) (incusclient.Operation, error) {
	f.actions = append(f.actions, "delete:"+name)
	return f.deleteOp, nil
}

type fakeOperation struct {
	id string
}

func (f fakeOperation) AddHandler(func(api.Operation)) (*incusclient.EventTarget, error) {
	return nil, nil
}
func (f fakeOperation) Cancel() error                                { return nil }
func (f fakeOperation) Get() api.Operation                           { return api.Operation{ID: f.id} }
func (f fakeOperation) GetWebsocket(string) (*websocket.Conn, error) { return nil, nil }
func (f fakeOperation) RemoveHandler(*incusclient.EventTarget) error { return nil }
func (f fakeOperation) Refresh() error                               { return nil }
func (f fakeOperation) Wait() error                                  { return nil }
func (f fakeOperation) WaitContext(context.Context) error            { return nil }
