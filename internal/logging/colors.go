package logging

var (
	ansiRed     = []byte("\033[31m")
	ansiGreen   = []byte("\033[32m")
	ansiYellow  = []byte("\033[33m")
	ansiBlue    = []byte("\033[34m")
	ansiMagenta = []byte("\033[35m")
	ansiCyan    = []byte("\033[36m")
	ansiWhite   = []byte("\033[37m")

	ansiBoldRed     = []byte("\033[1;31m")
	ansiBoldGreen   = []byte("\033[1;32m")
	ansiBoldYellow  = []byte("\033[1;33m")
	ansiBoldBlue    = []byte("\033[1;34m")
	ansiBoldMagenta = []byte("\033[1;35m")
	ansiBoldCyan    = []byte("\033[1;36m")
	ansiBoldWhite   = []byte("\033[1;37m")
	
	ansiReset   = []byte("\033[0m")
)
