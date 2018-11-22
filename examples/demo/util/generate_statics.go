// Generates source code containing static assets from specified directory
// as string literal constants.
//
// Assets are assigned unique, random names. The generated static() function
// returns the constant string. For example:
//
//     static("js/adapter-latest.js")

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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
	inputPath := filepath.Clean(flag.Arg(0))

	// If no output file specific, append go onto directory name
	if len(flagOutputFilename) == 0 {
		flagOutputFilename = filepath.Base(inputPath) + ".go"
	}

	// Create file for writing
	out, err := os.Create(flagOutputFilename)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	// Write header
	out.Write([]byte("package main\n\nconst (\n"))

	// Map for holding statics name-to-const lookups
	staticMap := make(map[string]string)

	// Process specified directory (recursively)
	i := 0
	err = filepath.Walk(inputPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			// Get path relative to input path
			if relPath, err := filepath.Rel(inputPath, path); err != nil {
				log.Fatal(err)
			} else {
				// Create a unique constant name
				constName := fmt.Sprintf("_static_%d", i)
				staticMap[relPath] = constName
				i++

				// Write const to generated source code output
				out.Write([]byte(constName + " = `"))
				if in, err := os.Open(path); err != nil {
					log.Fatal(err)
				} else {
					io.Copy(out, in)
				}
				out.Write([]byte("`\n"))
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// Write const block footer
	out.Write([]byte(")\n\n"))

	// Write static() function
	out.Write([]byte("func static(name string) []byte {\n    switch name {\n"))
	for k, v := range staticMap {
		out.Write([]byte(fmt.Sprintf("    case \"%s\":\n", k)))
		out.Write([]byte(fmt.Sprintf("        return []byte(%s)\n", v)))
	}
	out.Write([]byte("    }\n    return nil\n}\n"))
}
