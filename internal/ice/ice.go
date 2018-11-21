package ice

import (
	"fmt"
	"log"
	"os"
	"strings"
)

var traceEnabled = false

func init() {
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
