package logging

import (
	"fmt"
	"os"
)

// These are meant purely to ease migrations away from the standard 'log' package.
// Prefer the explicitly leveled API, e.g. log.Error().

func (log *Logger) Fatal(v ...interface{}) {
	log.Log(Error, 1, fmt.Sprint(v...))
	os.Exit(1)
}

func (log *Logger) Fatalf(format string, v ...interface{}) {
	log.Log(Error, 1, format, v...)
	os.Exit(1)
}

func (log *Logger) Fatalln(v ...interface{}) {
	log.Log(Error, 1, fmt.Sprintln(v...))
	os.Exit(1)
}

func (log *Logger) Panic(v ...interface{}) {
	s := fmt.Sprint(v...)
	log.Log(Error, 1, s)
	panic(s)
}

func (log *Logger) Panicf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	log.Log(Error, 1, s)
	panic(s)
}

func (log *Logger) Panicln(v ...interface{}) {
	s := fmt.Sprintln(v...)
	log.Log(Error, 1, s)
	panic(s)
}

func (log *Logger) Print(v ...interface{}) {
	log.Log(Info, 1, fmt.Sprint(v...))
}

func (log *Logger) Printf(format string, v ...interface{}) {
	log.Log(Info, 1, format, v...)
}

func (log *Logger) Println(v ...interface{}) {
	log.Log(Info, 1, fmt.Sprintln(v...))
}
