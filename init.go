package alohartc

import "github.com/lanikailabs/alohartc/internal/logging"

var log *logging.Logger

func init() {
	log = logging.DefaultLogger.WithTag("alohartc")
}
