package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const timestampFormat = "2006-01-02 15:04:05.000"

type Logger struct {
	// The level at which this logger logs. Any log messages intended for a higher
	// (more verbose) log level are ignored.
	Level

	// Tag used to filter and classify log messages.
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

// Wrapper for []byte that implements io.Writer. Simpler and cheaper than
// bytes.Buffer.
type buffer []byte

func (b *buffer) Write(p []byte) (int, error) {
	*b = append(*b, p...)
	return len(p), nil
}

func (b *buffer) writeString(s string) {
	*b = append(*b, s...)
}

func (b *buffer) writeByte(c byte) {
	*b = append(*b, c)
}

// A global buffer pool, shared across all loggers. Initial length is 256 to
// accommodate *most* log lines.
var bufPool = sync.Pool{
	New: func() interface{} {
		return make(buffer, 256)
	},
}

// Log a message at the given level. Include the file and line number from
// 'calldepth' steps up the call stack.
func (log *Logger) Log(level Level, calldepth int, format string, a ...interface{}) {
	if level > log.Level {
		// Message is too verbose for this logger.
		return
	}

	// Grab an empty buffer from the pool.
	buf := bufPool.Get().(buffer)
	// When we're done, reset the buffer and return it to the pool.
	defer bufPool.Put(buf[:0])

	buf.Write(ansiWhite)

	// Write the current timestamp.
	buf = time.Now().AppendFormat(buf, timestampFormat)

	// Write level and tag.
	fmt.Fprintf(&buf, " %s%c/%s", level.color(), level.letter(), log.Tag)

	// Get the caller of Error()/Warn()/Info()/etc.
	_, file, line, ok := runtime.Caller(calldepth + 1)
	if !ok {
		file = "?"
	}

	// Write file and line number.
	fmt.Fprintf(&buf, "[%s:%d] %s", filepath.Base(file), line, ansiReset)

	// Write formatted log message.
	fmt.Fprintf(&buf, format, a...)

	// Append newline if necessary.
	if n := len(format); n == 0 || format[n-1] != '\n' {
		buf.writeByte('\n')
	}

	// Lock before writing to avoid interleaving of log messages.
	log.mu.Lock()
	if _, err := log.out.Write(buf); err != nil {
		panic(fmt.Sprintf("Failed to log to %v: %v", log.out, err))
	}
	log.mu.Unlock()
}

func (log *Logger) Error(format string, a ...interface{}) {
	log.Log(Error, 1, format, a...)
}

func (log *Logger) Warn(format string, a ...interface{}) {
	log.Log(Warn, 1, format, a...)
}

func (log *Logger) Info(format string, a ...interface{}) {
	log.Log(Info, 1, format, a...)
}

func (log *Logger) Debug(format string, a ...interface{}) {
	log.Log(Debug, 1, format, a...)
}

func (log *Logger) Trace(n int, format string, a ...interface{}) {
	log.Log(Level(n), 1, format, a...)
}
