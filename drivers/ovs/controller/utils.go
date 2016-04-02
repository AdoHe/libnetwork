package controller

import (
	"net"
)

func getHostIP() string {
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:1")
	if err != nil {
		return ""
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return ""
	}

	defer conn.Close()

	host, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return ""
	}

	return host
}
