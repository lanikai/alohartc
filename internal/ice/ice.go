package ice

import (
	"flag"

	"github.com/lanikai/alohartc/internal/config"
	"github.com/lanikai/alohartc/internal/logging"
)

const defaultStunServer = "stun2.l.google.com:19302"

var (
	// Whether or not to allow IPv6 ICE candidates
	flagEnableIPv6 bool

	// Host:port of STUN server
	flagStunServer string

	// Whether or not to enable loopback ICE candidates
	flagEnableLoopback bool
)

var log = logging.DefaultLogger.WithTag("ice")

func init() {
	flag.BoolVar(&flagEnableIPv6, "ipv6", true, "Allow IPv6 ICE candidates")
	flag.StringVar(&flagStunServer, "stunServer", config.STUN_SERVER, "STUN server address")
	flag.BoolVar(&flagEnableLoopback, "loopback", false, "Enable loopback ICE candidates")
}
