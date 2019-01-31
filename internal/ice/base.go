package ice

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/lanikai/alohartc/internal/mux"
)

// [RFC8445] defines a base to be "The transport address that an ICE agent sends from for a
// particular candidate." It is represented here by a UDP connection, listening on a single port.
type Base struct {
	*net.UDPConn
	address   TransportAddress
	component int

	// STUN response handlers for transactions sent from this base, keyed by transaction ID.
	transactions map[string]stunHandler

	transactionsLock sync.Mutex
}

type stunHandler func(msg *stunMessage, addr net.Addr, base *Base)

// Create a base for each local IP address.
func establishBases(component int) (bases []*Base, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, iface := range ifaces {
		log.Debug("Interface %d: %s (%s)\n", iface.Index, iface.Name, iface.Flags)
		if iface.Flags&net.FlagLoopback != 0 {
			// Skip loopback interfaces to reduce the number of candidates.
			// TODO: Probably we need these if we're not connected to any network.
			continue
		}
		var addrs []net.Addr
		addrs, err = iface.Addrs()
		if err != nil {
			return
		}
		for _, addr := range addrs {
			log.Debug("Local address %v", addr)
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				log.Panicf("Unexpected address type: %T", addr)
			}

			ip := ipnet.IP
			if !flagEnableIPv6 {
				if ip4 := ip.To4(); ip4 == nil {
					// Must be an IPv6 address. Skip it.
					continue
				}
			}

			base, err := createBase(ip, component)
			if err != nil {
				log.Warn("Failed to create base for %s\n", ip)
				// This can happen for link-local IPv6 addresses. Just skip it.
				continue
			}
			bases = append(bases, base)
		}
	}
	return
}

func createBase(ip net.IP, component int) (base *Base, err error) {
	// Listen on an arbitrary UDP port.
	listenAddr := &net.UDPAddr{IP: ip, Port: 0}
	conn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return
	}

	address := makeTransportAddress(conn.LocalAddr())
	log.Info("Listening on %s\n", address)

	transactions := make(map[string]stunHandler)
	base = &Base{conn, address, component, transactions, sync.Mutex{}}
	return
}

// Return the server-reflexive address of this base.
func (base *Base) queryStunServer(stunServer string) (mapped TransportAddress, err error) {
	network := fmt.Sprintf("udp%d", base.address.family)
	stunServerAddr, err := net.ResolveUDPAddr(network, stunServer)
	if err != nil {
		return
	}

	req := newStunBindingRequest("")
	log.Debug("Sending to %s: %s\n", stunServer, req)

	done := make(chan error, 1)
	err = base.sendStun(req, stunServerAddr, func(resp *stunMessage, raddr net.Addr, base *Base) {
		if resp.class == stunSuccessResponse {
			mapped = makeTransportAddress(resp.getMappedAddress())
			done <- nil
		} else {
			done <- fmt.Errorf("STUN server query failed: %s", resp)
		}
	})
	if err != nil {
		return
	}

	select {
	case err = <-done:
	case <-time.After(3 * time.Second):
		err = fmt.Errorf("Timed out waiting for response from %s", stunServer)
	}
	return
}

// Send a STUN message to the given remote address. If a handler is supplied, it will be used to
// process the STUN response, based on the transaction ID.
func (base *Base) sendStun(msg *stunMessage, raddr net.Addr, handler stunHandler) error {
	_, err := base.WriteTo(msg.Bytes(), raddr)
	if err == nil && handler != nil {
		base.transactionsLock.Lock()
		base.transactions[msg.transactionID] = handler
		base.transactionsLock.Unlock()
	}
	return err
}

// Read continuously from the connection. STUN messages go to handlers, other data to dataIn.
func (base *Base) demuxStun(defaultHandler stunHandler, dataIn chan<- []byte) {
	buf := make([]byte, 4096)
	for {
		base.SetReadDeadline(time.Now().Add(60 * time.Second))
		n, raddr, err := base.ReadFrom(buf)
		if err == io.EOF {
			log.Warn("Connection closed: %s\n", base.address)
			return
		} else if err != nil {
			if nerr, ok := err.(net.Error); ok {
				if nerr.Timeout() {
					// Timeout is expected for bases that end up not being used.
					log.Info("Connection timed out: %s\n", base.address)
					return
				}
			}
			log.Fatal(err)
		}
		data := make([]byte, n)
		copy(data, buf[0:n])

		// Only process STUN messages
		if mux.MatchSTUN(data) {
			msg, err := parseStunMessage(data)
			if err != nil {
				log.Fatal(err)
			}

			if msg != nil {
				log.Debug("Received from %s: %s\n", raddr, msg)

				// Pass incoming STUN message to the appropriate handler.
				var handler stunHandler
				base.transactionsLock.Lock()
				handler, found := base.transactions[msg.transactionID]
				if found {
					delete(base.transactions, msg.transactionID)
				} else {
					handler = defaultHandler
				}
				base.transactionsLock.Unlock()
				handler(msg, raddr, base)
			}
		} else {
			// Not a STUN packet, forward it on
			select {
			case dataIn <- data:
			default:
				//log.Debug("Warning: Data discarded")
			}
		}
	}
}
