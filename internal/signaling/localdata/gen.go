// +build !production

package localdata

// Bundle static files with the binary, using github.com/mjibson/esc
//go:generate $GOPATH/bin/esc -o embed.go -ignore \.go$ -pkg localdata .
