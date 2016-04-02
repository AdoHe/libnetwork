package util

import (
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
)

const gatewayfirst = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth3	00000000	0100FE0A	0003	0	0	1024	00000000	0	0	0
eth3	0000FE0A	00000000	0001	0	0	0	0080FFFF	0	0	0
docker0	000011AC	00000000	0001	0	0	0	0000FFFF	0	0	0
virbr0	007AA8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`
const gatewaylast = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
docker0	000011AC	00000000	0001	0	0	0	0000FFFF	0	0	0
virbr0	007AA8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
eth3	0000FE0A	00000000	0001	0	0	0	0080FFFF	0	0	0
eth3	00000000	0100FE0A	0003	0	0	1024	00000000	0	0	0
`
const gatewaymiddle = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth3	0000FE0A	00000000	0001	0	0	0	0080FFFF	0	0	0
docker0	000011AC	00000000	0001	0	0	0	0000FFFF	0	0	0
eth3	00000000	0100FE0A	0003	0	0	1024	00000000	0	0	0
virbr0	007AA8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`
const noInternetConnection = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
docker0	000011AC	00000000	0001	0	0	0	0000FFFF	0	0	0
virbr0	007AA8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`
const nothing = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
`
const route_Invalidhex = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth3	00000000	0100FE0AA	0003	0	0	1024	00000000	0	0	0
eth3	0000FE0A	00000000	0001	0	0	0	0080FFFF	0	0	0
docker0	000011AC	00000000	0001	0	0	0	0000FFFF	0	0	0
virbr0	007AA8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`

func TestParseIP(t *testing.T) {
	testCases := []struct {
		tcase    string
		ip       string
		success  bool
		expected net.IP
	}{
		{"empty", "", false, nil},
		{"too short", "AA", false, nil},
		{"too long", "0011223344", false, nil},
		{"invalid", "invalid!", false, nil},
		{"zero", "00000000", true, net.IP{0, 0, 0, 0}},
		{"ffff", "FFFFFFFF", true, net.IP{0xff, 0xff, 0xff, 0xff}},
		{"valid", "12345678", true, net.IP{120, 86, 52, 18}},
	}
	for _, tc := range testCases {
		ip, err := parseIP(tc.ip)
		if !ip.Equal(tc.expected) {
			t.Errorf("case[%v]: expected %q, got %q . err : %v", tc.tcase, tc.expected, ip, err)
		}
	}
}

func TestIsInterfaceUp(t *testing.T) {
	testCases := []struct {
		tcase    string
		intf     net.Interface
		expected bool
	}{
		{"up", net.Interface{Index: 0, MTU: 0, Name: "eth3", HardwareAddr: nil, Flags: net.FlagUp}, true},
		{"down", net.Interface{Index: 0, MTU: 0, Name: "eth3", HardwareAddr: nil, Flags: 0}, false},
		{"nothing", net.Interface{}, false},
	}
	for _, tc := range testCases {
		it := isInterfaceUp(&tc.intf)
		if it != tc.expected {
			t.Errorf("case[%v]: expected %v, got %v .", tc.tcase, tc.expected, it)
		}
	}
}

type addrStruct struct{ val string }

func (a addrStruct) Network() string {
	return a.val
}
func (a addrStruct) String() string {
	return a.val
}

type validNetworkInterface struct {
}

func (_ validNetworkInterface) InterfaceByName(intfName string) (*net.Interface, error) {
	c := net.Interface{Index: 0, MTU: 0, Name: "eth3", HardwareAddr: nil, Flags: net.FlagUp}
	return &c, nil
}
func (_ validNetworkInterface) Addrs(intf *net.Interface) ([]net.Addr, error) {
	var ifat []net.Addr
	ifat = []net.Addr{
		addrStruct{val: "fe80::2f7:6fff:fe6e:2956/64"}, addrStruct{val: "10.254.71.145/17"}}
	return ifat, nil
}

type noNetworkInterface struct {
}

func (_ noNetworkInterface) InterfaceByName(intfName string) (*net.Interface, error) {
	return nil, fmt.Errorf("unable get Interface")
}
func (_ noNetworkInterface) Addrs(intf *net.Interface) ([]net.Addr, error) {
	return nil, nil
}

type networkInterfacewithNoAddrs struct {
}

func (_ networkInterfacewithNoAddrs) InterfaceByName(intfName string) (*net.Interface, error) {
	c := net.Interface{Index: 0, MTU: 0, Name: "eth3", HardwareAddr: nil, Flags: net.FlagUp}
	return &c, nil
}
func (_ networkInterfacewithNoAddrs) Addrs(intf *net.Interface) ([]net.Addr, error) {
	return nil, fmt.Errorf("unable get Addrs")
}

func TestGetIPFromInterface(t *testing.T) {
	testCases := []struct {
		tcase    string
		nwname   string
		nw       networkInterfacer
		expected net.IP
	}{
		{"valid", "eth3", validNetworkInterface{}, net.ParseIP("10.254.71.145")},
		{"nothing", "eth3", noNetworkInterface{}, nil},
	}
	for _, tc := range testCases {
		ip, err := getIPFromInterface(tc.nwname, tc.nw)
		if !ip.Equal(tc.expected) {
			t.Errorf("case[%v]: expected %v, got %+v .err : %v", tc.tcase, tc.expected, ip, err)
		}
	}
}

func TestChooseHostInterfaceFromRoute(t *testing.T) {
	testCases := []struct {
		tcase    string
		inFile   io.Reader
		nw       networkInterfacer
		expected net.IP
	}{
		{"valid_routefirst", strings.NewReader(gatewayfirst), validNetworkInterface{}, net.ParseIP("10.254.71.145")},
		{"valid_routelast", strings.NewReader(gatewaylast), validNetworkInterface{}, net.ParseIP("10.254.71.145")},
		{"valid_routemiddle", strings.NewReader(gatewaymiddle), validNetworkInterface{}, net.ParseIP("10.254.71.145")},
	}
	for _, tc := range testCases {
		intf, err := chooseHostInterfaceFromRoute(tc.inFile, tc.nw)
		ip, err := getIPFromInterface(intf.Name, tc.nw)
		if !ip.Equal(tc.expected) {
			t.Errorf("case[%v]: expected %v, got %+v .err : %v", tc.tcase, tc.expected, ip, err)
		}
	}
}
