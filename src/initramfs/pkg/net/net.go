package net

import (
	"fmt"
	"net"
)

// IP finds and returns the first non-loopback interface of the current machine.
func IP() (ip net.IP, err error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP, nil
			}
		}
	}

	return nil, fmt.Errorf("could not discover IP address")
}
