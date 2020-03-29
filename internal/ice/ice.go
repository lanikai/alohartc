package ice

import (
	flag "github.com/spf13/pflag"

	"github.com/lanikai/alohartc/internal/config"
	"github.com/lanikai/alohartc/internal/logging"
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
	flag.BoolVarP(&flagEnableIPv6, "enable-ipv6", "6", true, "Allow IPv6 ICE candidates")
	flag.StringVarP(&flagStunServer, "stun-address", "s", config.STUN_SERVER, "STUN server address")
}

// Start background services necessary for ICE.
func Start() error {
	if err := mdnsStart(); err != nil {
		return err
	}

	return nil
}

func Stop() {
	mdnsStop()
}
