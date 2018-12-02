package ice

import (
	"io"
	"log"
	"net"
	"time"
)

// [RFC8445] defines a base to be "The transport address that an ICE agent sends from for a
// particular candidate." It is represented here by a UDP connection, listening on a single port.
type Base struct {
	*net.UDPConn
	address   TransportAddress
	component int

	// STUN response handlers for transactions sent from this base, keyed by transaction ID.
	transactions map[string]stunHandler
}

type stunHandler func(msg *stunMessage, addr net.Addr, base Base)

func createBase(component int) (base Base, err error) {
	localIP, err := getLocalIP()
	if err != nil {
		return
	}

	// Listen on an arbitrary UDP port.
	listenAddr := &net.UDPAddr{IP: localIP, Port: 0}
	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		return
	}

	transactions := make(map[string]stunHandler)
	base = Base{conn, makeTransportAddress(conn.LocalAddr()), component, transactions}
	return
}

// Get the IP address of this machine.
func getLocalIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP, nil
}

// Send a STUN message to the given remote address. If a handler is supplied, it will be used to
// process the STUN response, based on the transaction ID.
func (base *Base) sendStun(msg *stunMessage, raddr net.Addr, handler stunHandler) error {
	_, err := base.WriteTo(msg.Bytes(), raddr)
	if err == nil && handler != nil {
		base.transactions[msg.transactionID] = handler
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
			log.Printf("Connection closed")
			return
		} else if err != nil {
			log.Fatal(err)
		}
		data := buf[0:n]

		msg, err := parseStunMessage(data)
		if err != nil {
			log.Fatal(err)
		}

		if msg != nil {
			trace("Received from %s: %s\n", raddr, msg)

			// Pass incoming STUN message to the appropriate handler.
			if handler, found := base.transactions[msg.transactionID]; found {
				delete(base.transactions, msg.transactionID)
				handler(msg, raddr, *base)
			} else {
				defaultHandler(msg, raddr, *base)
			}
		} else {
			select {
			case dataIn <- data:
			default:
				//trace("Warning: Data discarded")
			}
		}
	}
}
