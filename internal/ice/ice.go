package ice

import (
	"flag"

	"github.com/lanikailabs/webrtc/internal/logging"
)

const defaultStunServer = "stun2.l.google.com:19302"

var (
	// Whether or not to allow IPv6 ICE candidates
	flagEnableIPv6 bool

	// Host:port of STUN server
	flagStunServer string
)

var log = logging.DefaultLogger.WithTag("ice")

func init() {
	flag.BoolVar(&flagEnableIPv6, "6", false, "Allow use of IPv6")
	flag.StringVar(&flagStunServer, "stunServer", defaultStunServer, "STUN server address")
}
