package ovs

import (
	"net"

	"github.com/vishvananda/netlink"
)

type bridgeInterface struct {
	Link        netlink.Link
	gatewayIPv4 net.IP
}

// newInterface creates a new bridge interface structure. It attempts
// to find an already existing device identified by the configuration
// BridgeName field, or the default bridge name when specified.
func newInterface(config *networkConfiguration) *bridgeInterface {
	i := &bridgeInterface{}

	// Initialize the bridge name to default if unspecified
	if config.BridgeName == "" {
		config.BridgeName = DefaultOvsBridgeName
	}

	// Attempt to find an existing bridge named with the specified name.
	i.Link, _ = netlink.LinkByName(config.BridgeName)
	return i
}

// exists indicates if the existing bridge interface exists on the system.
func (i *bridgeInterface) exists() bool {
	return i.Link != nil
}
