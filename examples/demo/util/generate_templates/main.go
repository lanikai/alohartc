// Generates Go source containing templates from specified directory as string
// literal constants. Intended to be called via go:generate build tag.

package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var flagOutputFilename string

func init() {
	flag.StringVar(&flagOutputFilename, "o", "", "Output filename")
}

func main() {
	flag.Parse()

	// Check arguments
	if flag.NArg() != 1 {
		log.Fatal("Too many arguments or no input directory specified")
	}

	// Read list of files in specified directory
	templates, err := ioutil.ReadDir(flag.Arg(0))
	if err != nil {
		panic(err)
	}

	// If no output file specific, append go onto directory name
	if len(flagOutputFilename) == 0 {
		flagOutputFilename = filepath.Base(flag.Arg(0)) + ".go"
	}

	// Create file for writing
	out, err := os.Create(flagOutputFilename)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	// Write header
	out.Write([]byte("package main\n\nconst (\n"))

	// Copy templates to file. Each {name}.tmpl gets assigned to a const {name}.
	for _, template := range templates {
		if strings.HasSuffix(template.Name(), ".tmpl") {
			out.Write([]byte(strings.TrimSuffix(template.Name(), ".tmpl") + " = `"))
			in, err := os.Open(path.Join(flag.Arg(0), template.Name()))
			if err != nil {
				panic(err)
			}
			defer in.Close()
			io.Copy(out, in)
			out.Write([]byte("`\n"))
		}
	}

	// Write footer
	out.Write([]byte(")\n"))
}
