package ovs

import (
	"fmt"
	"os/exec"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/parsers/kernel"
	ovs "github.com/docker/libnetwork/drivers/ovs/ovsdbdriver"
	"github.com/docker/libnetwork/netutils"
	"github.com/vishvananda/netlink"
)

// SetupDevice create a new ovs bridge interface if necessary.
func setupDevice(d *ovs.OvsdbDriver, config *networkConfiguration) error {
	var setMac bool

	// We only attempt to create the bridge when the requested device name is
	// the default one.
	if config.BridgeName != DefaultOvsBridgeName && config.DefaultBridge {
		return NonDefaultBridgeExistError(config.BridgeName)
	}

	// Only set the bridge's MAC address if the kernel version is > 3.3
	kv, err := kernel.GetKernelVersion()
	if err != nil {
		logrus.Errorf("Failed to check kernel version: %v. Will not assign MAC address to the bridge interface", err)
	} else {
		setMac = kv.Kernel > 3 || (kv.Kernel == 3 && kv.Major >= 3)
	}

	logrus.Debugf("ovs driver add ovs bridge")
	if err = d.AddOvsBridge(config.BridgeName, false); err != nil {
		// fallback to ovs-vsctl
		if _, ok := err.(ovs.ErrBridgeAlreadyExists); ok {
			fmt.Println("bridge alreay exists, just skip")
			return nil
		}
		logrus.Debugf("Failed to create bridge %s via libovsdb %v. Trying ovs-vsctl", config.BridgeName, err)
		fmt.Printf("fall back to ovs-vsctl for err: %v", err)
		return ovsctlCreateBridge(config.BridgeName, setMac)
	}

	logrus.Debugf("ovs driver add ovs bridge done")
	return err
}

func ovsctlCreateBridge(bridgeName string, setMacAddr bool) error {
	if out, err := exec.Command("ovs-vsctl", "add-br", bridgeName).Output(); err != nil {
		logrus.Errorf("Failed to create ovs bridge %s with message %s, error: %v", bridgeName, out, err)
		return err
	}

	if setMacAddr {
		hwAddr := netutils.GenerateRandomMAC().String()
		hwAddrArg := fmt.Sprintf("other-config:hwaddr=%s", hwAddr)
		if _, err := exec.Command("ovs-vsctl", "set", "bridge", bridgeName, hwAddrArg).Output(); err != nil {
			return fmt.Errorf("Failed to set bridge mac-address %s : %s", hwAddr, err.Error())
		}
		logrus.Debugf("Setting bridge mac address to %s", hwAddr)
	}
	return nil
}

// SetupDeviceUp ups the given bridge interface.
func setupDeviceUp(_ *ovs.OvsdbDriver, config *networkConfiguration) error {
	link, err := netlink.LinkByName(config.BridgeName)
	if err != nil {
		logrus.Debugf("Error retriving bridge %s: %v", config.BridgeName, err)
		fmt.Printf("error retriving bridge %s: %v\n", config.BridgeName, err)
		return err
	}

	return netlink.LinkSetUp(link)
}
