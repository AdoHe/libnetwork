package util

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
)

func getHostName() string {
	var hostname string
	name, _ := exec.Command("uname", "-n").Output()
	hostname = string(name)
	return strings.ToLower(strings.TrimSpace(hostname))
}

type Route struct {
	Interface   string
	Destination net.IP
	Gateway     net.IP
}

func getRoutes(input io.Reader) ([]Route, error) {
	routes := []Route{}
	if input == nil {
		return nil, fmt.Errorf("input is nil")
	}
	scanner := bufio.NewReader(input)
	for {
		line, err := scanner.ReadString('\n')
		if err == io.EOF {
			break
		}
		//ignore the headers in the route info
		if strings.HasPrefix(line, "Iface") {
			continue
		}
		fields := strings.Fields(line)
		routes = append(routes, Route{})
		route := &routes[len(routes)-1]
		route.Interface = fields[0]
		ip, err := parseIP(fields[1])
		if err != nil {
			return nil, err
		}
		route.Destination = ip
		ip, err = parseIP(fields[2])
		if err != nil {
			return nil, err
		}
		route.Gateway = ip
	}
	return routes, nil
}

func parseIP(str string) (net.IP, error) {
	if str == "" {
		return nil, fmt.Errorf("input is nil")
	}
	bytes, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}
	if len(bytes) != net.IPv4len {
		return nil, fmt.Errorf("only IPv4 is supported")
	}
	bytes[0], bytes[1], bytes[2], bytes[3] = bytes[3], bytes[2], bytes[1], bytes[0]
	return net.IP(bytes), nil
}

func isInterfaceUp(intf *net.Interface) bool {
	if intf == nil {
		return false
	}
	if intf.Flags&net.FlagUp != 0 {
		return true
	}
	return false
}

//getFinalIP method receives all the IP addrs of a Interface
//and returns a nil if the address is Loopback, Ipv6, link-local or nil.
//It returns a valid IPv4 if an Ipv4 address is found in the array.
func getFinalIP(addrs []net.Addr) (net.IP, error) {
	if len(addrs) > 0 {
		for i := range addrs {
			ip, _, err := net.ParseCIDR(addrs[i].String())
			if err != nil {
				return nil, err
			}
			//Only IPv4
			if ip.To4() != nil {
				if !ip.IsLoopback() && !ip.IsLinkLocalMulticast() && !ip.IsLinkLocalUnicast() {
					return ip, nil
				}
			}
		}
	}
	return nil, nil
}

func getIPFromInterface(intfName string, nw networkInterfacer) (net.IP, error) {
	intf, err := nw.InterfaceByName(intfName)
	if err != nil {
		return nil, err
	}
	if isInterfaceUp(intf) {
		addrs, err := nw.Addrs(intf)
		if err != nil {
			return nil, err
		}
		finalIP, err := getFinalIP(addrs)
		if err != nil {
			return nil, err
		}
		if finalIP != nil {
			return finalIP, nil
		}
	}

	return nil, nil
}

func flagsSet(flags net.Flags, test net.Flags) bool {
	return flags&test != 0
}

func flagsClear(flags net.Flags, test net.Flags) bool {
	return flags&test == 0
}

func chooseHostInterfaceNativeGo() (*net.Interface, error) {
	intfs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	i := 0
	var ip net.IP
	var intf *net.Interface
	for i = range intfs {
		if flagsSet(intfs[i].Flags, net.FlagUp) && flagsClear(intfs[i].Flags, net.FlagLoopback|net.FlagPointToPoint) {
			addrs, err := intfs[i].Addrs()
			if err != nil {
				return nil, err
			}
			if len(addrs) > 0 {
				for _, addr := range addrs {
					if addrIP, _, err := net.ParseCIDR(addr.String()); err == nil {
						if addrIP.To4() != nil {
							ip = addrIP.To4()
							if !ip.IsLinkLocalMulticast() && !ip.IsLinkLocalUnicast() {
								intf = &intfs[i]
								break
							}
						}
					}
				}
				if ip != nil && intf != nil {
					// This interface should suffice.
					break
				}
			}
		}
	}
	if intf == nil {
		return nil, fmt.Errorf("no acceptable interface from host")
	}
	return intf, nil
}

//ChooseHostInterface is a method used fetch an interface for a daemon.
//It uses data from /proc/net/route file.
//For a node with no internet connection ,it returns error
//For a multi n/w interface node it returns the interface with gateway on it.
func ChooseHostInterface() (*net.Interface, error) {
	inFile, err := os.Open("/proc/net/route")
	if err != nil {
		if os.IsNotExist(err) {
			return chooseHostInterfaceNativeGo()
		}
		return nil, err
	}
	defer inFile.Close()
	var nw networkInterfacer = networkInterface{}
	return chooseHostInterfaceFromRoute(inFile, nw)
}

// GetHostDefaultGW is a method to retrieve host default gw address.
func GetHostDefaultGW() (net.IP, error) {
	inFile, err := os.Open("/proc/net/route")
	if err != nil {
		return nil, err
	}
	defer inFile.Close()
	routes, err := getRoutes(inFile)
	if err != nil {
		return nil, err
	}
	zero := net.IP{0, 0, 0, 0}
	for i := range routes {
		if routes[i].Destination.Equal(zero) {
			return routes[i].Gateway, nil
		}
	}
	return nil, fmt.Errorf("cannot find the gw route")
}

// GetAddrFromInterface is a method to retrieve addr info from network interface
func GetAddrFromInterface(intfName string) (net.Addr, error) {
	var nw networkInterfacer = networkInterface{}
	intf, err := nw.InterfaceByName(intfName)
	if err != nil {
		return nil, err
	}
	if isInterfaceUp(intf) {
		addrs, err := nw.Addrs(intf)
		if err != nil {
			return nil, err
		}
		if len(addrs) > 0 {
			for i := range addrs {
				ip, _, err := net.ParseCIDR(addrs[i].String())
				if err != nil {
					return nil, err
				}
				//Only IPv4
				if ip.To4() != nil {
					if !ip.IsLoopback() && !ip.IsLinkLocalMulticast() && !ip.IsLinkLocalUnicast() {
						return addrs[i], nil
					}
				}
			}
		}
	}
	return nil, nil
}

type networkInterfacer interface {
	InterfaceByName(intfName string) (*net.Interface, error)
	Addrs(intf *net.Interface) ([]net.Addr, error)
}

type networkInterface struct{}

func (_ networkInterface) InterfaceByName(intfName string) (*net.Interface, error) {
	intf, err := net.InterfaceByName(intfName)
	if err != nil {
		return nil, err
	}
	return intf, nil
}

func (_ networkInterface) Addrs(intf *net.Interface) ([]net.Addr, error) {
	addrs, err := intf.Addrs()
	if err != nil {
		return nil, err
	}
	return addrs, nil
}

func chooseHostInterfaceFromRoute(inFile io.Reader, nw networkInterfacer) (*net.Interface, error) {
	routes, err := getRoutes(inFile)
	if err != nil {
		return nil, err
	}
	zero := net.IP{0, 0, 0, 0}
	for i := range routes {
		if routes[i].Destination.Equal(zero) {
			intf, err := nw.InterfaceByName(routes[i].Interface)
			if err != nil {
				return nil, err
			}
			if intf != nil {
				return intf, nil
			}
		}
	}
	return nil, nil
}
