// +build rtsp !production

package rtsp

import (
	"errors"
	"fmt"
	"time"

	"github.com/nareix/joy4/codec/h264parser"

	"github.com/lanikai/alohartc/internal/media"
	"github.com/lanikai/alohartc/internal/rtp"
	"github.com/lanikai/alohartc/internal/sdp"
)

func Open(uri string) (media.VideoSource, error) {
	// Normalize URI.
	u, err := ParseURL(uri)
	if err != nil {
		return nil, err
	}
	uri = u.String()

	cli, err := Dial(u.Host)
	if err != nil {
		return nil, err
	}

	desc, err := cli.Describe(uri)
	if err != nil {
		return nil, err
	}
	log.Debug("RTSP SDP:\n%s", &desc)

	for _, m := range desc.Media {
		if m.Type == "video" {
			return newVideoSource(cli, m)
		}
	}

	return nil, errors.New("RTSP stream does not contain video: " + uri)
}

type videoSource struct {
	media.Flow

	// Signal channel used to stop the video Flow.
	quit chan struct{}

	// RTSP client.
	cli *Client

	// RTSP URI for playing this video stream.
	uri string

	// H.264 Sequence Parameter Set.
	sps h264parser.SPSInfo
}

func newVideoSource(cli *Client, m sdp.Media) (*videoSource, error) {
	uri, sps, err := extractVideoMetadata(m)
	if err != nil {
		return nil, err
	}

	video := &videoSource{
		cli: cli,
		uri: uri,
		sps: sps,
	}
	video.Flow.Start = video.start
	video.Flow.Stop = video.stop
	return video, nil
}

func extractVideoMetadata(m sdp.Media) (controlURI string, sps h264parser.SPSInfo, err error) {
	controlURI = m.GetAttr("control")
	if controlURI == "" {
		err = errors.New("RTSP video source: SDP missing 'control' attribute")
		return
	}

	fmtp := m.GetAttrs("fmtp")
	if len(fmtp) != 1 {
		err = errors.New("RTSP video source: expected unique 'fmtp' attribute")
		return
	}

	var payloadType int
	var fmtpOptions string
	var h264fmtp sdp.H264FormatParameters
	fmt.Sscanf(fmtp[0], "%d %s", &payloadType, &fmtpOptions)
	h264fmtp.Unmarshal(fmtpOptions)
	if len(h264fmtp.SpropParameterSets) == 0 {
		err = errors.New("RTSP video source: expected 'sprop-parameter-sets' attribute")
		return
	}

	sps, err = h264parser.ParseSPS(h264fmtp.SpropParameterSets[0])
	log.Debug("RTSP video source: SPS = %#v", sps)
	return
}

func (video *videoSource) Codec() string {
	return "H264" // TODO: Parse rtpmap
}

func (video *videoSource) ClockRate() int {
	return 90000 // TODO: Parse rtpmap
}

func (video *videoSource) Width() int {
	return int(video.sps.Width)
}

func (video *videoSource) Height() int {
	return int(video.sps.Height)
}

func (video *videoSource) start() {
	transport, sessionID, err := video.cli.Setup(video.uri)
	if err != nil {
		// TODO: Propagate errors normally.
		panic(err)
	}
	log.Debug("video Transport: %s", transport.Header())

	video.quit = make(chan struct{})

	go func() {
		// Initialize RTP session to receive the video stream.
		rtpSession := rtp.NewSession(rtp.SessionOptions{
			DataConn:    transport.RTP,
			ControlConn: transport.RTCP,
		})
		stream := rtpSession.AddStream(rtp.StreamOptions{
			RemoteSSRC: transport.SSRC,
			Direction:  "recvonly",
		})

		// Feed video buffers from the RTP stream into video.Flow, until the
		// stream is interrupted.
		err := stream.ReceiveVideo(video.quit, video.Flow.Put)

		// Clean up nicely on exit.
		stream.Close()
		video.cli.Teardown(video.uri, sessionID)
		video.Flow.Shutdown(err)
	}()

	// Tell RTSP server to begin sending the video stream.
	if _, err := video.cli.Play(video.uri, sessionID); err != nil {
		panic(err)
	}

	// Send periodic RTSP keepalives.
	go func() {
		for {
			select {
			case <-video.quit:
				return
			case <-time.After(10 * time.Second):
				video.cli.GetParameter(video.uri, sessionID)
			}
		}
	}()
}

func (video *videoSource) stop() {
	// Close video.quit only if it's not already closed.
	if video.quit != nil {
		select {
		case <-video.quit:
		default:
			close(video.quit)
		}
	}
}
