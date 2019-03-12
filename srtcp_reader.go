// Copyright (c) 2019 Lanikai Labs. All rights reserved.

package alohartc

import (
	"github.com/lanikai/alohartc/internal/mux"
	"github.com/lanikai/alohartc/internal/rtcp"
	"github.com/lanikai/alohartc/internal/srtp"
)

// srtcpReaderRunloop handles incoming SRTCP packets, namely reception reports.
// Expected to run as a separate goroutine. Exits when the mux is closed.
func srtcpReaderRunloop(m *mux.Mux, key, salt []byte) error {
	buffer := make([]byte, maxSRTCPSize)

	// Create new endpoint for SRTCP packets
	endpoint := m.NewEndpoint(mux.MatchSRTCP)
	defer endpoint.Close()

	// Create cipher context
	ctx, err := srtp.CreateContext(key, salt)
	if err != nil {
		return err
	}

	for {
		// Blocks on reading packet from endpoint
		if n, err := endpoint.Read(buffer); err != nil {
			return err // Endpoint closed. Exit loop.
		} else {
			rawPacket := buffer[:n]

			// Decipher in-place
			if _, err := ctx.DecipherRTCP(rawPacket, rawPacket); err != nil {
				log.Error(err) // Error deciphering. Skip.
				continue
			}

			// Parse
			if packet, _, err := rtcp.Unmarshal(rawPacket); err != nil {
				log.Error(err) // Malformed packet
				continue
			} else {
				switch p := packet.(type) {
				case *rtcp.ReceiverReport:
					log.Debug(p.String()) // Print packet, for now
				default:
					break
				}
			}
		}
	}

	return nil // Should never get here
}
