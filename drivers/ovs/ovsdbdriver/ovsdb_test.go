package ovsdbdriver

import (
	"fmt"
	"testing"
)

var bridgeName = "test-br0"

func TestValidOvsdbConnection(t *testing.T) {
	ovs, err := NewOvsdber("", 0)
	defer ovs.Disconnect()
	if err != nil {
		t.Fatal("error should be nil")
	}
	if ovs == nil {
		t.Fatal("should get a valid ovsdb connection")
	}
}

func TestCreateDeleteOvsBridge(t *testing.T) {
	ovs, err := NewOvsdber("", 0)
	if err != nil {
		t.Fatalf("failed to connect to ovsdb: %v", err)
	}
	err = ovs.createOvsBridge(bridgeName, false)
	if err != nil {
		t.Fatalf("failed to create ovs bridge %s: %v", bridgeName, err)
	}
	fmt.Printf("bridge %s create successful\n", bridgeName)
	err = ovs.deleteOvsBridge(bridgeName)
	if err != nil {
		t.Fatalf("failed to delete ovs bridge %s: %v", bridgeName, err)
	}
	fmt.Printf("bridge %s delete successful\n", bridgeName)
	ovs.Disconnect()
}

func TestAddRemoveOvsBridge(t *testing.T) {
	ovs, err := NewOvsdber("", 0)
	if err != nil {
		t.Fatalf("failed to connect to ovsdb: %v", err)
	}
	err = ovs.AddOvsBridge(bridgeName, false)
	if err != nil {
		t.Fatalf("failed to add ovs bridge %s: %v", bridgeName, err)
	}
	fmt.Printf("bridge %s add successful\n", bridgeName)
	err = ovs.RemoveOvsBridge(bridgeName)
	if err != nil {
		t.Fatalf("failed to remove ovs bridge %s: %v", bridgeName, err)
	}
	fmt.Printf("bridge %s remove successful\n", bridgeName)
	ovs.Disconnect()
}

func TestAddAlreadyExistOvsBridge(t *testing.T) {
	ovs, err := NewOvsdber("", 0)
	if err != nil {
		t.Fatalf("failed to connect to ovsdb: %v", err)
	}
	_ = ovs.AddOvsBridge(bridgeName, false)
	err = ovs.AddOvsBridge(bridgeName, false)
	if _, ok := err.(ErrBridgeAlreadyExists); !ok {
		t.Fatal("should get BridgeAlreayExists error")
	}
	err = ovs.RemoveOvsBridge(bridgeName)
	if err != nil {
		t.Fatalf("failed to remove ovs bridge %s: %v", bridgeName, err)
	}
	ovs.Disconnect()
}

func TestRemoveNonExistBridge(t *testing.T) {
	ovs, err := NewOvsdber("", 0)
	if err != nil {
		t.Fatalf("failed to connect to ovsdb: %v", err)
	}
	err = ovs.RemoveOvsBridge("nonexist")
	if _, ok := err.(ErrBridgeNotExists); !ok {
		t.Fatal("should fail when try to remove nonexist bridge")
	}
	ovs.Disconnect()
}
