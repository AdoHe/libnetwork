package ovsdbdriver

import (
	"fmt"
	"testing"
	"time"
)

const (
	testBridgeName = "TestBridge"
	testPortName   = "TestPort"
	defaultTag     = 100
)

type TestOvsdber struct {
	bridgeName string

	delegate *OvsdbDriver
}

func NewTestOvsdber(t *testing.T) *TestOvsdber {
	ovs, err := NewOvsdber("", 0)
	if err != nil {
		t.Fatal("failed to connect to ovsdb")
	}
	o := &TestOvsdber{
		bridgeName: testBridgeName,
		delegate:   ovs,
	}
	err = o.delegate.AddOvsBridge(o.bridgeName, false)
	if err != nil {
		t.Fatal("failed to create test ovs bridge")
	}

	return o
}

func (o *TestOvsdber) Terminate(t *testing.T) {
	err := o.delegate.RemoveOvsBridge(o.bridgeName)
	if err != nil {
		t.Fatal("failed to remove test ovs bridge")
	}
	o.delegate.Disconnect()
}

func (o *TestOvsdber) addVethPairPort(portName string) error {
	return o.delegate.AddOvsVethPort(o.bridgeName, portName, defaultTag)
}

func (o *TestOvsdber) addInternalPort(portName string) error {
	return o.delegate.AddOvsInternalPort(o.bridgeName, portName, defaultTag)
}

func (o *TestOvsdber) deletePort(portName string) error {
	return o.delegate.DeletePort(o.bridgeName, portName)
}

func (o *TestOvsdber) portExists(portName string) (bool, error) {
	return o.delegate.portExists(portName)
}

func TestAddVethPairPort(t *testing.T) {
	ovs := NewTestOvsdber(t)
	defer ovs.Terminate(t)
	err := ovs.addVethPairPort(testPortName)
	if err != nil {
		t.Fatalf("failed to add veth pair port %s: %v", testPortName, err)
	}

	// wait a little for OVS to create the interface
	time.Sleep(300 * time.Millisecond)

	fmt.Printf("veth port %s create successful\n", testPortName)
	exists, err := ovs.portExists(testPortName)
	if err != nil {
		t.Fatalf("failed to query port %s: %v", testPortName, err)
	}
	if !exists {
		t.Fatalf("failed to create port %s", testPortName)
	}
}

func TestAddInternalPort(t *testing.T) {
	ovs := NewTestOvsdber(t)
	defer ovs.Terminate(t)
	err := ovs.addInternalPort(testPortName)
	if err != nil {
		t.Fatalf("failed to add ovs internal port %s: %v", testPortName, err)
	}

	// wait a little for OVS to create the interface
	time.Sleep(300 * time.Millisecond)

	fmt.Printf("internal port %s create successful\n", testPortName)
	exists, err := ovs.portExists(testPortName)
	if err != nil {
		t.Fatalf("failed to query port %s: %v", testPortName, err)
	}
	if !exists {
		t.Fatalf("failed to create port %s", testPortName)
	}
}

func TestDeletePort(t *testing.T) {
	ovs := NewTestOvsdber(t)
	defer ovs.Terminate(t)
	err := ovs.addVethPairPort(testPortName)
	if err != nil {
		t.Fatalf("failed to add veth pair port %s: %v", testPortName, err)
	}
	fmt.Printf("veth port %s create successful\n", testPortName)
	exists, err := ovs.portExists(testPortName)
	if err != nil {
		t.Fatalf("failed to query port %s: %v", testPortName, err)
	}
	if !exists {
		t.Fatalf("failed to create port %s", testPortName)
	}

	// wait a little for OVS to create the interface
	time.Sleep(300 * time.Millisecond)

	err = ovs.deletePort(testPortName)
	if err != nil {
		t.Fatalf("failed to delete port %s: %v", testPortName, err)
	}
	exists, err = ovs.portExists(testPortName)
	if err != nil {
		t.Fatalf("failed to query port %s: %v", testPortName, err)
	}
	if exists {
		t.Fatalf("failed to delete port %s", testPortName)
	}
	fmt.Printf("veth port %s delete successful\n", testPortName)
}
