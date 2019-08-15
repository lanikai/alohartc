// +build rtsp

package rtsp

import (
	"testing"
)

func TestBindUDPPair(t *testing.T) {
	even, odd, err := bindUDPPair()
	if err != nil {
		t.Fatal(err)
	}
	defer even.Close()
	defer odd.Close()

	t.Log("even:", even.LocalAddr())
	t.Log("odd:", odd.LocalAddr())
}
