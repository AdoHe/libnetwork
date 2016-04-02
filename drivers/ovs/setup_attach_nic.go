package ovs

import (
	"fmt"
	"net"
	"os/exec"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	ovs "github.com/docker/libnetwork/drivers/ovs/ovsdbdriver"
	"github.com/docker/libnetwork/drivers/ovs/util"
	"github.com/vishvananda/netlink"
)

// setupAttachNIC attachs NIC on the host to the bridgeInterface
func setupAttachNIC(_ *ovs.OvsdbDriver, config *networkConfiguration) error {
	var err error

	// get host interface based on routes.
	hostInterface, err := util.ChooseHostInterface()
	if err != nil {
		return err
	}
	if hostInterface == nil {
		return fmt.Errorf("failed to find interface with default route")
	}
	addr, err := util.GetAddrFromInterface(hostInterface.Name)
	if err != nil {
		logrus.Errorf("failed to get host network interface IP addr. %v", err)
		return err
	}
	if addr == nil {
		return fmt.Errorf("host network interface is nil")
	}
	gw, err := util.GetHostDefaultGW()
	if err != nil {
		logrus.Errorf("failed to get host default gw: %v", err)
		return err
	}

	cm := NewCommandManager()
	// Add the BridgeAddIpCommand
	cm.AddCommand(&BridgeAddIpCommand{bridgeName: config.BridgeName, ip: addr.String()})
	// Add the RemoveIpCommand
	cm.AddCommand(&RemoveIpCommand{deviceName: hostInterface.Name, ip: addr.String()})
	// Add the BridgeAddIntfCommand
	cm.AddCommand(&BridgeAddIntfCommand{bridgeName: config.BridgeName, deviceName: hostInterface.Name})
	// Add the DeleteDefaultRouteCommand
	cm.AddCommand(&DeleteDefaultRouteCommand{device: hostInterface.Name, gw: gw})
	// Add the AddDefaultRouteCommand
	cm.AddCommand(&AddDefaultRouteCommand{bridgeName: config.BridgeName, deviceName: hostInterface.Name, gw: gw})

	if err = cm.Execute(); err != nil {
		return fmt.Errorf("command manager execute err: %v", err)
	}
	return nil
}

type UndoableCommand interface {
	Execute() error
	Undo() error
}

type BridgeAddIpCommand struct {
	bridgeName string
	ip         string // ip is in CIDR notation

	link netlink.Link
	addr *netlink.Addr
}

// Add an IP address to the bridge device.
// Equivalent to: `ip addr add $addr dev $bridge`
func (c *BridgeAddIpCommand) Execute() error {
	logrus.Debugf("start add ip to bridge")
	var err error
	if c.link, err = netlink.LinkByName(c.bridgeName); err != nil {
		return fmt.Errorf("Failed to find bridge %s : %v", c.bridgeName, err)
	}
	c.addr, err = netlink.ParseAddr(c.ip)
	if err != nil {
		return fmt.Errorf("ip %s should be in $ip/$mask format", c.ip)
	}
	if err = netlink.AddrAdd(c.link, c.addr); err != nil {
		return fmt.Errorf("Failed to add address %s to bridge %s : %v", c.ip, c.bridgeName, err)
	}
	logrus.Debugf("bridge add ip done")

	return err
}

func (c *BridgeAddIpCommand) Undo() error {
	logrus.Debugf("bridge add ip undo")
	return netlink.AddrDel(c.link, c.addr)
}

type RemoveIpCommand struct {
	deviceName string
	ip         string // ip is in CIDR notation

	link netlink.Link
	addr *netlink.Addr
}

// Delete an IP address from a link device.
// Equivalent to: `ip addr del $addr dev $link`
func (c *RemoveIpCommand) Execute() error {
	logrus.Debugf("statt remove ip")
	var err error
	if c.link, err = netlink.LinkByName(c.deviceName); err != nil {
		return fmt.Errorf("Failed to find link device %s : %v", c.deviceName, err)
	}
	c.addr, err = netlink.ParseAddr(c.ip)
	if err != nil {
		return fmt.Errorf("ip %s should be in $ip/$mask format", c.ip)
	}
	if err = netlink.AddrDel(c.link, c.addr); err != nil {
		return fmt.Errorf("Failed to delete link device %s address.(%v)", c.deviceName, err)
	}

	logrus.Debugf("remove ip done")
	return err
}

func (c *RemoveIpCommand) Undo() error {
	logrus.Debugf("remove ip undo")
	return netlink.AddrAdd(c.link, c.addr)
}

type BridgeAddIntfCommand struct {
	bridgeName string
	deviceName string
}

// Add a link device to the ovs bridge.
func (c *BridgeAddIntfCommand) Execute() error {
	logrus.Debugf("start bridge add intf")
	var err error
	if err = ovs.OvsctlCreateNormalPort(c.bridgeName, c.deviceName); err != nil {
		return fmt.Errorf("failed to add device %s to bridge %s: %v", c.deviceName, c.bridgeName, err)
	}
	logrus.Debugf("bridge add intf done")

	return err
}

func (c *BridgeAddIntfCommand) Undo() error {
	logrus.Debugf("bridge add intf undo")
	return ovs.OvsctlDeletePort(c.bridgeName, c.deviceName)
}

type DeleteDefaultRouteCommand struct {
	device string // device is the link device name which has default route
	gw     net.IP

	route *netlink.Route
}

// Delete the default route.
// Equivalent: `ip route del $route`
func (c *DeleteDefaultRouteCommand) Execute() error {
	logrus.Debugf("start deleting default route")
	var err error
	var link netlink.Link

	if link, err = netlink.LinkByName(c.device); err != nil {
		return fmt.Errorf("failed to get device %s which has default route: %v", c.device, err)
	}

	c.route = &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Table:     syscall.RT_TABLE_MAIN,
		Gw:        c.gw,
	}
	if err := netlink.RouteDel(c.route); err != nil {
		logrus.Debugf("failed to delete default route. Ignore this error")
	}
	logrus.Debugf("delete default route done")
	return nil
}

func (c *DeleteDefaultRouteCommand) Undo() error {
	logrus.Debugf("delete default route undo")
	return netlink.RouteAdd(c.route)
}

type AddDefaultRouteCommand struct {
	bridgeName string
	deviceName string
	gw         net.IP

	link  netlink.Link
	route *netlink.Route
}

// Add default route to the system
// Equivalent: `ip route add $route`
func (c *AddDefaultRouteCommand) Execute() error {
	logrus.Debugf("start adding default route again")
	var err error
	var link netlink.Link
	if link, err = netlink.LinkByName(c.bridgeName); err != nil {
		return fmt.Errorf("Failed to get bridge %s : %v", c.bridgeName, err)
	}
	c.route = &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Table:     syscall.RT_TABLE_MAIN,
		Gw:        c.gw,
	}
	if err = netlink.RouteAdd(c.route); err != nil {
		return fmt.Errorf("Failed to add default route %v", err)
	}

	cmd := exec.Command("ip", "route", "flush", "cache")
	_, err = cmd.CombinedOutput()
	if err != nil {
		logrus.Debugf("ip route flush cache fail. Ignore this error")
	}
	cmd = exec.Command("ip", "route", "del", "default", "via", c.gw.String(), "dev", c.deviceName)
	for retry := 1; retry <= 3; retry++ {
		_, err = cmd.CombinedOutput()
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		logrus.Debugf("delete originally default route fail after 3 retry. Ignore this error")
	}
	cmd = exec.Command("ip", "route", "flush", "cache")
	_, err = cmd.CombinedOutput()
	if err != nil {
		logrus.Debugf("ip route flush cache fail. Ignore this error")
	}
	logrus.Debugf("add default route again done")
	return nil
}

func (c *AddDefaultRouteCommand) Undo() error {
	logrus.Debugf("add default route undo")
	return netlink.RouteDel(c.route)
}

type CommandStack struct {
	commands []UndoableCommand
}

func NewStack(num int) *CommandStack {
	return &CommandStack{
		commands: make([]UndoableCommand, 0, num),
	}
}

// IsEmpty returns whether the stack is empty
func (s *CommandStack) IsEmpty() bool {
	return s.Len() == 0
}

// Len returns the number of commands in the stack
func (s *CommandStack) Len() int {
	return len(s.commands)
}

// Push put a command into stack
func (s *CommandStack) Push(command UndoableCommand) {
	s.commands = append(s.commands, command)
}

// Pop pops the top item out.
func (s *CommandStack) Pop() (UndoableCommand, error) {
	if s.Len() > 0 {
		rect := s.commands[s.Len()-1]
		s.commands = s.commands[:s.Len()-1]
		return rect, nil
	}
	return nil, ErrEmptyStack
}

// Peak returns the top item.
func (s *CommandStack) Peek() (UndoableCommand, error) {
	if s.Len() > 0 {
		return s.commands[s.Len()-1], nil
	}
	return nil, ErrEmptyStack
}

type CommandManager struct {
	commands     []UndoableCommand
	undoCommands *CommandStack
}

func NewCommandManager() *CommandManager {
	cm := &CommandManager{
		commands: make([]UndoableCommand, 0, 5),
	}
	cm.undoCommands = NewStack(5)
	return cm
}

func (cm *CommandManager) AddCommand(c UndoableCommand) {
	cm.commands = append(cm.commands, c)
}

// execute() executes the command chain, if command execute
// get error, undo previous commands
func (cm *CommandManager) Execute() error {
	errs := []error(nil)
	for i := range cm.commands {
		c := cm.commands[i]
		if err := c.Execute(); err != nil {
			logrus.Debugf("command execute err: %v", err)
			if !cm.undoCommands.IsEmpty() {
				for uc, err := cm.undoCommands.Pop(); err != ErrEmptyStack; uc, err = cm.undoCommands.Pop() {
					if err = uc.Undo(); err != nil {
						errs = append(errs, err)
					}
				}
				return util.NewAggregate(errs)
			} else {
				return fmt.Errorf("get error when undo command stack is empty %v", err)
			}
		}
		cm.undoCommands.Push(c)
	}

	return nil
}
