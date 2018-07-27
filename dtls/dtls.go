package dtls

import (
	"net"
	"strings"
	"time"
)


// Client returns a new TLS client side connection
// using conn as the underlying transport.
// The config cannot be nil: users must set either ServerName or
// InsecureSkipVerify in the config.
func Client(conn net.Conn, config *Config) *Conn {
	return &Conn{conn: conn, config: config, isClient: true}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "tls: DialWithDialer timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func DialWithConnection(rawConn net.Conn) (*Conn, error) {
//	sendClientHello(conn)

	var err error

	// We want the Timeout and Deadline values from dialer to cover the
	// whole process: TCP connection and TLS handshake. This means that we
	// also need to start our own timers now.
	timeout := 60 * time.Second

//	if !dialer.Deadline.IsZero() {
//		deadlineTimeout := time.Until(dialer.Deadline)
//		if timeout == 0 || deadlineTimeout < timeout {
//			timeout = deadlineTimeout
//		}
//	}

	var errChannel chan error

	if timeout != 0 {
		errChannel = make(chan error, 2)
		time.AfterFunc(timeout, func() {
			errChannel <- timeoutError{}
		})
	}

//	rawConn, err := dialer.Dial(network, addr)
//	if err != nil {
//		return nil, err
//	}

	addr := rawConn.LocalAddr().String()
	var config *Config

	colonPos := strings.LastIndex(addr, ":")
	if colonPos == -1 {
		colonPos = len(addr)
	}
	hostname := addr[:colonPos]

	if config == nil {
		config = defaultConfig()
	}
	// If no ServerName is set, infer the ServerName
	// from the hostname we're connecting to.
	if config.ServerName == "" {
		// Make a copy to avoid polluting argument or default.
		c := config.Clone()
		c.ServerName = hostname
		config = c
	}

	// WebRTC certificate chain and host name are not set by browser(s)
	config.InsecureSkipVerify = true
	config.MinVersion = VersionDTLS12
	conn := Client(rawConn, config)

	if timeout == 0 {
		err = conn.Handshake()
	} else {
		go func() {
			errChannel <- conn.Handshake()
		}()

		err = <-errChannel
	}

	if err != nil {
		rawConn.Close()
		return nil, err
	}

	return conn, nil
}
