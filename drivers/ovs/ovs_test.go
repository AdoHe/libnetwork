package ovs

import (
	"testing"
)

// Leave this test cases for non-default networks
func TestCreateFullOptions(t *testing.T) {

}

func TestCreateNoConfig(t *testing.T) {

}

func TestCreateFullOptionsLabels(t *testing.T) {

}

func TestCreate(t *testing.T) {

}

func TestCreateFail(t *testing.T) {

}

func TestQueryEndpointInfo(t *testing.T) {

}

func TestValidateConfig(t *testing.T) {
	// Test mtu
	c := networkConfiguration{Mtu: -2}
	err := c.validate()
	if err == nil {
		t.Fatalf("failed to detect invalid mtu number")
	}

	c.Mtu = 1500
	err = c.validate()
	if err != nil {
		t.Fatalf("unexpected validation error on MTU number")
	}
}
