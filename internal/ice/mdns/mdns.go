package mdns

// This package implements the RTCWeb mdns-ice-candidates proposal for using
// ephemeral Multicast DNS hostnames to avoid exposing sensitive IP addresses.
// See https://tools.ietf.org/html/draft-ietf-rtcweb-mdns-ice-candidates-04

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/lanikai/alohartc/internal/logging"
)

var log = logging.DefaultLogger.WithTag("mdns")

// Global client instance.
var _client *Client

// Initialize global mDNS client.
func Start() error {
	if _client != nil {
		return errors.New("mdns: global client already started")
	}

	c, err := NewClient()
	if err != nil {
		return err
	}

	_client = c
	return nil
}

func Stop() {
	checkStarted()
	_client.Close()
	_client = nil
}

func Resolve(ctx context.Context, name string) (net.IP, error) {
	checkStarted()
	return _client.Resolve(ctx, name)
}

func Announce(ctx context.Context, name string, ip net.IP, ttl time.Duration) error {
	checkStarted()
	return _client.Announce(ctx, name, ip, ttl)
}

func checkStarted() {
	if _client == nil {
		panic("mdns: global client never started")
	}
}
