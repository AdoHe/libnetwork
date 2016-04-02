package ovs

import (
	ovs "github.com/docker/libnetwork/drivers/ovs/ovsdbdriver"
)

type setupStep func(*ovs.OvsdbDriver, *networkConfiguration) error

// We need this since we may do more setup work with
// the OVS bridge later.
type bridgeSetup struct {
	config *networkConfiguration
	ovs    *ovs.OvsdbDriver
	steps  []setupStep
}

func newBridgeSetup(ovs *ovs.OvsdbDriver, c *networkConfiguration) *bridgeSetup {
	return &bridgeSetup{
		ovs:    ovs,
		config: c,
	}
}

func (b *bridgeSetup) apply() error {
	for _, fn := range b.steps {
		if err := fn(b.ovs, b.config); err != nil {
			return err
		}
	}
	return nil
}

func (b *bridgeSetup) queueStep(step setupStep) {
	b.steps = append(b.steps, step)
}
