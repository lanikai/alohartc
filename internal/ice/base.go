package ice

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/lanikai/alohartc/internal/mux"
)

const (
	// Packets larger than the maximum transmission unit (MTU) of a path are
	// fragmented into smaller packets, or dropped. The MTU should be
	// discovered, but 1500 is typically a safe value.
	sizeMaximumTransmissionUnit = 1500

	// Timeout for querying STUN server.
	timeoutQuerySTUNServer = 5 * time.Second

	// Timeout for reads from base (i.e. its UDPConn).
	// STUN re-bindings sent every 2500ms on Safari
	timeoutReadFromBase = 5 * time.Second
)

// [RFC8445] defines a base to be "The transport address that an ICE agent sends from for a
// particular candidate." It is represented here by a UDP connection, listening on a single port.
type Base struct {
	net.PacketConn

	address   TransportAddress
	component int
	sdpMid    string

	// STUN response handlers for transactions sent from this base, keyed by transaction ID.
	handlers transactionHandlers

	// Single-fire channel used to indicate that the read loop has died.
	dead chan struct{}

	// Error that caused the read loop to terminate.
	err error
}

type stunHandler func(msg *stunMessage, addr net.Addr, base *Base)

// Create a base for each local IP address.
func initializeBases(component int, sdpMid string) (bases []*Base, err error) {
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

		// Skip down interfaces
		if iface.Flags&net.FlagUp == 0 {
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
				log.Error("Unexpected address type: %T", addr)
			}

			ip := ipnet.IP
			if !flagEnableIPv6 {
				if ip4 := ip.To4(); ip4 == nil {
					// Not an IPv4 address -- skip it
					continue
				}
			}

			base, err := createBase(ip, component, sdpMid)
			if err != nil {
				// This can happen for link-local IPv6 addresses. Just skip it.
				log.Debug("Failed to create base for %s\n", ip)
				continue
			}
			bases = append(bases, base)
		}
	}
	return
}

func createBase(ip net.IP, component int, sdpMid string) (*Base, error) {
	// Listen on an arbitrary UDP port.
	listenAddr := &net.UDPAddr{IP: ip, Port: 0}
	conn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return nil, err
	}

	address := makeTransportAddress(conn.LocalAddr())
	log.Info("Listening on %s\n", address)

	return &Base{
		PacketConn: conn,
		address:    address,
		component:  component,
		sdpMid:     sdpMid,
	}, nil
}

// Gather host and server-reflexive candidates for each base. Blocks until
// gathering is complete.
func gatherAllCandidates(ctx context.Context, pt *PriorityTable, bases []*Base, take func(c Candidate)) {
	var wg sync.WaitGroup
	for _, b := range bases {
		wg.Add(1)
		go func(base *Base) {
			base.gatherCandidates(ctx, pt, take)
			wg.Done()
		}(b)
	}
	wg.Wait()
}

// Gather candidates host and server-reflexive candidates for this base.
func (base *Base) gatherCandidates(ctx context.Context, pt *PriorityTable, take func(c Candidate)) {
	log.Debug("Gathering local candidates for base %s\n", base.address)
	// Host candidate for peers on the same LAN.
	take(makeHostCandidate(pt, base))

	if base.address.protocol == UDP && !base.address.linkLocal {
		// Query STUN server to get a server reflexive candidate.
		mappedAddress, err := base.queryStunServer(ctx, flagStunServer)

		// If the context ended, ignore the error.
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err != nil {
			log.Debug("Failed to create STUN server candidate for base %s: %s\n", base.address, err)
		} else if mappedAddress == base.address {
			log.Debug("Server-reflexive address for %s is same as base\n", base.address)
		} else {
			take(makeServerReflexiveCandidate(pt, base, mappedAddress, flagStunServer))
		}
	}
}

// Return the server-reflexive address of this base.
func (base *Base) queryStunServer(ctx context.Context, stunServer string) (mapped TransportAddress, err error) {
	network := fmt.Sprintf("udp%d", base.address.family)
	stunServerAddr, err := net.ResolveUDPAddr(network, stunServer)
	if err != nil {
		return
	}

	req := newStunBindingRequest("")
	log.Debug("Sending to %s: %s\n", stunServer, req)

	// TODO: Handle retransmissions.
	errCh := make(chan error, 1)
	err = base.sendStun(req, stunServerAddr, func(resp *stunMessage, raddr net.Addr, base *Base) {
		if resp.class == stunSuccessResponse {
			mapped = makeTransportAddress(resp.getMappedAddress())
			errCh <- nil
		} else {
			errCh <- fmt.Errorf("STUN server query failed: %s", resp)
		}
	})
	if err != nil {
		return
	}

	select {
	case err = <-errCh:
	case <-ctx.Done():
		err = ctx.Err()
	case <-time.After(timeoutQuerySTUNServer):
		err = errors.New("timeout")
	}

	base.handlers.remove(req.transactionID)
	return
}

// Send a STUN message to the given remote address. If a handler is supplied, it will be used to
// process the STUN response, based on the transaction ID.
func (base *Base) sendStun(msg *stunMessage, raddr net.Addr, responseHandler stunHandler) error {
	_, err := base.WriteTo(msg.Bytes(), raddr)
	if err == nil && responseHandler != nil {
		base.handlers.put(msg.transactionID, responseHandler)
	}
	return err
}

// Read incoming packets from the underlying PacketConn, until an error occurs.
// STUN messages are handled, the rest are sent to the dataIn channel.
func (base *Base) readLoop(defaultHandler stunHandler, dataIn chan []byte) {
	if base.dead != nil {
		panic("Base read loop already started")
	}

	base.dead = make(chan struct{})
	defer close(base.dead)

	// Single packet read buffer.
	buf := make([]byte, sizeMaximumTransmissionUnit)

	var logOnce sync.Once
	for {
		// Set read timeout
		base.SetReadDeadline(time.Now().Add(timeoutReadFromBase))

		// Blocks (or timeouts) waiting for packet from underlying UDPConn
		n, raddr, err := base.ReadFrom(buf)

		if err != nil {
			if neterr, ok := err.(net.Error); ok {
				// Timeout is expected for bases that are not selected.
				if neterr.Timeout() {
					log.Debug("Connection timed out: %s\n", base.address)
					base.err = errReadTimeout
					break
				}

				// Temporary glitch? Try to continue.
				if neterr.Temporary() {
					continue
				}
			}

			// Exit cleanly if the underlying PacketConn was closed.
			if operr, ok := err.(*net.OpError); ok {
				if operr.Op == "read" {
					log.Debug("Connection closed while reading: %s\n", base.address)
					break
				}
			}

			log.Warn("Read error in %s: %v\n", base.address, err)
			base.err = err
			break
		}

		// TODO: Use a sync.Pool of buffers to avoid allocating on each packet.
		data := make([]byte, n)
		copy(data, buf[0:n])

		if mux.MatchSTUN(data) {
			// Process STUN packets.
			msg, err := parseStunMessage(data)
			if err != nil {
				log.Fatal(err)
			}

			if msg != nil {
				log.Debug("Received from %s: %s\n", raddr, msg)

				// Pass incoming STUN message to the appropriate handler.
				handler := base.handlers.get(msg.transactionID, defaultHandler)
				handler(msg, raddr, base)
			}
		} else {
			// Pass data packets (non-STUN) to the dataIn channel.
			select {
			case dataIn <- data:
			default:
				logOnce.Do(func() {
					log.Warn("Dropping data packet (first byte %x) because reader cannot keep up", data[0])
				})
			}
		}
	}
}

// transactionHandlers manages a map of STUN transaction ID -> stunHandler. When an
// outgoing STUN request is made, a handler can be registered for processing the
// remote peer's STUN response.
type transactionHandlers struct {
	sync.Mutex
	m map[string]stunHandler
}

func (t *transactionHandlers) get(transactionID string, def stunHandler) stunHandler {
	t.lockAndInitialize()
	handler, found := t.m[transactionID]
	if found {
		delete(t.m, transactionID)
	} else {
		handler = def
	}
	t.Unlock()
	return handler
}

func (t *transactionHandlers) put(transactionID string, handler stunHandler) {
	t.lockAndInitialize()
	t.m[transactionID] = handler
	t.Unlock()
}

func (t *transactionHandlers) remove(transactionID string) {
	t.lockAndInitialize()
	delete(t.m, transactionID)
	t.Unlock()
}

func (t *transactionHandlers) lockAndInitialize() {
	t.Lock()
	if t.m == nil {
		t.m = make(map[string]stunHandler)
	}
}
