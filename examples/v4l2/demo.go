// Reads NALU frames from v4l2 device and writes to a file

package main

import (
	"flag"
	"log"
	"os"
	"strconv"

	"github.com/lanikai/alohartc/internal/v4l2"
)

var (
	flagBitrate    string
	flagInputFile  string
	flagNumPackets string
	flagOutputFile string
)

func init() {
	flag.StringVar(&flagBitrate, "b", "2000000", "Bitrate")
	flag.StringVar(&flagInputFile, "i", "/dev/video0", "Input device")
	flag.StringVar(&flagNumPackets, "n", "0", "Number of packets to capture")
	flag.StringVar(&flagOutputFile, "o", "", "Output file")
}

func main() {
	flag.Parse()

	// Default output file
	outputFile := os.Stdout

	// Open
	dev, err := v4l2.Open(flagInputFile, &v4l2.Config{
		Width:  1280,
		Height: 720,
		Format: v4l2.V4L2_PIX_FMT_H264,
	})
	if err != nil {
		panic(err)
	}
	defer dev.Close()

	// Set bitrate
	if bitrate, err := strconv.Atoi(flagBitrate); err != nil {
		panic(err)
	} else {
		dev.SetBitrate(uint(bitrate))
	}

	// Start
	if err := dev.Start(); err != nil {
		panic(err)
	}
	defer dev.Stop()

	// Open file for writing
	if flagOutputFile != "" {
		outputFile, err := os.Create(flagOutputFile)
		if err != nil {
			panic(err)
		}
		defer outputFile.Close()
	}

	// Get number of packets to read for
	numPackets, err := strconv.Atoi(flagNumPackets)
	if err != nil {
		panic(err)
	}

	// Read NALU after NALU
	p := make([]byte, 1000000)
	for i := 0; numPackets == 0 || i < numPackets; i++ {
		if nr, err := dev.Read(p); err != nil {
			panic(err)
		} else {
			if nw, err := outputFile.Write(p[:nr]); err != nil {
				panic(err)
			} else {
				log.Println("read:", nr, "written:", nw)
			}
		}
	}
}
