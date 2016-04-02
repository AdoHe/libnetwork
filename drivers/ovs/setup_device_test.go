package ovs

import (
	"net"
	"testing"
	"time"

	ovs "github.com/docker/libnetwork/drivers/ovs/ovsdbdriver"
	"github.com/docker/libnetwork/testutils"
	"github.com/vishvananda/netlink"
)

func TestSetupNewBridge(t *testing.T) {
	defer testutils.SetupTestOSContext(t)

	config := &networkConfiguration{BridgeName: DefaultOvsBridgeName}
	d, err := ovs.NewOvsdber("", 0)
	if err != nil {
		t.Fatalf("faild to connect to ovsdb: %v", err)
	}
	if err := setupDevice(d, config); err != nil {
		t.Fatalf("bridge creation failed: %v", err)
	}
	time.Sleep(5 * time.Second)
	link, err := netlink.LinkByName(DefaultOvsBridgeName)
	if err != nil {
		t.Fatalf("failed to retrieve bridge device: %v", err)
	}
	if link.Attrs().Flags&net.FlagUp == net.FlagUp {
		t.Fatalf("bridgeInterface should be created down")
	}
	d.Disconnect()
}

func TestSetupNewNonDefaultBridge(t *testing.T) {
	defer testutils.SetupTestOSContext(t)

	config := &networkConfiguration{BridgeName: "test0", DefaultBridge: true}
	d, err := ovs.NewOvsdber("", 0)
	if err != nil {
		t.Fatalf("failed to connect to ovsdb: %v", err)
	}

	err = setupDevice(d, config)
	if err == nil {
		t.Fatal("expected bridge creation failure with \"non default name\", succeeded")
	}
	if _, ok := err.(NonDefaultBridgeExistError); !ok {
		t.Fatalf("Did not fail with expected error. Actual error: %v", err)
	}
	d.Disconnect()
}

func TestSetupDeviceUp(t *testing.T) {
	defer testutils.SetupTestOSContext(t)

	config := &networkConfiguration{BridgeName: DefaultOvsBridgeName}
	d, err := ovs.NewOvsdber("", 0)
	if err != nil {
		t.Fatalf("failed to connect to ovsdb: %v", err)
	}
	if err := setupDevice(d, config); err != nil {
		t.Fatalf("bridge creation failed: %v", err)
	}
	time.Sleep(10 * time.Second)
	if err := setupDeviceUp(d, config); err != nil {
		t.Fatalf("failed to up bridge device: %v", err)
	}

	link, _ := netlink.LinkByName(DefaultOvsBridgeName)
	if link.Attrs().Flags&net.FlagUp != net.FlagUp {
		t.Fatalf("bridgeInterface should be up")
	}
	d.Disconnect()
}
