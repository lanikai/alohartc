package main

import (
	"fmt"

	"github.com/fatih/color"
	flag "github.com/spf13/pflag"
)

var (
	flagEnableIPv6     bool
	flagSTUNAddress    string
	flagBitrate        int
	flagInput          string
	flagHeight         int
	flagWidth          int
	flagHorizontalFlip bool
	flagVerticalFlip   bool
	flagHelp           bool
	flagVersion        bool
)

func init() {
	flag.IntVarP(&flagBitrate, "bitrate", "b", 1000, "Video bitrate, in KiB")
	flag.StringVarP(&flagInput, "input", "i", "/dev/video0", "Video source")
	flag.IntVarP(&flagHeight, "height", "y", 720, "Video width")
	flag.IntVarP(&flagWidth, "width", "x", 1280, "Video height")
	flag.BoolVarP(&flagHorizontalFlip, "hflip", "", false, "Flip horizontally")
	flag.BoolVarP(&flagVerticalFlip, "vflip", "", false, "Flip vertically")

	flag.BoolVarP(&flagHelp, "help", "h", false, "Print usage information and exit")
	flag.BoolVarP(&flagVersion, "version", "v", false, "Print version information and exit")
}

const helpString = `Real-time video communication for connected devices

Usage: alohartcd [OPTION]...

Authentication:
  -c, --certificate=FILE Client certificate (default: /etc/alohartcd/cert.pem)
  -k, --private-key=FILE Client private key (default: /etc/alohartcd/key.pem)

Network:
  -6, --enable-ipv6      Permit use of IPv6 (default: disabled)
  -m, --mqtt-address=URI MQTT broker address (default: mqtt.alohartc.com:8883)
  -s, --stun-address=URI STUN server address (default: turn.alohartc.com:3478)

Video source:
  -b, --bitrate=NUM      Set a fixed video bitrate, in KiB (default: 1000)
  -i, --input=FILE       Video source (default: /dev/video0)
  -x, --width=NUM        Set video width (default: 1280)
  -y, --height=NUM       Set video height (default: 720)
      --hflip            Flip video horizontally
      --vflip            Flip video vertically

Miscellaneous:
  -h, --help             Prints this help message and exits
  -v, --version          Prints version information and exits

Please report bugs to: aloha@lanikailabs.com
AlohaRTC home page: https://alohartc.com`

// Help information is printed and program exits
func help() {
	r := color.New(color.FgRed)
	y := color.New(color.FgYellow)
	b := color.New(color.FgCyan)

	//         _         _                   _
	//   __ _ | |  ___  | |__    __ _  _ __ | |_  ___
	//  / _` || | / _ \ | '_ \  / _` || '__|| __|/ __|
	// | (_| || || (_) || | | || (_| || |   | |_| (__
	//  \__,_||_| \___/ |_| |_| \__,_||_|    \__|\___|

	// Line 1
	r.Printf("        ")
	y.Printf(" _ ")
	b.Printf("       ")
	y.Printf(" _     ")
	r.Printf("       ")
	y.Printf("      ")
	b.Printf(" _  ")
	y.Println("     ")

	// Line 2
	r.Printf("   __ _ ")
	y.Printf("| |")
	b.Printf("  ___  ")
	y.Printf("| |__  ")
	r.Printf("  __ _ ")
	y.Printf(" _ __ ")
	b.Printf("| |_ ")
	y.Println(" ___ ")

	// Line 3
	r.Printf("  / _` |")
	y.Printf("| |")
	b.Printf(" / _ \\ ")
	y.Printf("| '_ \\ ")
	r.Printf(" / _` |")
	y.Printf("| '__|")
	b.Printf("| __|")
	y.Println("/ __|")

	// Line 4
	r.Printf(" | (_| |")
	y.Printf("| |")
	b.Printf("| (_) |")
	y.Printf("| | | |")
	r.Printf("| (_| |")
	y.Printf("| |   ")
	b.Printf("| |_")
	y.Println("| (__ ")

	// Line 5
	r.Printf("  \\__,_|")
	y.Printf("|_|")
	b.Printf(" \\___/ ")
	y.Printf("|_| |_|")
	r.Printf(" \\__,_|")
	y.Printf("|_|   ")
	b.Printf(" \\__|")
	y.Println("\\___|")

	fmt.Println(helpString)
}
