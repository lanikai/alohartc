package sdp

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

type Session struct {
	Version    int
	Origin     Origin
	Name       string
	Info       string      // Optional
	Uri        string      // Optional
	Email      string      // Optional
	Phone      string      // Optional
	Connection *Connection // Optional
	//	bandwidth []string
	Time []Time
	//	timezone string  // Optional
	//	encryptionKey string  // Optional
	Attributes []Attribute
	Media      []Media

	// Initialized on first call to GetAttr()
	attributeCache map[string]string
}

type Origin struct {
	Username       string
	SessionId      string
	SessionVersion uint64
	NetworkType    string
	AddressType    string
	Address        string
}

type Connection struct {
	NetworkType string
	AddressType string
	Address     string
}

type Time struct {
	Start *time.Time
	Stop  *time.Time // Optional
	//	repeat []string
}

type Attribute struct {
	Key   string
	Value string
}

type Media struct {
	Type   string
	Port   int
	Proto  string
	Format []string

	Info       string      // Optional
	Connection *Connection // Optional
	//	bandwidth []string
	//	encryptionKey string  // Optional
	Attributes []Attribute

	// Initialized on first call to GetAttr()
	attributeCache map[string]string
}

type writer strings.Builder

func (w *writer) Write(fragments ...string) {
	for _, s := range fragments {
		(*strings.Builder)(w).WriteString(s)
	}
}

func (w *writer) Writef(format string, args ...interface{}) {
	fmt.Fprintf((*strings.Builder)(w), format, args...)
}

func (w *writer) String() string {
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

func (o *Origin) String() string {
	return fmt.Sprintf("%s %s %d %s %s %s",
		o.Username, o.SessionId, o.SessionVersion, o.NetworkType, o.AddressType, o.Address)
}

func parseOrigin(s string) (o Origin, err error) {
	_, err = fmt.Sscanf(s, "%s %s %d %s %s %s",
		&o.Username, &o.SessionId, &o.SessionVersion, &o.NetworkType, &o.AddressType, &o.Address)
	if err != nil {
		err = &sdpParseError{"origin", s, err}
	}
	return
}

func (c *Connection) String() string {
	return fmt.Sprintf("%s %s %s", c.NetworkType, c.AddressType, c.Address)
}

func parseConnection(s string) (c Connection, err error) {
	_, err = fmt.Sscanf(s, "%s %s %s", &c.NetworkType, &c.AddressType, &c.Address)
	if err != nil {
		err = &sdpParseError{"connection", s, err}
	}
	return
}

func (t Time) String() string {
	return fmt.Sprintf("%d %d", toNtp(t.Start), toNtp(t.Stop))
}

func parseTime(s string) (t Time, err error) {
	var start, stop int64
	_, err = fmt.Sscanf(s, "%d %d", &start, &stop)
	t.Start = fromNtp(start)
	t.Stop = fromNtp(stop)
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

func (a Attribute) String() string {
	if a.Value == "" {
		return a.Key
	}
	return fmt.Sprintf("%s:%s", a.Key, a.Value)
}

func parseAttribute(s string) (a Attribute, err error) {
	f := strings.SplitN(s, ":", 2)
	a.Key = f[0]
	if len(f) == 2 {
		a.Value = f[1]
	} else {
		a.Value = ""
	}
	return
}

func (m *Media) GetAttr(key string) string {
	if m.attributeCache == nil {
		m.attributeCache = make(map[string]string)
		for _, a := range m.Attributes {
			m.attributeCache[a.Key] = a.Value
		}
	}
	return m.attributeCache[key]
}

func (m *Media) String() string {
	var w writer
	w.Writef("m=%s %d %s %s\r\n", m.Type, m.Port, m.Proto, strings.Join(m.Format, " "))
	if m.Info != "" {
		w.Write("i=", m.Info, "\r\n")
	}
	if m.Connection != nil {
		w.Write("c=", m.Connection.String(), "\r\n")
	}
	for _, a := range m.Attributes {
		w.Write("a=", a.String(), "\r\n")
	}
	return w.String()
}

// Returns the remaining unparsed SDP text as 'rtext'.
func parseMedia(text string) (m Media, rtext string, err error) {
	line, more := nextLine(text)
	if line[0:2] != "m=" {
		return m, text, fmt.Errorf("Invalid media line: %s", line)
	}

	fields := strings.Fields(line[2:])
	if len(fields) < 3 {
		return m, text, fmt.Errorf("Invalid media line: %s", line)
	}
	m.Type = fields[0]
	m.Port, err = strconv.Atoi(fields[1])
	m.Proto = fields[2]
	m.Format = fields[3:]

	var typecode byte
	var value string
	for text = more; text != ""; text = more {
		line, more = nextLine(text)
		typecode, value, err = splitTypeValue(line)
		switch typecode {
		case 'm':
			break
		case 'i':
			m.Info = value
		case 'c':
			var c Connection
			c, err = parseConnection(value)
			m.Connection = &c
		case 'a':
			var a Attribute
			a, err = parseAttribute(value)
			m.Attributes = append(m.Attributes, a)
		}

		if err != nil {
			err = &sdpParseError{"media", line, err}
			break
		}
	}
	return m, text, err
}

func (s *Session) GetAttr(key string) string {
	if s.attributeCache == nil {
		s.attributeCache = make(map[string]string)
		for _, a := range s.Attributes {
			s.attributeCache[a.Key] = a.Value
		}
	}
	return s.attributeCache[key]
}

func (s *Session) String() string {
	var w writer
	w.Writef("v=%d\r\n", s.Version)
	w.Write("o=", s.Origin.String(), "\r\n")
	w.Write("s=", s.Name, "\r\n")
	if s.Info != "" {
		w.Write("i=", s.Info, "\r\n")
	}
	if s.Uri != "" {
		w.Write("u=", s.Uri, "\r\n")
	}
	if s.Email != "" {
		w.Write("e=", s.Email, "\r\n")
	}
	if s.Phone != "" {
		w.Write("p=", s.Phone, "\r\n")
	}
	if s.Connection != nil {
		w.Write("c=", s.Connection.String(), "\r\n")
	}
	for _, t := range s.Time {
		w.Write("t=", t.String(), "\r\n")
	}
	for _, a := range s.Attributes {
		w.Write("a=", a.String(), "\r\n")
	}
	for _, m := range s.Media {
		w.Write(m.String())
	}
	return w.String()
}

func ParseSession(text string) (s Session, err error) {
	var typecode byte
	var line, more, value string
	for ; text != ""; text = more {
		line, more = nextLine(text)
		typecode, value, err = splitTypeValue(line)
		switch typecode {
		case 'v':
			s.Version, err = strconv.Atoi(value)
		case 'o':
			s.Origin, err = parseOrigin(value)
		case 's':
			s.Name = value
		case 'i':
			s.Info = value
		case 'u':
			s.Uri = value
		case 'e':
			s.Email = value
		case 'p':
			s.Phone = value
		case 'c':
			var c Connection
			c, err = parseConnection(value)
			s.Connection = &c
		case 't':
			var t Time
			t, err = parseTime(value)
			s.Time = append(s.Time, t)
		case 'a':
			var a Attribute
			a, err = parseAttribute(value)
			s.Attributes = append(s.Attributes, a)
		case 'm':
			var m Media
			m, more, err = parseMedia(text)
			s.Media = append(s.Media, m)
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
