package logging

import (
	"errors"
	"strconv"
	"strings"
)

// Logging level. Higher values indicate more verbosity.
type Level int

const (
	Error Level = iota - 2
	Warn
	Info
	Debug

	// Allow numeric logging levels up to 9.
	MaxLevel Level = 9
)

// Default level can be changed by environment variable.
var defaultLevel = Info

func parseLevel(s string) (level Level, err error) {
	// First check for well-known level names or abbreviations.
	switch strings.ToUpper(s) {
	case "E", "ERROR":
		return Error, nil
	case "W", "WARN":
		return Warn, nil
	case "I", "INFO":
		return Info, nil
	case "D", "DEBUG":
		return Debug, nil
	case "T", "TRACE":
		return MaxLevel, nil
	}

	// Otherwise expect an explicit numeric level.
	if n, ierr := strconv.Atoi(s); ierr != nil {
		err = errors.New("Invalid logging level: " + s)
	} else {
		level = Level(n)
		if level < Error || level > MaxLevel {
			err = errors.New("Numeric level out of range: " + s)
		}
	}
	return
}

func (l Level) String() string {
	switch l {
	case Error:
		return "Error"
	case Warn:
		return "Warn"
	case Info:
		return "Info"
	case Debug:
		return "Debug"
	default:
		return strconv.Itoa(int(l))
	}
}

func (l Level) letter() byte {
	if l <= Debug {
		return "EWID"[l-Error]
	} else {
		// Numeric values up to 9 are allowed.
		return byte('0' + l)
	}
}

func (l Level) color() []byte {
	switch l {
	case Error:
		return ansiBoldRed
	case Warn:
		return ansiRed
	case Info:
		return ansiReset
	case Debug:
		return ansiGreen
	default:
		return ansiYellow
	}
}
