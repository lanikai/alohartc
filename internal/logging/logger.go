package logging

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Logger struct {
	// The level at which this logger logs. Any log messages intended for a higher
	// (more verbose) log level are ignored.
	Level

	Tag string

	out io.Writer

	// Mutex to prevent messages from different goroutines from interleaving.
	// Shared by all derived loggers.
	mu *sync.Mutex

	// TODO: Support tee'ing to other loggers.
	//children []*Logger
}

// Expose this when we allow child loggers (i.e. tee'ing).
//func NewLogger(tag string, out io.Writer) *Logger {
//	return &Logger{determineLevel(tag), tag, out, new(sync.Mutex)}
//}

// Write to stderr by default.
var DefaultLogger = &Logger{defaultLevel, "", os.Stderr, new(sync.Mutex)}

// Override the destination for this logger.
func (log *Logger) SetDestination(out io.Writer) {
	log.out = out
}

// Derive a new logger with the given tag. Look up the level based on the tag.
func (log *Logger) WithTag(tag string) *Logger {
	// TODO: Make sure tag doesn't contain special characters.
	return &Logger{determineLevel(tag, log.Level), tag, log.out, log.mu}
}

// Derive a new logger with the given default level. This can still be overridden at
// runtime.
func (log *Logger) WithDefaultLevel(level Level) *Logger {
	return &Logger{determineLevel(log.Tag, level), log.Tag, log.out, log.mu}
}

var newline = []byte{'\n'}

// Log a message at the given level. Include the file and line number from
// 'calldepth' steps up the call stack.
func (log *Logger) Log(level Level, calldepth int, format string, a ...interface{}) {
	if level > log.Level {
		// Message is too verbose for this logger.
		return
	}

	now := time.Now()

	// Get the caller of Error()/Warn()/Info()/etc.
	_, file, line, ok := runtime.Caller(calldepth + 1)
	if !ok {
		file = "?"
	}
	// Truncate full path to just the filename.
	finalSlash := strings.LastIndexByte(file, os.PathSeparator)
	file = file[finalSlash+1:]

	// Lock to prevent log messages from interleaving.
	log.mu.Lock()
	defer log.mu.Unlock()

	// Write timestamp, e.g. "2019-01-25 04:14:10.523"
	log.Write(ansiWhite)
	if ts, err := now.Round(time.Millisecond).MarshalText(); err != nil {
		panic("Invalid time conversion: " + err.Error())
	} else {
		ts[10] = ' '  // Replace 'T' with a space for readability, as per RFC 3339
		ts = ts[0:23] // Strip timezone offset
		log.Write(ts)
	}

	// Write level, tag, file, and line number.
	fmt.Fprintf(log, " %s%c/%s[%s:%d] ", level.color(), level.letter(), log.Tag, file, line)

	// Write formatted log message.
	fmt.Fprintf(log, format, a...)

	// Append newline if necessary.
	lf := len(format)
	if lf == 0 || format[lf-1] != '\n' {
		log.Write(newline)
	}

	log.Write(ansiReset)
}

// Implement io.Writer but panic on error, because if we're unable to log then
// we have no other way of surfacing an error.
func (log *Logger) Write(b []byte) (n int, err error) {
	n, err = log.out.Write(b)
	if err != nil {
		panic(fmt.Sprintf("Failed to log to %v: %v", log.out, err))
	}
	return
}

func (log *Logger) Error(v ...interface{}) {
	log.Log(Error, 1, fmt.Sprint(v...))
}

// go:inline
func (log *Logger) Errorf(format string, a ...interface{}) {
	log.Log(Error, 1, format, a...)
}

func (log *Logger) Warn(v ...interface{}) {
	log.Log(Warn, 1, fmt.Sprint(v...))
}

func (log *Logger) Warnf(format string, a ...interface{}) {
	log.Log(Warn, 1, format, a...)
}

func (log *Logger) Info(v ...interface{}) {
	log.Log(Info, 1, fmt.Sprint(v...))
}

func (log *Logger) Infof(format string, a ...interface{}) {
	log.Log(Info, 1, format, a...)
}

func (log *Logger) Debug(v ...interface{}) {
	log.Log(Debug, 1, fmt.Sprint(v...))
}

func (log *Logger) Debugf(format string, a ...interface{}) {
	log.Log(Debug, 1, format, a...)
}

func (log *Logger) Trace(n int, format string, a ...interface{}) {
	log.Log(Level(n), 1, format, a...)
}
