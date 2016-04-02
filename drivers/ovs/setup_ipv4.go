package ovs

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	ovs "github.com/docker/libnetwork/drivers/ovs/ovsdbdriver"
)

// Enable Loopback Address Routing
func setupLoopbackAddressRouting(_ *ovs.OvsdbDriver, config *networkConfiguration) error {
	sysPath := filepath.Join("/proc/sys/net/ipv4/conf", config.BridgeName, "route_localnet")
	ipv4LoRoutingData, err := ioutil.ReadFile(sysPath)
	if err != nil {
		return fmt.Errorf("Cannot read IPv4 local routing setup: %v", err)
	}
	// Enable loopback address routing only if it isn't already enabled
	if ipv4LoRoutingData[0] != '1' {
		if err := ioutil.WriteFile(sysPath, []byte{'1', '\n'}, 0644); err != nil {
			return fmt.Errorf("Unable to enable local routing for hairpin mode: %v", err)
		}
	}
	return nil
}
