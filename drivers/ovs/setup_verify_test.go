package ovs

import (
	"testing"
)

func TestCheckDeviceExists(t *testing.T) {
	config := &networkConfiguration{
		BridgeName: "nonexist",
	}
	if err := setupVerifyInterface(nil, config); err == nil {
		t.Fatal("bridge named nonexist should not exist")
	}
}
