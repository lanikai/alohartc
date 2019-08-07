package rtsp

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

type Transport struct {
	rtpPort  int
	rtcpPort int

	rtpConn  *net.UDPConn
	rtcpConn *net.UDPConn

	rtpServerAddr  *net.UDPAddr
	rtcpServerAddr *net.UDPAddr

	ssrc uint32
	mode string
}

func (tr *Transport) Close() error {
	if tr.rtpConn != nil {
		tr.rtpConn.Close()
	}
	if tr.rtcpConn != nil {
		tr.rtcpConn.Close()
	}
	return nil
}

func NewTransport() (*Transport, error) {
	even, odd, err := bindUDPPair()
	if err != nil {
		return nil, err
	}

	return &Transport{
		rtpPort:  even.LocalAddr().(*net.UDPAddr).Port,
		rtcpPort: odd.LocalAddr().(*net.UDPAddr).Port,
		rtpConn:  even,
		rtcpConn: odd,
	}, nil
}

func (tr *Transport) ClientHeader() string {
	return fmt.Sprintf("RTP/AVP/UDP;unicast;client_port=%d-%d", tr.rtpPort, tr.rtcpPort)
}

func (tr *Transport) Header() string {
	s := tr.ClientHeader()
	if tr.rtpServerAddr != nil {
		s += fmt.Sprintf(";server_port=%d-%d", tr.rtpServerAddr.Port, tr.rtcpServerAddr.Port)
	}
	return s
}

func (tr *Transport) ParseServerResponse(transportHeader string, serverIP net.IP) error {
	fmt.Println(transportHeader)

	// See https://tools.ietf.org/html/rfc2326#section-12.39
	var spec string
	params := make(map[string]string)
	for i, param := range strings.Split(transportHeader, ";") {
		if i == 0 {
			spec = param
		} else {
			kv := strings.SplitN(param, "=", 2)
			if len(kv) == 2 {
				params[kv[0]] = kv[1]
			} else {
				params[kv[0]] = ""
			}
		}
	}

	if spec != "RTP/AVP/UDP" {
		return fmt.Errorf("unsupported transport spec: %s", spec)
	}
	if _, ok := params["unicast"]; !ok {
		return fmt.Errorf("expected unicast: %s", transportHeader)
	}

	source, ok := params["source"]
	if !ok {
		source = serverIP.String()
	}

	if serverPort, ok := params["server_port"]; ok {
		evenOdd := strings.Split(serverPort, "-")
		if len(evenOdd) != 2 {
			return fmt.Errorf("invalid server_port value: %s", serverPort)
		}
		var err error
		tr.rtpServerAddr, err = net.ResolveUDPAddr("udp", source+":"+evenOdd[0])
		if err != nil {
			return err
		}
		tr.rtcpServerAddr, err = net.ResolveUDPAddr("udp", source+":"+evenOdd[1])
		if err != nil {
			return err
		}
	}

	if ssrc, ok := params["ssrc"]; ok {
		buf, err := hex.DecodeString(ssrc)
		if err != nil {
			return err
		}
		tr.ssrc = binary.BigEndian.Uint32(buf)
	}

	tr.mode = strings.ToUpper(params["mode"])

	return nil
}

// Bind consecutive local UDP ports for RTP and RTCP.
func bindUDPPair() (even, odd *net.UDPConn, err error) {
	for i := 0; i < 20; i++ {
		even, odd, err = tryBindUDPPair()
		if err == nil {
			return
		}
	}

	return nil, nil, fmt.Errorf("failed to bind even/odd port pair: %v", err)
}

func tryBindUDPPair() (even, odd *net.UDPConn, err error) {
	// Bind a random local port.
	conn, err := net.ListenUDP("udp4", new(net.UDPAddr))
	if err != nil {
		return
	}

	// Make a copy of the net.UDPAddr.
	laddr := *conn.LocalAddr().(*net.UDPAddr)

	if laddr.Port%2 == 0 {
		// Randomly assigned port P was even. Use P+1 for the odd port.
		even = conn
		laddr.Port += 1
		odd, err = net.ListenUDP("udp4", &laddr)
	} else {
		// Randomly assigned port P was odd. Use P-1 for the even port.
		odd = conn
		laddr.Port -= 1
		even, err = net.ListenUDP("udp4", &laddr)
	}

	if err != nil {
		// Unbind the first port if we failed to bind the second one.
		conn.Close()
	}
	return
}
