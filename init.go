package alohartc

import "github.com/lanikai/alohartc/internal/logging"

var log *logging.Logger

func init() {
	log = logging.DefaultLogger.WithTag("alohartc")
}
