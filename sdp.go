package webrtc

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// Implements (in part or in full) the following specifications:
// - RFC 4566 (https://tools.ietf.org/html/rfc4566)
// - RFC 3264 (https://tools.ietf.org/html/rfc3264)
// - https://tools.ietf.org/html/draft-ietf-mmusic-ice-sip-sdp-21

type SessionDesc struct {
	version int
	origin OriginDesc
	name string
	info string  // Optional
	uri string  // Optional
	email string  // Optional
	phone string  // Optional
	connection *ConnectionDesc  // Optional
//	bandwidth []string
	time []TimeDesc
//	timezone string  // Optional
//	encryptionKey string  // Optional
	attributes []AttributeDesc
	media []MediaDesc
}

type OriginDesc struct {
	username string
	sessionId string
	sessionVersion uint64
	networkType string
	addressType string
	address string
}

type ConnectionDesc struct {
	networkType string
	addressType string
	address string
}

type TimeDesc struct {
	start *time.Time
	stop *time.Time  // Optional
//	repeat []string
}

type AttributeDesc struct {
	key string
	value string
}

type MediaDesc struct {
	typ string
	port int
	proto string
	format []string

	info string  // Optional
	connection *ConnectionDesc  // Optional
//	bandwidth []string
//	encryptionKey string  // Optional
	attributes []AttributeDesc
}


type Desc interface {
	fmt.Stringer
}

type sdpBuilder struct {
	s strings.Builder
}

func (b *sdpBuilder) add(desc Desc) {
	b.s.WriteString(desc.String())
	b.s.WriteString("\r\n")
}

func (b *sdpBuilder) addLine(format string, args ...interface{}) {
	fmt.Fprintf(&b.s, format, args...)
	b.s.WriteString("\r\n")
}

func (b *sdpBuilder) String() string {
	return b.s.String()
}


func (o OriginDesc) String() string {
	return fmt.Sprintf("o=%s %s %d %s %s %s",
		o.username, o.sessionId, o.sessionVersion, o.networkType, o.addressType, o.address)
}

func parseOrigin(line string) (o OriginDesc, err error) {
	_, err = fmt.Sscanf(line, "o=%s %s %d %s %s %s",
		&o.username, &o.sessionId, &o.sessionVersion, &o.networkType, &o.addressType, &o.address)
	return
}


func (c ConnectionDesc) String() string {
	return fmt.Sprintf("c=%s %s %s", c.networkType, c.addressType, c.address)
}

func parseConnection(line string) (c *ConnectionDesc, err error) {
	c = new(ConnectionDesc)
	_, err = fmt.Sscanf(line, "c=%s %s %s", &c.networkType, &c.addressType, &c.address)
	return
}


func (t TimeDesc) String() string {
	return fmt.Sprintf("t=%d %d", toNtp(t.start), toNtp(t.stop))
}

func parseTime(line string) (t TimeDesc, err error) {
	var start, stop int64
	_, err = fmt.Sscanf(line, "t=%d %d", &start, &stop)
	t.start = fromNtp(start)
	t.stop = fromNtp(stop)
	return
}

// Difference between NTP timestamps (measure from 1900) and Unix timestamps (measured from 1970).
const ntpOffset = 2208988800

func toNtp(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.Unix() + ntpOffset
}

func fromNtp(ntp int64) *time.Time {
	if ntp == 0 {
		return nil
	}
	t := time.Unix(ntp - ntpOffset, 0)
	return &t
}


func (a AttributeDesc) String() string {
	return fmt.Sprintf("a=%s:%s", a.key, a.value)
}

func parseAttribute(line string) (a AttributeDesc, err error) {
	_, err = fmt.Sscanf(line, "a=%s:%s", &a.key, &a.value)
	return
}

func (m MediaDesc) String() string {
	var b sdpBuilder
	b.addLine("m=%s %d %s %s", m.typ, m.port, m.proto, strings.Join(m.format, " "))
	if m.info != "" {
		b.addLine("i=%s", m.info)
	}
	if m.connection != nil {
		b.add(m.connection)
	}
	for _, a := range m.attributes {
		b.add(a)
	}
	return b.String()
}

// Returns the remaining unparsed SDP string as 'remainder'.
func parseMedia(sdp string) (m MediaDesc, remainder string, err error) {
	line, remainder := nextLine(sdp)
	if line[0:2] != "m=" {
		err = fmt.Errorf("Invalid media line: %s", line)
		return
	}
	fields := strings.Fields(line[2:])
	if len(fields) < 3 {
		err = fmt.Errorf("Invalid media line: %s", line)
		return
	}

	m.typ = fields[0]
	m.port, err = strconv.Atoi(fields[1])
	m.proto = fields[2]
	m.format = fields[3:]

	for ; sdp != ""; sdp = remainder {
		line, remainder = nextLine(sdp)
		var typecode byte
		var value string
		typecode, value, err = splitTypeValue(line)
		switch typecode {
		case 'm':
			break
		case 'i':
			m.info = value
		case 'c':
			m.connection, err = parseConnection(line)
		case 'a':
			var a AttributeDesc
			a, err = parseAttribute(line)
			m.attributes = append(m.attributes, a)
		}

		if err != nil {
			log.Printf("Failed to parse media description: %s", line)
			break
		}
	}
	remainder = sdp
	return
}


func (s *SessionDesc) String() string {
	var b sdpBuilder
	b.addLine("v=%d", s.version)
	b.add(s.origin)
	b.addLine("s=%s", s.name)
	if s.info != "" {
		b.addLine("i=%s", s.info)
	}
	if s.uri != "" {
		b.addLine("u=%s", s.uri)
	}
	if s.email != "" {
		b.addLine("e=%s", s.email)
	}
	if s.phone != "" {
		b.addLine("p=%s", s.phone)
	}
	if s.connection != nil {
		b.add(s.connection)
	}
	for _, t := range s.time {
		b.add(t)
	}
	for _, a := range s.attributes {
		b.add(a)
	}
	for _, m := range s.media {
		b.add(m)
	}
	return b.String()
}

func parseSession(sdp string) (s SessionDesc, remainder string, err error) {
	var line string
	for ; sdp != ""; sdp = remainder {
		line, remainder = nextLine(sdp)
		var typecode byte
		var value string
		typecode, value, err = splitTypeValue(line)
		switch typecode {
		case 'v':
			s.version, err = strconv.Atoi(value)
		case 'o':
			s.origin, err = parseOrigin(line)
		case 's':
			s.name = value
		case 'i':
			s.info = value
		case 'u':
			s.uri = value
		case 'e':
			s.email = value
		case 'p':
			s.phone = value
		case 'c':
			s.connection, err = parseConnection(line)
		case 't':
			var t TimeDesc
			t, err = parseTime(line)
			s.time = append(s.time, t)
		case 'a':
			var a AttributeDesc
			a, err = parseAttribute(line)
			s.attributes = append(s.attributes, a)
		case 'm':
			var m MediaDesc
			m, remainder, err = parseMedia(sdp)
			s.media = append(s.media, m)
		}

		if err != nil {
			log.Printf("Failed to parse session description: %s", line)
			break
		}
	}
	remainder = sdp
	return
}

func nextLine(input string) (line string, remainder string) {
	n := strings.IndexByte(input, '\n')
	if n == -1 {
		line = input
	} else {
		if n > 0 && input[n-1] == '\r' {
			// Leave off the carriage return.
			line = input[:n-1]
		} else {
			line = input[:n]
		}
		remainder = input[n+1:]
	}
	return
}

func splitTypeValue(line string) (typecode byte, value string, err error) {
	if len(line) < 2 || line[1] != '=' {
		err = fmt.Errorf("Invalid SDP line: %s", line)
		return
	}
	typecode = line[0]
	value = line[2:]
	return
}
