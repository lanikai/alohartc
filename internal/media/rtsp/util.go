// +build rtsp !release

package rtsp

import (
	"errors"
	"net/url"
	"strings"
)

func ParseURL(rawurl string) (*url.URL, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "rtsp" {
		return nil, errors.New("invalid RTSP URL: " + rawurl)
	}

	if strings.IndexByte(u.Host, ':') < 0 {
		// Add default RTSP port to the host.
		u.Host += ":554"
	}

	return u, nil
}
