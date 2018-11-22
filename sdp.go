package webrtc

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Implements (in part or in full) the following specifications:
// - RFC 4566 (https://tools.ietf.org/html/rfc4566)
// - RFC 3264 (https://tools.ietf.org/html/rfc3264)
// - https://tools.ietf.org/html/draft-ietf-mmusic-ice-sip-sdp-21

type SessionDesc struct {
	version    int
	origin     OriginDesc
	name       string
	info       string          // Optional
	uri        string          // Optional
	email      string          // Optional
	phone      string          // Optional
	connection *ConnectionDesc // Optional
	//	bandwidth []string
	time []TimeDesc
	//	timezone string  // Optional
	//	encryptionKey string  // Optional
	attributes []AttributeDesc
	media      []MediaDesc

	// Initialized on first call to GetAttr()
	attributeCache map[string]string
}

type OriginDesc struct {
	username       string
	sessionId      string
	sessionVersion uint64
	networkType    string
	addressType    string
	address        string
}

type ConnectionDesc struct {
	networkType string
	addressType string
	address     string
}

type TimeDesc struct {
	start *time.Time
	stop  *time.Time // Optional
	//	repeat []string
}

type AttributeDesc struct {
	key   string
	value string
}

type MediaDesc struct {
	typ    string
	port   int
	proto  string
	format []string

	info       string          // Optional
	connection *ConnectionDesc // Optional
	//	bandwidth []string
	//	encryptionKey string  // Optional
	attributes []AttributeDesc

	// Initialized on first call to GetAttr()
	attributeCache map[string]string
}

type Desc interface {
	String() string
}

type SdpWriter strings.Builder

func (w *SdpWriter) Write(fragments ...string) {
	for _, s := range fragments {
		(*strings.Builder)(w).WriteString(s)
	}
}

func (w *SdpWriter) Writef(format string, args ...interface{}) {
	fmt.Fprintf((*strings.Builder)(w), format, args...)
}

func (w *SdpWriter) String() string {
	return (*strings.Builder)(w).String()
}

type sdpParseError struct {
	which string
	value string
	cause error
}

func (e *sdpParseError) Error() (msg string) {
	msg = fmt.Sprintf("SDP parser: Invalid %s description: %q", e.which, e.value)
	if e.cause != nil {
		msg += "\nCaused by: " + e.cause.Error()
	}
	return
}

func (o *OriginDesc) String() string {
	return fmt.Sprintf("%s %s %d %s %s %s",
		o.username, o.sessionId, o.sessionVersion, o.networkType, o.addressType, o.address)
}

func parseOrigin(s string) (o OriginDesc, err error) {
	_, err = fmt.Sscanf(s, "%s %s %d %s %s %s",
		&o.username, &o.sessionId, &o.sessionVersion, &o.networkType, &o.addressType, &o.address)
	if err != nil {
		err = &sdpParseError{"origin", s, err}
	}
	return
}

func (c *ConnectionDesc) String() string {
	return fmt.Sprintf("%s %s %s", c.networkType, c.addressType, c.address)
}

func parseConnection(s string) (c ConnectionDesc, err error) {
	_, err = fmt.Sscanf(s, "%s %s %s", &c.networkType, &c.addressType, &c.address)
	if err != nil {
		err = &sdpParseError{"connection", s, err}
	}
	return
}

func (t TimeDesc) String() string {
	return fmt.Sprintf("%d %d", toNtp(t.start), toNtp(t.stop))
}

func parseTime(s string) (t TimeDesc, err error) {
	var start, stop int64
	_, err = fmt.Sscanf(s, "%d %d", &start, &stop)
	t.start = fromNtp(start)
	t.stop = fromNtp(stop)
	if err != nil {
		err = &sdpParseError{"time", s, err}
	}
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
	t := time.Unix(ntp-ntpOffset, 0)
	return &t
}

func (a AttributeDesc) String() string {
	if a.value == "" {
		return a.key
	}
	return fmt.Sprintf("%s:%s", a.key, a.value)
}

func parseAttribute(s string) (a AttributeDesc, err error) {
	f := strings.SplitN(s, ":", 2)
	a.key = f[0]
	if len(f) == 2 {
		a.value = f[1]
	} else {
		a.value = ""
	}
	return
}

func (m *MediaDesc) GetAttr(key string) string {
	if m.attributeCache == nil {
		m.attributeCache = make(map[string]string)
		for _, a := range m.attributes {
			m.attributeCache[a.key] = a.value
		}
	}
	return m.attributeCache[key]
}

func (m *MediaDesc) String() string {
	var w SdpWriter
	w.Writef("m=%s %d %s %s\r\n", m.typ, m.port, m.proto, strings.Join(m.format, " "))
	if m.info != "" {
		w.Write("i=", m.info, "\r\n")
	}
	if m.connection != nil {
		w.Write("c=", m.connection.String(), "\r\n")
	}
	for _, a := range m.attributes {
		w.Write("a=", a.String(), "\r\n")
	}
	return w.String()
}

// Returns the remaining unparsed SDP text as 'rtext'.
func parseMedia(text string) (m MediaDesc, rtext string, err error) {
	line, more := nextLine(text)
	if line[0:2] != "m=" {
		return m, text, fmt.Errorf("Invalid media line: %s", line)
	}

	fields := strings.Fields(line[2:])
	if len(fields) < 3 {
		return m, text, fmt.Errorf("Invalid media line: %s", line)
	}
	m.typ = fields[0]
	m.port, err = strconv.Atoi(fields[1])
	m.proto = fields[2]
	m.format = fields[3:]

	var typecode byte
	var value string
	for text = more; text != ""; text = more {
		line, more = nextLine(text)
		typecode, value, err = splitTypeValue(line)
		switch typecode {
		case 'm':
			break
		case 'i':
			m.info = value
		case 'c':
			var c ConnectionDesc
			c, err = parseConnection(value)
			m.connection = &c
		case 'a':
			var a AttributeDesc
			a, err = parseAttribute(value)
			m.attributes = append(m.attributes, a)
		}

		if err != nil {
			err = &sdpParseError{"media", line, err}
			break
		}
	}
	return m, text, err
}

func (s *SessionDesc) GetAttr(key string) string {
	if s.attributeCache == nil {
		s.attributeCache = make(map[string]string)
		for _, a := range s.attributes {
			s.attributeCache[a.key] = a.value
		}
	}
	return s.attributeCache[key]
}

func (s *SessionDesc) GetMedia() *MediaDesc {
	if len(s.media) != 1 {
		return nil // TODO: should be an error
	}
	return &s.media[0]
}

func (s *SessionDesc) String() string {
	var w SdpWriter
	w.Writef("v=%d\r\n", s.version)
	w.Write("o=", s.origin.String(), "\r\n")
	w.Write("s=", s.name, "\r\n")
	if s.info != "" {
		w.Write("i=", s.info, "\r\n")
	}
	if s.uri != "" {
		w.Write("u=", s.uri, "\r\n")
	}
	if s.email != "" {
		w.Write("e=", s.email, "\r\n")
	}
	if s.phone != "" {
		w.Write("p=", s.phone, "\r\n")
	}
	if s.connection != nil {
		w.Write("c=", s.connection.String(), "\r\n")
	}
	for _, t := range s.time {
		w.Write("t=", t.String(), "\r\n")
	}
	for _, a := range s.attributes {
		w.Write("a=", a.String(), "\r\n")
	}
	for _, m := range s.media {
		w.Write(m.String())
	}
	return w.String()
}

func parseSession(text string) (s SessionDesc, err error) {
	var typecode byte
	var line, more, value string
	for ; text != ""; text = more {
		line, more = nextLine(text)
		typecode, value, err = splitTypeValue(line)
		switch typecode {
		case 'v':
			s.version, err = strconv.Atoi(value)
		case 'o':
			s.origin, err = parseOrigin(value)
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
			var c ConnectionDesc
			c, err = parseConnection(value)
			s.connection = &c
		case 't':
			var t TimeDesc
			t, err = parseTime(value)
			s.time = append(s.time, t)
		case 'a':
			var a AttributeDesc
			a, err = parseAttribute(value)
			s.attributes = append(s.attributes, a)
		case 'm':
			var m MediaDesc
			m, more, err = parseMedia(text)
			s.media = append(s.media, m)
		}

		if err != nil {
			return s, &sdpParseError{"session", line, err}
			break
		}
	}
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
