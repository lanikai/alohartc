package logging

import (
	"errors"
	"fmt"
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

var levelToName = map[Level]string{
	Error: "Error",
	Warn:  "Warn",
	Info:  "Info",
	Debug: "Debug",
}

func (l Level) String() string {
	if name, ok := levelToName[l]; ok {
		return name
	} else {
		return fmt.Sprintf("Trace(%d)", l)
	}
}

func (l Level) Letter() byte {
	if l <= Debug {
		return "EWID"[l-Error]
	} else {
		// Allow numeric values up to 9
		return byte('0' + l)
	}
}
