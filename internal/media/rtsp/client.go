// +build rtsp

package rtsp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/lanikai/alohartc/internal/sdp"
)

// RTSP 1.0 client implementation.
// See [RFC 2326](https://tools.ietf.org/html/rfc2326).
type Client struct {
	// TCP connection to the RTSP server.
	conn net.Conn

	// Monotonically increasing request sequence number.
	cseq int

	sync.Mutex
}

func Dial(address string) (*Client, error) {
	return DialContext(context.Background(), address)
}

func DialContext(ctx context.Context, address string) (*Client, error) {
	// Connect to RTSP server.
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp4", address)
	if err != nil {
		return nil, err
	}

	cli := &Client{
		conn: conn,
	}
	return cli, nil
}

type HeaderMap map[string]string

type Response struct {
	Status  int
	Reason  string
	Headers HeaderMap
	Content []byte
}

type RequestFailure struct {
	method string
	uri    string
	status int
	reason string
}

func (f *RequestFailure) Error() string {
	return fmt.Sprintf("RTSP request failure: %s %s => %d %s", f.method, f.uri, f.status, f.reason)
}

// Sends a request to the RTSP server, and parses the response.
func (cli *Client) Request(method, uri string, headers HeaderMap) (*Response, error) {
	cli.Lock()
	defer cli.Unlock()

	cli.cseq++

	buf := &bytes.Buffer{}

	// RTSP request line, e.g. "DESCRIBE rtsp://127.0.0.1:554/foo RTSP/1.0\r\n"
	fmt.Fprintf(buf, "%s %s RTSP/1.0\r\n", method, uri)

	// Mandatory CSeq header.
	fmt.Fprintf(buf, "CSeq: %s\r\n", strconv.Itoa(cli.cseq))

	// Request-specific headers.
	for name, value := range headers {
		fmt.Fprintf(buf, "%s: %s\r\n", name, value)
	}

	// Terminating CLRF.
	buf.WriteString("\r\n")

	// Write request bytes.
	_, err := cli.conn.Write(buf.Bytes())
	if err != nil {
		return nil, err
	}

	resp := &Response{
		Headers: make(HeaderMap),
	}
	br := bufio.NewReader(cli.conn)
	contentLength := 0

	// Read response one line at a time.
	for {
		lineBytes, _, err := br.ReadLine()
		if err != nil {
			return nil, err
		}
		line := string(lineBytes)

		if resp.Status == 0 {
			// Parse RTSP status line.
			_, err = fmt.Sscanf(line, "RTSP/1.0 %3d", &resp.Status)
			if err != nil {
				return nil, err
			}
			resp.Reason = strings.TrimSpace(line[12:])
		} else if line == "" {
			// Empty line indicates end of response headers.
			break
		} else {
			// Parse response header.
			i := strings.IndexByte(line, ':')
			if i < 0 {
				return nil, fmt.Errorf("invalid RTSP header: %q", line)
			}
			name := line[0:i]
			value := strings.TrimSpace(line[i+1:])
			resp.Headers[name] = value
			if name == "Content-Length" {
				contentLength, _ = strconv.Atoi(value)
			}
		}
	}

	if contentLength > 0 {
		resp.Content = make([]byte, contentLength)
		if _, err := io.ReadFull(br, resp.Content); err != nil {
			return nil, err
		}
	}

	// TODO: Automatically handle redirects.
	if resp.Status >= 400 {
		return nil, &RequestFailure{method, uri, resp.Status, resp.Reason}
	}

	return resp, err
}

// Send an OPTIONS request, and parse the response from the Public header..
func (cli *Client) Options() ([]string, error) {
	resp, err := cli.Request("OPTIONS", "*", nil)
	if err != nil {
		return nil, err
	}

	// See https://tools.ietf.org/html/rfc2068#section-14.35
	public := resp.Headers["Public"]
	options := strings.Split(public, ",")
	for i := range options {
		options[i] = strings.TrimSpace(options[i])
	}

	return options, nil
}

// Send a DESCRIBE request, and parse the SDP response.
func (cli *Client) Describe(uri string) (sdp.Session, error) {
	resp, err := cli.Request("DESCRIBE", uri, HeaderMap{
		"Accept": "application/sdp",
	})
	if err != nil {
		return sdp.Session{}, err
	}

	return sdp.ParseSession(string(resp.Content))
}

// Send a SETUP request, and return the established transport and session ID.
// See https://tools.ietf.org/html/rfc2326#section-10.4
func (cli *Client) Setup(uri string) (*Transport, string, error) {
	tr, err := NewTransport()
	if err != nil {
		return nil, "", err
	}

	resp, err := cli.Request("SETUP", uri, HeaderMap{
		"Transport": tr.ClientHeader(),
	})
	if err != nil {
		tr.Close()
		return nil, "", err
	}

	serverIP := cli.conn.RemoteAddr().(*net.TCPAddr).IP
	tr.parseServerResponse(resp.Headers["Transport"], serverIP)

	// See https://tools.ietf.org/html/rfc2326#section-12.37
	session := strings.Split(resp.Headers["Session"], ";")[0]

	return tr, session, nil
}

// See https://tools.ietf.org/html/rfc2326#section-10.5
func (cli *Client) Play(uri, session string) (rtpInfo string, err error) {
	resp, err := cli.Request("PLAY", uri, HeaderMap{
		"Session": session,
	})
	if err != nil {
		return
	}

	rtpInfo = resp.Headers["RTP-Info"]
	return
}

// Send a PAUSE request.
// See https://tools.ietf.org/html/rfc2326#section-10.6
func (cli *Client) Pause(uri, session string) error {
	_, err := cli.Request("PAUSE", uri, HeaderMap{
		"Session": session,
	})
	return err
}

// Send a TEARDOWN request.
// See https://tools.ietf.org/html/rfc2326#section-10.7
func (cli *Client) Teardown(uri, session string) error {
	_, err := cli.Request("TEARDOWN", uri, HeaderMap{
		"Session": session,
	})
	return err
}

// Send a GET_PARAMETER request, and return the response.
// TODO: What should the request body be?
func (cli *Client) GetParameter(uri, session string) (string, error) {
	resp, err := cli.Request("GET_PARAMETER", uri, HeaderMap{
		"Session": session,
	})
	if err != nil {
		return "", err
	}

	return string(resp.Content), nil
}
