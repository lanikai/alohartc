// +build rtsp

package rtsp

import (
	"time"

	"github.com/lanikai/alohartc/internal/media"
	"github.com/lanikai/alohartc/internal/rtp"
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

	sdp, err := cli.Describe(uri)
	if err != nil {
		return nil, err
	}
	log.Debug("RTSP SDP:\n%s", &sdp)

	// TODO: For now we assume H.264, need to handle other types.
	videoURI := sdp.Media[0].GetAttr("control")

	transport, sessionID, err := cli.Setup(videoURI)
	if err != nil {
		return nil, err
	}
	log.Debug("Transport: %#v", transport)

	rtpSession := rtp.NewSession(transport.RTP, rtp.SessionOptions{})
	videoStream := rtpSession.AddStream(rtp.StreamOptions{
		RemoteSSRC: transport.SSRC,
		Direction:  "recvonly",
	})

	video := &rtspVideoSource{}
	video.Flow.Start = func() {
		video.quit = make(chan struct{})
		go videoStream.ReceiveVideo(video.quit, video.Put)
		cli.Play(videoURI, sessionID)
		go func() {
			for {
				select {
				case <-video.quit:
					cli.Pause(videoURI, sessionID)
					return
				case <-time.After(10 * time.Second):
					// Send keep-alive.
					cli.GetParameter(uri, sessionID)
				}
			}
		}()
	}
	video.Flow.Stop = func() {
		close(video.quit)
		video.quit = nil
	}

	return video, nil
}

type rtspVideoSource struct {
	media.Flow

	quit chan struct{}
}

func (vs *rtspVideoSource) Codec() string {
	return "H264"
}

func (vs *rtspVideoSource) Width() int {
	return 0 // TODO: Parse sprop-parameter-sets
}

func (vs *rtspVideoSource) Height() int {
	return 0 // TODO: Parse sprop-parameter-sets
}
