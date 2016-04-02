package ovs

import (
	"time"

	ovs "github.com/docker/libnetwork/drivers/ovs/ovsdbdriver"
	"github.com/vishvananda/netlink"
)

// Check whether the bridge interface exist
func setupVerifyInterface(_ *ovs.OvsdbDriver, config *networkConfiguration) error {
	var (
		maxRetry int = 3
		found        = false
	)

	for retry := 1; retry <= maxRetry; retry++ {
		_, err := netlink.LinkByName(config.BridgeName)
		if err == nil {
			found = true
			break
		}
		time.Sleep(2 * time.Second)
	}
	if !found {
		return &ErrNotFoundAfterMaxRetry{maxRetry: maxRetry}
	}
	return nil
}
