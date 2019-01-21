package ice

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

const defaultStunServer = "stun2.l.google.com:19302"

var (
	// Whether or not to allow IPv6 ICE candidates
	flagEnableIPv6 bool

	// Host:port of STUN server
	flagStunServer string

	traceEnabled = false
)

func init() {
	flag.BoolVar(&flagEnableIPv6, "6", false, "Allow use of IPv6")
	flag.StringVar(&flagStunServer, "stunServer", defaultStunServer, "STUN server address")

	for _, tag := range strings.Split(os.Getenv("TRACE"), ",") {
		if tag == "ice" {
			traceEnabled = true
			break
		}
	}
}

func trace(format string, a ...interface{}) {
	if !traceEnabled {
		return
	}

	format = "[ice] " + format
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	log.Output(2, fmt.Sprintf(format, a...))
}
