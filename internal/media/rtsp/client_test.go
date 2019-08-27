package rtsp

import (
	"testing"
)

// Start a local RTSP server, e.g. using VLC:
//    cvlc --loop test.mp4 ':sout=#rtp{sdp=rtsp://:8554/test}'

const (
	serverAddress = ":8554"
	testURI       = "rtsp://:8554/test"
)

func TestLocalServer(t *testing.T) {
	cli, err := Dial(serverAddress)
	if err != nil {
		t.Skip("Can't connect to local RTSP server:", err)
	}

	resp, err := cli.Request("DESCRIBE", testURI, HeaderMap{
		"Accept":     "application/sdp",
		"User-Agent": "alohartc",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Status:", resp.Status)
	t.Log("Reason:", resp.Reason)
	t.Log("Headers:", resp.Headers)
	t.Log("Content:", string(resp.Content))

	// OPTIONS
	options, err := cli.Options()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Options:", options)

	// DESCRIBE
	sdp, err := cli.Describe(testURI)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("SDP:", sdp)

	controlURI := sdp.Media[0].GetAttr("control")

	// SETUP
	tr, sessionID, err := cli.Setup(controlURI)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	t.Log("Transport:", tr.Header())
	t.Log("Session ID:", sessionID)

	// PLAY
	rtpInfo, err := cli.Play(testURI, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("RTP-Info:", rtpInfo)

	// GET_PARAMETER
	params, err := cli.GetParameter(testURI, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("GET_PARAMETER:", params)
}
