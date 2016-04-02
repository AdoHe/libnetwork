package ovs

import (
	"fmt"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

type fakeBridge struct {
	netlink.LinkAttrs

	addr *netlink.Addr
}

func (bridge *fakeBridge) Attrs() *netlink.LinkAttrs {
	return &bridge.LinkAttrs
}

func (bridge *fakeBridge) Type() string {
	return "fake-bridge"
}

type fakeDevice struct {
	netlink.LinkAttrs

	addr *netlink.Addr
}

func (device *fakeDevice) Attrs() *netlink.LinkAttrs {
	return &device.LinkAttrs
}

func (device *fakeDevice) Type() string {
	return "fake-device"
}

type fakeBridgeAddIpCommand struct {
	bridgeName string
	ip         string

	link netlink.Link
	addr *netlink.Addr
}

func (fc *fakeBridgeAddIpCommand) Execute() error {
	if fc.bridgeName == "non-bridge" {
		return fmt.Errorf("could not find a bridge named \"no-bridge\"")
	}
	fc.link = &fakeBridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: fc.bridgeName,
		},
	}
	fc.addr, _ = netlink.ParseAddr(fc.ip)
	bridgeAddIp(fc.link, fc.addr)
	return nil
}

func (fc *fakeBridgeAddIpCommand) Undo() error {
	bridgeRemoveIp(fc.link)
	fc.addr = nil
	return nil
}

func bridgeAddIp(link netlink.Link, addr *netlink.Addr) {
	br, _ := link.(*fakeBridge)
	br.addr = addr
}

func bridgeRemoveIp(link netlink.Link) {
	br, _ := link.(*fakeBridge)
	br.addr = nil
}

type fakeRemoveIpCommand struct {
	deviceName string
	ip         string

	link netlink.Link
	addr *netlink.Addr
}

func (fc *fakeRemoveIpCommand) Execute() error {
	if fc.deviceName == "non-device" {
		return fmt.Errorf("could not find device named \"non-device\"")
	}
	fc.addr, _ = netlink.ParseAddr(fc.ip)
	fc.link = &fakeDevice{
		LinkAttrs: netlink.LinkAttrs{
			Name: fc.deviceName,
		},
		addr: fc.addr,
	}
	deviceRemoveIp(fc.link)
	return nil
}

func (fc *fakeRemoveIpCommand) Undo() error {
	deviceAddIp(fc.link, fc.addr)
	return nil
}

func deviceRemoveIp(link netlink.Link) {
	fd, _ := link.(*fakeDevice)
	fd.addr = nil
}

func deviceAddIp(link netlink.Link, addr *netlink.Addr) {
	fd, _ := link.(*fakeDevice)
	fd.addr = addr
}

type fakeBridgeAddIntfCommand struct {
	bridgeName string
	deviceName string

	bridge *netlink.Bridge
	device netlink.Link
}

func (fc *fakeBridgeAddIntfCommand) Execute() error {
	if fc.deviceName == "non-device" {
		return fmt.Errorf("could not find device named \"non-device\"")
	}
	return nil
}

func (fc *fakeBridgeAddIntfCommand) Undo() error {
	return nil
}

type fakeDeleteDefaultRouteCommand struct {
	device string
	route  *netlink.Route
}

func (fc *fakeDeleteDefaultRouteCommand) Execute() error {
	return nil
}

func (fc *fakeDeleteDefaultRouteCommand) Undo() error {
	return nil
}

type fakeAddDefaultRouteCommand struct {
	bridgeName string
	gw         net.IP

	link  netlink.Link
	route *netlink.Route
}

func (fc *fakeAddDefaultRouteCommand) Execute() error {
	return nil
}

func (fc *fakeAddDefaultRouteCommand) Undo() error {
	return nil
}

func TestBridgeAddIpFail(t *testing.T) {
	cm := NewCommandManager()
	cm.AddCommand(&fakeBridgeAddIpCommand{bridgeName: "non-bridge"})
	cm.AddCommand(&fakeRemoveIpCommand{})

	err := cm.Execute()
	if err == nil {
		t.Fatalf("nonexist bridge should not add ip")
	}
	if !cm.undoCommands.IsEmpty() {
		t.Fatalf("command manager undo commands stack should be empty")
	}
}

func TestRemoveIpFail(t *testing.T) {
	cm := NewCommandManager()
	fBridgeCmd := &fakeBridgeAddIpCommand{bridgeName: "br0", ip: "10.10.101.92/24"}
	fRemoveCmd := &fakeRemoveIpCommand{deviceName: "non-device"}
	cm.AddCommand(fBridgeCmd)
	cm.AddCommand(fRemoveCmd)

	err := cm.Execute()
	if err != nil {
		t.Fatalf("unexpected err when run commands")
	}
	if fBridgeCmd.addr != nil {
		t.Fatalf("bridge addr should be nil once undo finish")
	}
	br, _ := fBridgeCmd.link.(*fakeBridge)
	if br.addr != nil {
		t.Fatalf("bridge device addr should be nil after undo")
	}
}

func TestBridgeAddIntfFail(t *testing.T) {
	cm := NewCommandManager()
	fBridgeCmd := &fakeBridgeAddIpCommand{bridgeName: "br0", ip: "10.10.101.92/24"}
	fRemoveCmd := &fakeRemoveIpCommand{deviceName: "eth0", ip: "10.10.101.92/24"}
	fBridgeAddIntfCmd := &fakeBridgeAddIntfCommand{bridgeName: "br0", deviceName: "non-device"}
	cm.AddCommand(fBridgeCmd)
	cm.AddCommand(fRemoveCmd)
	cm.AddCommand(fBridgeAddIntfCmd)

	err := cm.Execute()
	if err != nil {
		t.Fatalf("unexpected err when run commands")
	}
	lnk, _ := fRemoveCmd.link.(*fakeDevice)
	if lnk.addr == nil {
		t.Fatalf("device link addr should keep original once undo")
	}
	if !lnk.addr.Equal(*fRemoveCmd.addr) {
		t.Fatalf("device link addr should keep original once undo")
	}
	if fBridgeCmd.addr != nil {
		t.Fatalf("bridge addr should be nil once undo finish")
	}
	br, _ := fBridgeCmd.link.(*fakeBridge)
	if br.addr != nil {
		t.Fatalf("bridge device addr should be nil after undo")
	}
}
