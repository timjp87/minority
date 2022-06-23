package netutil

import (
	"net"

	"github.com/ethereum/go-ethereum/log"
)

// externalAddress iterates over all the network interfaces of the machine and
// returns the first non-loopback one (or the loopback if none can be found).
func ExternalAddress() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Crit("Failed to retrieve network interfaces", "err", err)
	}
	for _, iface := range ifaces {
		// Skip disconnected and loopback interfaces
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			log.Warn("Failed to retrieve network addresses", "err", err)
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				return v.IP.String()
			case *net.IPAddr:
				return v.IP.String()
			}
		}
	}
	return "127.0.0.1"
}
