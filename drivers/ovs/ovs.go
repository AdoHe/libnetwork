package ovs

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/ovs/controller"
	ovs "github.com/docker/libnetwork/drivers/ovs/ovsdbdriver"
	"github.com/docker/libnetwork/drivers/ovs/util"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/osl"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

const (
	networkType      = "ovs"
	containerEthName = "eth"
	vethPrefix       = "veth"
	vethLen          = 7
	// DefaultBridgeName is the default name for bridge interface managed
	// by the driver when unspecified by the caller.
	DefaultOvsBridgeName = "ovs0"
)

// configuration info for the "ovs" driver
type configuration struct {
	EnableIPForwarding  bool
	EnableUserlandProxy bool

	OvsHost              string
	OvsPort              int
	NetworkControllerUrl string
}

// networkConfiguration for network specific configuration
type networkConfiguration struct {
	ID            string
	BridgeName    string
	Mtu           int
	DefaultBridge bool
	dbIndex       uint64
	dbExists      bool
}

// endpointConfiguration represents the user specified configuration.
type endpointConfiguration struct {
	MacAddress net.HardwareAddr

	NetworkName string
	// ID of container which this endpoint belongs to
	ContainerID string
	PublicIP    string
	VlanID      uint
}

type ovsEndpoint struct {
	id         string
	addr       *net.IPNet
	macAddress net.HardwareAddr
	config     *endpointConfiguration // User specified configuration
	srcName    string
	dstName    string // dstName is the host side veth pair name
}

type ovsNetwork struct {
	id        string
	bridge    *bridgeInterface // the ovs bridge's L3 interface
	config    *networkConfiguration
	endpoints map[string]*ovsEndpoint // key: endpoint id.
	driver    *driver                 // The network's driver
	sync.Mutex
}

type driver struct {
	config   *configuration
	network  *ovsNetwork
	networks map[string]*ovsNetwork

	ovsdber *ovs.OvsdbDriver
	client  *controller.Client

	store datastore.DataStore

	sync.Mutex
}

// New constructs a new ovs driver
func newDriver() *driver {
	return &driver{networks: map[string]*ovsNetwork{}, config: &configuration{}}
}

// Init registers a new instance of ovs driver
func Init(dc driverapi.DriverCallback, config map[string]interface{}) error {
	// try to modprobe openvswitch first in case not loaded into kernel
	if out, err := exec.Command("modprobe", "-v", "openvswitch").Output(); err != nil {
		logrus.Warnf("Running modprobe openvswitch failed with message: %s, error: %v", out, err)
	}

	d := newDriver()
	if err := d.configure(config); err != nil {
		return err
	}

	c := driverapi.Capability{
		DataScope: datastore.LocalScope,
	}
	return dc.RegisterDriver(networkType, d, c)
}

func (d *driver) configure(option map[string]interface{}) error {
	var (
		config *configuration
		err    error
	)

	genericData, ok := option[netlabel.GenericData]
	if !ok || genericData == nil {
		return nil
	}

	switch opt := genericData.(type) {
	case options.Generic:
		opaqueConfig, err := options.GenerateFromModel(opt, &configuration{})
		if err != nil {
			return err
		}
		config = opaqueConfig.(*configuration)
	case *configuration:
		config = opt
	default:
		return &ErrInvalidOvsDriverConfig{}
	}

	// Enable IPForwarding
	if config.EnableIPForwarding {
		err = setupIPForwarding()
		if err != nil {
			return err
		}
	}

	// Init ovs db connection
	ovsdber, err := ovs.NewOvsdber(config.OvsHost, config.OvsPort)
	if err != nil {
		return err
	}
	d.ovsdber = ovsdber

	// Init network controller client
	client, err := controller.NewClient(config.NetworkControllerUrl)
	if err != nil {
		return err
	}
	d.client = client

	err = d.initStore(option)
	if err != nil {
		return err
	}

	return nil
}

func (c *networkConfiguration) fromLabels(labels map[string]string) error {
	var err error
	for label, value := range labels {
		switch label {
		case BridgeName:
			c.BridgeName = value
		case netlabel.DriverMTU:
			if c.Mtu, err = strconv.Atoi(value); err != nil {
				return parseErr(label, value, err.Error())
			}
		case DefaultBridge:
			if c.DefaultBridge, err = strconv.ParseBool(value); err != nil {
				return parseErr(label, value, err.Error())
			}
		}
	}

	return nil
}

func parseErr(label, value, errString string) error {
	return types.BadRequestErrorf("failed to parse %s value: %v (%s)", label, value, errString)
}

// Validate performs a static validation on the network configuration parameters.
// Put more validation work here.
func (c *networkConfiguration) validate() error {
	if c.Mtu < 0 {
		return ErrInvalidMtu(c.Mtu)
	}

	return nil
}

func (d *driver) getNetwork(id string) (*ovsNetwork, error) {
	d.Lock()
	defer d.Unlock()

	if id == "" {
		return nil, types.BadRequestErrorf("invalid network id: %s", id)
	}

	if nw, ok := d.networks[id]; ok {
		return nw, nil
	}

	return nil, types.NotFoundErrorf("network not found: %s", id)
}

func parseNetworkGenericOptions(data interface{}) (*networkConfiguration, error) {
	var (
		err    error
		config *networkConfiguration
	)

	switch opt := data.(type) {
	case *networkConfiguration:
		config = opt
	case map[string]string:
		config = &networkConfiguration{}
		err = config.fromLabels(opt)
	case options.Generic:
		var opaqueConfig interface{}
		if opaqueConfig, err = options.GenerateFromModel(opt, config); err == nil {
			config = opaqueConfig.(*networkConfiguration)
		}
	default:
		err = types.BadRequestErrorf("do not recognize network configuration format: %T", opt)
	}

	return config, err
}

func parseNetworkOptions(id string, option options.Generic) (*networkConfiguration, error) {
	var (
		err    error
		config = &networkConfiguration{}
	)

	// Parse generic label first, config will be re-assigned
	if genData, ok := option[netlabel.GenericData]; ok && genData != nil {
		if config, err = parseNetworkGenericOptions(genData); err != nil {
			return nil, err
		}
	}

	// Finally validate the configuration
	if err = config.validate(); err != nil {
		return nil, err
	}

	if config.BridgeName == "" && config.DefaultBridge == false {
		config.BridgeName = "br-" + id[:12]
	}

	config.ID = id
	return config, nil
}

// Return a slice of networks over which called can iterate safely
func (d *driver) getNetworks() []*ovsNetwork {
	d.Lock()
	defer d.Unlock()

	ls := make([]*ovsNetwork, len(d.networks))
	for _, nw := range d.networks {
		ls = append(ls, nw)
	}
	return ls
}

// Create ovs network
func (d *driver) CreateNetwork(nid string, option map[string]interface{}, ipV4Data, ipV6Data []driverapi.IPAMData) error {
	// Sanity checks
	d.Lock()
	if _, ok := d.networks[nid]; ok {
		d.Unlock()
		return types.ForbiddenErrorf("network %s exists", nid)
	}
	d.Unlock()

	// Parse and validate the config. It should not conflict with existing networks' config
	config, err := parseNetworkOptions(nid, option)
	if err != nil {
		return err
	}

	if err = d.createNetwork(config); err != nil {
		return err
	}

	return d.storeUpdate(config)
}

func (d *driver) createNetwork(config *networkConfiguration) error {
	var err error

	defer osl.InitOSContext()

	// Create and set network handler in driver
	network := &ovsNetwork{
		id:        config.ID,
		endpoints: make(map[string]*ovsEndpoint),
		config:    config,
		driver:    d,
	}

	d.Lock()
	d.networks[config.ID] = network
	d.Unlock()

	// On failure make sure to reset driver network handler to nil
	defer func() {
		if err != nil {
			d.Lock()
			delete(d.networks, config.ID)
			d.Unlock()
		}
	}()

	// Create or retrieve the bridge L3 interface
	bridgeIface := newInterface(config)
	network.bridge = bridgeIface

	// Prepare the bridge setup configuration
	bridgeSetup := newBridgeSetup(d.ovsdber, config)

	// If the bridge interface donen't exist, we need to start the setup steps
	// by creating a new device and attach nic
	bridgeAlreadyExists := bridgeIface.exists()
	if !bridgeAlreadyExists {
		logrus.Infof("bridge not alreay exists setup device")
		bridgeSetup.queueStep(setupDevice)
	}

	// Check bridge device exists.
	bridgeSetup.queueStep(setupVerifyInterface)

	// Setup Loopback Address Routing
	if !d.config.EnableUserlandProxy {
		bridgeSetup.queueStep(setupLoopbackAddressRouting)
	}

	// Setup bridge device up.
	bridgeSetup.queueStep(setupDeviceUp)

	// Attach NIC to the bridge
	if !bridgeAlreadyExists {
		bridgeSetup.queueStep(setupAttachNIC)
	}

	// Apply the prepared list of steps, and abort at the first error.
	if err = bridgeSetup.apply(); err != nil {
		logrus.Debugf("setup execute err: %v", err)
		return err
	}

	return nil
}

func (d *driver) DeleteNetwork(nid string) error {
	var err error

	defer osl.InitOSContext()()

	// Get network handler and remove it from driver.
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()

	if !ok {
		return types.InternalMaskableErrorf("network %s does not exist", nid)
	}

	n.Lock()
	config := n.config
	n.Unlock()

	d.Lock()
	delete(d.networks, nid)
	d.Unlock()

	// On failure set network handler back in driver, but
	// only if it is not already taken over by some other thread.
	defer func() {
		if err != nil {
			d.Lock()
			if _, ok := d.networks[nid]; !ok {
				d.networks[nid] = n
			}
			d.Unlock()
		}
	}()

	// Sanity check
	if n == nil {
		err = driverapi.ErrNoNetwork(nid)
		return err
	}

	// TODO: endpoints consideration

	if config.DefaultBridge {
		return types.ForbiddenErrorf("default network of type \"%s\" cannot be deleted", networkType)
	}

	return d.storeDelete(config)
}

// Create a sandbox endpoint using ovs driver.
func (d *driver) CreateEndpoint(nid, eid string, ifInfo driverapi.InterfaceInfo, options map[string]interface{}) error {
	defer osl.InitOSContext()()

	if ifInfo == nil {
		return errors.New("Invalid interface info passed")
	}

	// Get the network handler and make sure it exists.
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()

	if !ok {
		return types.NotFoundErrorf("network %s does not exist", nid)
	}
	if n == nil {
		return driverapi.ErrNoNetwork(nid)
	}

	// Sanity check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check if endpoint id is good and retrieve correspond endpoint.
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return err
	}

	// Endpoint with that id exists either on desired or other sandbox.
	if ep != nil {
		return driverapi.ErrEndpointExists(eid)
	}

	// Try to convert the options to endpoint configuration.
	epConfig, err := parseEndpointOptions(options)
	if err != nil {
		return err
	}

	// Create and add the endpoint
	n.Lock()
	endpoint := &ovsEndpoint{id: eid, config: epConfig}
	n.endpoints[eid] = endpoint
	n.Unlock()

	// On failure make sure to remove the endpoint.
	defer func() {
		if err != nil {
			n.Lock()
			delete(n.endpoints, eid)
			n.Unlock()
		}
	}()

	// Generate a name for what will be the host side pipe interface.
	hostIfName, err := netutils.GenerateIfaceName(vethPrefix, vethLen)
	if err != nil {
		return err
	}

	// Generate a name for what will be the sandbox side pipe interface.
	containerIfName, err := netutils.GenerateIfaceName(vethPrefix, vethLen)
	if err != nil {
		return err
	}

	// Generate and add the interface pipe host <--> sandbox
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostIfName, TxQLen: 0},
		PeerName:  containerIfName,
	}
	if err = netlink.LinkAdd(veth); err != nil {
		return types.InternalErrorf("failed to add the host (%s) <=> sandbox (%s) pair interfaces: %v", hostIfName, containerIfName, err)
	}

	// Get the host side pipe interface handler.
	host, err := netlink.LinkByName(hostIfName)
	if err != nil {
		return types.InternalErrorf("failed to find host side interface %s: %v", hostIfName, err)
	}

	defer func() {
		if err != nil {
			netlink.LinkDel(host)
		}
	}()

	// Get the sandbox side pipe interface handler.
	sbox, err := netlink.LinkByName(containerIfName)
	if err != nil {
		return types.InternalErrorf("failed to find sandbox side interface %s: %v", containerIfName, err)
	}

	defer func() {
		if err != nil {
			netlink.LinkDel(sbox)
		}
	}()

	n.Lock()
	config := n.config
	n.Unlock()

	// Add bridge inherited attributes to pipe interfaces.
	if config.Mtu != 0 {
		err = netlink.LinkSetMTU(host, config.Mtu)
		if err != nil {
			return types.InternalErrorf("failed to set MTU on host interface %s: %v", hostIfName, err)
		}
		err = netlink.LinkSetMTU(sbox, config.Mtu)
		if err != nil {
			return types.InternalErrorf("failed to set MTU on sandbox interface %s: %v", containerIfName, err)
		}
	}

	logrus.Debugf("epConfig publicip %s", epConfig.PublicIP)
	logrus.Debugf("epConfig vlanid %d", epConfig.VlanID)
	logrus.Debugf("epConfig networkName %s", epConfig.NetworkName)
	// Request IP Resource for first time
	logrus.Debugf("epConfig before: %#v", epConfig)
	if epConfig.PublicIP == "" {
		logrus.Debugf("we need to request address")
		resp, err := d.client.RequestIP(epConfig.NetworkName, epConfig.ContainerID)
		if err != nil {
			return err
		}
		// PublicIP should be CIDR notation
		logrus.Debugf("resp: %v", resp)
		logrus.Debugf("resp fix ip: %s", resp.FixIP)
		logrus.Debugf("resp seg id: %d", resp.SegID)
		epConfig.PublicIP = resp.FixIP
		epConfig.VlanID = uint(resp.SegID)
	} else {
		logrus.Debugf("use already ip address")
	}

	logrus.Debugf("epConfig after: %#v", epConfig)

	// Attach host side pipe interface into ovs bridge
	if err = d.addToBridge(hostIfName, config.BridgeName, epConfig.VlanID); err != nil {
		return fmt.Errorf("adding interface %s to ovs bridge %s failed: %v", hostIfName, config.BridgeName, err)
	}

	// Create the sandbox side pipe interface
	endpoint.dstName = hostIfName
	endpoint.srcName = containerIfName
	endpoint.addr = getIPAddress(epConfig.PublicIP)

	// Set endpointInterface ip address
	if err = ifInfo.SetIPAddress(endpoint.addr); err != nil {
		return err
	}

	// Set endpointInterface vlan tag
	if err = ifInfo.SetVlanID(epConfig.VlanID); err != nil {
		return err
	}

	// Set endpointInterface network name
	if err = ifInfo.SetNetworkName(epConfig.NetworkName); err != nil {
		return err
	}

	// Down the interface before configuring mac address.
	if err = netlink.LinkSetDown(sbox); err != nil {
		return fmt.Errorf("could not set link down for container interface %s", containerIfName)
	}

	// Set the sbox's MAC. If specified, use the one configured by user, otherwise generate one based on IP.
	if endpoint.macAddress == nil {
		endpoint.macAddress = electMacAddress(epConfig, endpoint.addr.IP)
		if err = ifInfo.SetMacAddress(endpoint.macAddress); err != nil {
			return err
		}
	}
	err = netlink.LinkSetHardwareAddr(sbox, endpoint.macAddress)
	if err != nil {
		return fmt.Errorf("could not set mac address for container interface %s", containerIfName)
	}

	// Up the host interface after finishing all netlink configuration.
	if err = netlink.LinkSetUp(host); err != nil {
		return fmt.Errorf("could not set link up for host interface %s: %v", hostIfName, err)
	}

	return nil
}

func (d *driver) DeleteEndpoint(nid, eid string) error {
	var err error

	defer osl.InitOSContext()()

	// Get the network handler and make sure it exists.
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()

	if !ok {
		return types.NotFoundErrorf("network %s does not exist", nid)
	}
	if n == nil {
		return driverapi.ErrNoNetwork(nid)
	}

	// Sanity check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check endpoint id and if an endpoint is actually here.
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return err
	}
	if ep == nil {
		return InvalidEndpointIDError(eid)
	}

	// Remove it
	n.Lock()
	delete(n.endpoints, eid)
	n.Unlock()

	// On failure make sure to set back ep in n.endpoints, but only
	// if it hasn't been taken over already by some other thread.
	defer func() {
		if err != nil {
			n.Lock()
			if _, ok := n.endpoints[eid]; !ok {
				n.endpoints[eid] = ep
			}
			n.Unlock()
		}
	}()

	n.Lock()
	config := n.config
	n.Unlock()

	// Try removal ovsdb port record
	d.removeFromBridge(ep.dstName, config.BridgeName)

	// Try removal of link. Discard error: it is a best effort.
	// Also make sure defer does not see this error either.
	if link, err := netlink.LinkByName(ep.srcName); err == nil {
		netlink.LinkDel(link)
	}

	return nil
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	// Get the network handler and make sure it exists
	d.Lock()
	n, ok := d.networks[nid]
	d.Unlock()

	if !ok {
		return nil, types.NotFoundErrorf("network %s does not exist", nid)
	}
	if n == nil {
		return nil, driverapi.ErrNoNetwork(nid)
	}

	// Sanity check
	n.Lock()
	if n.id != nid {
		n.Unlock()
		return nil, InvalidNetworkIDError(nid)
	}
	n.Unlock()

	// Check if endpoint id is good and retrieve correspond endpoint
	ep, err := n.getEndpoint(eid)
	if err != nil {
		return nil, err
	}
	if ep == nil {
		return nil, driverapi.ErrNoEndpoint(eid)
	}

	m := make(map[string]interface{})

	if len(ep.macAddress) != 0 {
		m[netlabel.MacAddress] = ep.macAddress
	}

	return m, nil
}

func (d *driver) Join(nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, options map[string]interface{}) error {
	defer osl.InitOSContext()

	network, err := d.getNetwork(nid)
	if err != nil {
		return err
	}

	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return nil
	}

	if endpoint == nil {
		return EndpointNotFoundError(eid)
	}

	iNames := jinfo.InterfaceName()
	err = iNames.SetNames(endpoint.srcName, containerEthName)
	if err != nil {
		return err
	}

	_, err = util.GetHostDefaultGW()
	if err != nil {
		logrus.Infof("failed to get host default gw")
		return err
	}
	/*err = jinfo.SetGateway(net.ParseIP("192.168.1.1"))
	if err != nil {
		return err
	}*/

	return nil
}

func (d *driver) Leave(nid, eid string) error {
	defer osl.InitOSContext()

	network, err := d.getNetwork(nid)
	if err != nil {
		return err
	}

	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return err
	}

	if endpoint == nil {
		return EndpointNotFoundError(eid)
	}

	return nil
}

func (d *driver) Type() string {
	return networkType
}

// DiscoverNew is a notification for a new discovery event, such as a new node joining a cluster
func (d *driver) DiscoverNew(dType driverapi.DiscoveryType, data interface{}) error {
	return nil
}

// DiscoverDelete is a notification for a discovery event, such as a node leaving a cluster
func (d *driver) DiscoverDelete(dTYpe driverapi.DiscoveryType, data interface{}) error {
	return nil
}

// Call the controller to release ip address
func (d *driver) ReleaseIP(id, ip string) error {
	return d.client.ReleaseIP(id, ip)
}

// Attach host side interface to the bridge
func (d *driver) addToBridge(ifaceName, bridgeName string, vlanID uint) error {
	_, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("could not find interface %s: %v", ifaceName, err)
	}
	if err := d.ovsdber.AddOvsVethPort(bridgeName, ifaceName, vlanID); err != nil {
		logrus.Debugf("Failed to add %s to bridge via libovsdb. Trying ovs-vsctl: %v", ifaceName, err)
		_, err := net.InterfaceByName(ifaceName)
		if err != nil {
			return fmt.Errorf("could not find network interface %s: %v", ifaceName, err)
		}

		_, err = net.InterfaceByName(bridgeName)
		if err != nil {
			return fmt.Errorf("could not find bridge %s: %v", bridgeName, err)
		}

		return ovs.OvsctlCreateNormalPort(bridgeName, ifaceName)
	}

	return nil
}

// Deattach interface from bridge
func (d *driver) removeFromBridge(ifaceName, bridgeName string) error {
	if err := d.ovsdber.DeletePort(bridgeName, ifaceName); err != nil {
		return ovs.OvsctlDeletePort(bridgeName, ifaceName)
	}
	return nil
}

func (n *ovsNetwork) getEndpoint(eid string) (*ovsEndpoint, error) {
	n.Lock()
	defer n.Unlock()

	if eid == "" {
		return nil, InvalidEndpointIDError(eid)
	}

	if e, ok := n.endpoints[eid]; ok {
		return e, nil
	}
	return nil, nil
}

func getIPAddress(s string) *net.IPNet {
	ip, ipn, err := net.ParseCIDR(s)
	if err != nil {
		return nil
	}
	return &net.IPNet{
		IP:   ip,
		Mask: ipn.Mask,
	}
}

func electMacAddress(epConfig *endpointConfiguration, ip net.IP) net.HardwareAddr {
	if epConfig != nil && epConfig.MacAddress != nil {
		return epConfig.MacAddress
	}
	return generateMacAddr(ip)
}

func generateMacAddr(ip net.IP) net.HardwareAddr {
	hw := make(net.HardwareAddr, 6)

	hw[0] = 0x02

	hw[1] = 0x42

	copy(hw[2:], ip.To4())

	return hw
}

func setHairpinMode(link netlink.Link, enable bool) error {
	err := netlink.LinkSetHairpin(link, enable)
	if err != nil && err != syscall.EINVAL {
		// If error is not EINVAL something else went wrong, bail out right away
		return fmt.Errorf("unable to set hairpin mode on %s via netlink",
			link.Attrs().Name)
	}

	// Hairpin mode successfully set up
	if err == nil {
		return nil
	}

	// The netlink method failed with EINVAL which is probably because of an older kernel.
	// Try one more time via the sysfs method.
	path := filepath.Join("/sys/class/net", link.Attrs().Name, "brport/hairpin_mode")

	var val []byte
	if enable {
		val = []byte{'1', '\n'}
	} else {
		val = []byte{'0', '\n'}
	}

	if err := ioutil.WriteFile(path, val, 0644); err != nil {
		return fmt.Errorf("unable to set hairpin mode on %s via sysfs", link.Attrs().Name)
	}

	return nil
}

func parseEndpointOptions(epOptions map[string]interface{}) (*endpointConfiguration, error) {
	if epOptions == nil {
		return nil, nil
	}

	ec := &endpointConfiguration{}

	if opt, ok := epOptions[netlabel.MacAddress]; ok {
		if mac, ok := opt.(net.HardwareAddr); ok {
			ec.MacAddress = mac
		} else {
			return nil, &ErrInvalidEndpointConfig{}
		}
	}

	if opt, ok := epOptions[netlabel.PublicIP]; ok {
		if ip, ok := opt.(string); ok {
			ec.PublicIP = ip
		} else {
			return nil, &ErrInvalidEndpointConfig{}
		}
	}

	if opt, ok := epOptions[netlabel.NetworkName]; ok {
		if name, ok := opt.(string); ok {
			ec.NetworkName = name
		} else {
			return nil, &ErrInvalidEndpointConfig{}
		}
	}

	if opt, ok := epOptions[netlabel.VlanTag]; ok {
		if id, ok := opt.(uint); ok {
			ec.VlanID = id
		} else {
			return nil, &ErrInvalidEndpointConfig{}
		}
	}

	if opt, ok := epOptions[netlabel.ContainerID]; ok {
		if id, ok := opt.(string); ok {
			ec.ContainerID = id
		} else {
			return nil, &ErrInvalidEndpointConfig{}
		}
	}

	return ec, nil
}
