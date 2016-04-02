package ovs

import (
	"errors"
	"fmt"
)

// ErrConfigExists error is returned when driver already has a configuration.
type ErrConfigExists struct{}

func (ece *ErrConfigExists) Error() string {
	return "configuration already exists, bridge configuration can be applied only once"
}

// Forbidden denotes the type of this error
func (ece *ErrConfigExists) Forbidden() {}

// ErrInvalidDriverConfig error is returned when Ovs Driver is passed an invalid configuration.
type ErrInvalidOvsDriverConfig struct{}

func (eiodc ErrInvalidOvsDriverConfig) Error() string {
	return fmt.Sprint("invalid ovs driver config")
}

// ErrNotFoundAfterMaxRetry err is returned when still not found after maxRetry count.
type ErrNotFoundAfterMaxRetry struct {
	maxRetry int
}

func (enfmr *ErrNotFoundAfterMaxRetry) Error() string {
	return fmt.Sprintf("Not found event after maxRetry: %d", enfmr.maxRetry)
}

// InvalidNetworkIDError is returned when the passed
// network id for an existing network is not a known id.
type InvalidNetworkIDError string

func (inie InvalidNetworkIDError) Error() string {
	return fmt.Sprintf("invalid network id %s", string(inie))
}

// InvalidEndpointIDError is returned when the passed
// endpoint id is empty.
type InvalidEndpointIDError string

func (ieie InvalidEndpointIDError) Error() string {
	return fmt.Sprintf("invalid endpoint id %s", string(ieie))
}

type ErrIPFwdCfg struct{}

func (eipf *ErrIPFwdCfg) Error() string {
	return "unexpected request to enable IP Forwarding"
}

// BadRequest denotes the type of this error
func (eipf *ErrIPFwdCfg) BadRequest() {}

// NonDefaultBridgeExistError is returned when a non-default
// bridge config is passed but it does not already exist.
type NonDefaultBridgeExistError string

func (ndbee NonDefaultBridgeExistError) Error() string {
	return fmt.Sprintf("ovs bridge with non default name %s must be created manually", string(ndbee))
}

// Forbidden denotes the type of this error
func (ndbee NonDefaultBridgeExistError) Forbidden() {}

// ErrInvalidName is returned when a query-by-name or resource create method is
// invoked with an empty name parameter
type ErrInvalidName string

func (in ErrInvalidName) Error() string {
	return fmt.Sprintf("invalid name: %s", string(in))
}

// BadRequest denotes the type of this error
func (in ErrInvalidName) BadRequest() {}

// ErrInvalidMtu is returned when the user provided MTU is invalid
type ErrInvalidMtu int

func (eim ErrInvalidMtu) Error() string {
	return fmt.Sprintf("invalid MTU number: %d", int(eim))
}

// BadRequest denotes the type of this error
func (eim ErrInvalidMtu) BadRequest() {}

// ErrInvalidEndpointConfig error is returned when a endpoint create
// is attempted with an invalid endpoint configuration.
type ErrInvalidEndpointConfig struct{}

func (eiec *ErrInvalidEndpointConfig) Error() string {
	return "trying to create an endpoint with an invalid endpoint configuration"
}

// BadRequest denotes the type of the error
func (eiec *ErrInvalidEndpointConfig) BadRequest() {}

// EndpointNotFoundError is returned when the no endpoint
// with the passed endpoint id is found.
type EndpointNotFoundError string

func (enfe EndpointNotFoundError) Error() string {
	return fmt.Sprintf("endpoint not found: %s", string(enfe))
}

// NotFound denotes the type of this error
func (enfe EndpointNotFoundError) NotFound() {}

var ErrEmptyStack = errors.New("stack is empty")
