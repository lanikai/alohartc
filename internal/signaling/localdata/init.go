package localdata

// Bundle static files with the binary, using github.com/mjibson/esc
//go:generate go run github.com/mjibson/esc -o fs.go -pkg localdata -ignore \.go$ .
