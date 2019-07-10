// Copyright 2019 Lanikai Labs LLC. All rights reserved.

package sdp

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

type H264FormatParameters struct {
	LevelAsymmetryAllowed bool
	PacketizationMode     int
	PayloadType           int
	ProfileLevelID        int
	SpropParameterSets    [][]byte
}

type PCMUFormatParameters struct {
}

// Marshal format parameters to string
func (fmtp *H264FormatParameters) Marshal() string {
	format := []string{
		fmt.Sprintf("profile-level-id=%06x", fmtp.ProfileLevelID),
	}

	if fmtp.LevelAsymmetryAllowed {
		format = append(format, "level-asymmetry-allowed=1")
	}

	if fmtp.PacketizationMode > 0 {
		format = append(format, fmt.Sprintf("packetization-mode=%d", fmtp.PacketizationMode))
	}

	if len(fmtp.SpropParameterSets) > 0 {
		var encoded []string
		for _, ps := range fmtp.SpropParameterSets {
			encoded = append(encoded, base64.StdEncoding.EncodeToString(ps))
		}
		format = append(format, fmt.Sprintf("sprop-parameter-sets=%s", strings.Join(encoded, ",")))
	}

	return strings.Join(format, ";")
}

// Unmarshal format parameters from string
func (fmtp *H264FormatParameters) Unmarshal(format string) error {
	errMalformedFormatParameters := errors.New("malformed format parameters")

	for _, param := range strings.Split(format, ";") {
		pieces := strings.SplitN(param, "=", 2)
		if len(pieces) < 2 {
			return errMalformedFormatParameters
		}

		switch pieces[0] {
		case "level-asymmetry-allowed":
			switch pieces[1] {
			case "0":
				fmtp.LevelAsymmetryAllowed = false
			case "1":
				fmtp.LevelAsymmetryAllowed = true
			default:
				return errMalformedFormatParameters
			}
		case "packetization-mode":
			switch pieces[1] {
			case "0":
				fmtp.PacketizationMode = 0
			case "1":
				fmtp.PacketizationMode = 1
			case "2":
				fmtp.PacketizationMode = 2
			default:
				return errMalformedFormatParameters
			}
		case "profile-level-id":
			if _, err := fmt.Sscanf(pieces[1], "%06x", &fmtp.ProfileLevelID); err != nil {
				return errMalformedFormatParameters
			}
		case "sprop-parameter-sets":
			encoded := strings.Split(pieces[1], ",")
			for _, e := range encoded {
				ps, err := base64.StdEncoding.DecodeString(e)
				if err != nil {
					return errMalformedFormatParameters
				}
				fmtp.SpropParameterSets = append(fmtp.SpropParameterSets, ps)
			}
		}
	}

	return nil
}
