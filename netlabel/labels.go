package netlabel

import (
	"strings"
)

const (
	// Prefix constant marks the reserved label space for libnetwork
	Prefix = "com.docker.network"

	// DriverPrefix constant marks the reserved label space for libnetwork drivers
	DriverPrefix = Prefix + ".driver"

	// DriverPrivatePrefix constant marks the reserved label space
	// for internal libnetwork drivers
	DriverPrivatePrefix = DriverPrefix + ".private"

	// GenericData constant that helps to identify an option as a Generic constant
	GenericData = Prefix + ".generic"

	// PortMap constant represents Port Mapping
	PortMap = Prefix + ".portmap"

	// MacAddress constant represents Mac Address config of a Container
	MacAddress = Prefix + ".endpoint.macaddress"

	// ExposedPorts constant represents the container's Exposed Ports
	ExposedPorts = Prefix + ".endpoint.exposedports"

	// PublicIP constant represents the container's Public IP Address
	PublicIP = Prefix + ".endpoint.publicip"

	// NetworkName constant represents the network name which the container belonging
	NetworkName = Prefix + ".endpoint.networkname"

	// VlanTag constant represents the container's vlan id
	VlanTag = Prefix + ".endpoint.vlantag"

	// ContainerID constant represents the container's id
	ContainerID = Prefix + ".endpoint.containerid"

	//EnableIPv6 constant represents enabling IPV6 at network level
	EnableIPv6 = Prefix + ".enable_ipv6"

	// DriverMTU constant represents the MTU size for the network driver
	DriverMTU = DriverPrefix + ".mtu"

	// OverlayBindInterface constant represents overlay driver bind interface
	OverlayBindInterface = DriverPrefix + ".overlay.bind_interface"

	// OverlayNeighborIP constant represents overlay driver neighbor IP
	OverlayNeighborIP = DriverPrefix + ".overlay.neighbor_ip"

	// Gateway represents the gateway for the network
	Gateway = Prefix + ".gateway"
)

var (
	// GlobalKVProvider constant represents the KV provider backend
	GlobalKVProvider = MakeKVProvider("global")

	// GlobalKVProviderURL constant represents the KV provider URL
	GlobalKVProviderURL = MakeKVProviderURL("global")

	// GlobalKVProviderConfig constant represents the KV provider Config
	GlobalKVProviderConfig = MakeKVProviderConfig("global")

	// LocalKVProvider constant represents the KV provider backend
	LocalKVProvider = MakeKVProvider("local")

	// LocalKVProviderURL constant represents the KV provider URL
	LocalKVProviderURL = MakeKVProviderURL("local")

	// LocalKVProviderConfig constant represents the KV provider Config
	LocalKVProviderConfig = MakeKVProviderConfig("local")
)

// MakeKVProvider returns the kvprovider label for the scope
func MakeKVProvider(scope string) string {
	return DriverPrivatePrefix + scope + "kv_provider"
}

// MakeKVProviderURL returns the kvprovider url label for the scope
func MakeKVProviderURL(scope string) string {
	return DriverPrivatePrefix + scope + "kv_provider_url"
}

// MakeKVProviderConfig returns the kvprovider config label for the scope
func MakeKVProviderConfig(scope string) string {
	return DriverPrivatePrefix + scope + "kv_provider_config"
}

// Key extracts the key portion of the label
func Key(label string) (key string) {
	if kv := strings.SplitN(label, "=", 2); len(kv) > 0 {
		key = kv[0]
	}
	return
}

// Value extracts the value portion of the label
func Value(label string) (value string) {
	if kv := strings.SplitN(label, "=", 2); len(kv) > 1 {
		value = kv[1]
	}
	return
}

// KeyValue decomposes the label in the (key,value) pair
func KeyValue(label string) (key string, value string) {
	if kv := strings.SplitN(label, "=", 2); len(kv) > 0 {
		key = kv[0]
		if len(kv) > 1 {
			value = kv[1]
		}
	}
	return
}
