// +build rtsp

package rtsp

import (
	"fmt"
	"net"
	"strings"
)

type Transport struct {
	RTP  *net.UDPConn
	RTCP *net.UDPConn

	SSRC uint32
	Mode string
}

func (tr *Transport) Close() error {
	if tr.RTP != nil {
		tr.RTP.Close()
	}
	if tr.RTCP != nil {
		tr.RTCP.Close()
	}
	return nil
}

func NewTransport() (*Transport, error) {
	even, odd, err := bindUDPPair()
	if err != nil {
		return nil, err
	}

	return &Transport{
		RTP:  even,
		RTCP: odd,
	}, nil
}

func (tr *Transport) ClientHeader() string {
	rtpPort := getPort(tr.RTP.LocalAddr())
	rtcpPort := getPort(tr.RTCP.LocalAddr())
	return fmt.Sprintf("RTP/AVP/UDP;unicast;client_port=%d-%d", rtpPort, rtcpPort)
}

func (tr *Transport) Header() string {
	s := tr.ClientHeader()

	rtpServerPort := getPort(tr.RTP.RemoteAddr())
	rtcpServerPort := getPort(tr.RTCP.RemoteAddr())
	if rtpServerPort > 0 && rtcpServerPort > 0 {
		s += fmt.Sprintf(";server_port=%d-%d", rtpServerPort, rtcpServerPort)
	}
	if tr.SSRC != 0 {
		s += fmt.Sprintf(";ssrc=%08X", tr.SSRC)
	}
	if tr.Mode != "" {
		s += ";mode=" + tr.Mode
	}
	return s
}

func (tr *Transport) parseServerResponse(transportHeader string, serverIP net.IP) error {
	// See https://tools.ietf.org/html/rfc2326#section-12.39
	var spec string
	params := make(map[string]string)
	for i, s := range strings.Split(transportHeader, ";") {
		if i == 0 {
			spec = s
		} else {
			name, value := split2(s, '=')
			params[name] = value
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
		// Parse server RTP-RTCP port.
		rtpPort, rtcpPort := split2(serverPort, '-')
		if rtcpPort == "" {
			return fmt.Errorf("invalid server_port value: %s", serverPort)
		}

		rtpServerAddr, err := net.ResolveUDPAddr("udp4", source+":"+rtpPort)
		if err != nil {
			return err
		}
		if tr.RTP, err = rebindUDP(tr.RTP, rtpServerAddr); err != nil {
			return err
		}

		rtcpServerAddr, err := net.ResolveUDPAddr("udp4", source+":"+rtcpPort)
		if err != nil {
			return err
		}
		if tr.RTCP, err = rebindUDP(tr.RTCP, rtcpServerAddr); err != nil {
			return err
		}
	}

	fmt.Sscanf(params["ssrc"], "%x", &tr.SSRC)
	tr.Mode = strings.ToUpper(params["mode"])

	return nil
}

// Split a string into 2 parts, separated by c.
func split2(s string, c byte) (string, string) {
	i := strings.IndexByte(s, c)
	if i < 0 {
		return s, ""
	}
	return s[0:i], s[i+1:]
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

// Rebind a listening UDPConn to a remote address.
func rebindUDP(c *net.UDPConn, raddr *net.UDPAddr) (*net.UDPConn, error) {
	laddr := c.LocalAddr().(*net.UDPAddr)
	c.Close()
	return net.DialUDP("udp4", laddr, raddr)
}

func getPort(addr net.Addr) int {
	switch a := addr.(type) {
	case *net.UDPAddr:
		return a.Port
	case *net.TCPAddr:
		return a.Port
	default:
		return 0
	}
}
