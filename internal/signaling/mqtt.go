package signaling

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"sync"

	flag "github.com/spf13/pflag"

	"github.com/lanikai/alohartc/internal/config"
	"github.com/lanikai/alohartc/internal/ice"
	"github.com/lanikai/oahu/api/mq"
)

var (
	mqttBrokerFlag string
	certFlag       string
	keyFlag        string
)

func init() {
	flag.StringVarP(&mqttBrokerFlag, "mqtt-address", "m", config.MQTT_BROKER, "MQTT broker address")
	flag.StringVarP(&certFlag, "certificate", "c", "/etc/alohartcd/cert.pem", "Client certificate for connecting to MQTT broker")
	flag.StringVarP(&keyFlag, "private-key", "k", "/etc/alohartcd/key.pem", "Private key corresponding to client certificate")

	RegisterListener(mqttListener)
}

// Connect to the Oahu MQTT broker and subscribe to topics for incoming calls.
func mqttListener(handler SessionHandler) error {
	// Load certificate and key.
	cert, err := tls.LoadX509KeyPair(certFlag, keyFlag)
	if err != nil {
		return err
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	// Extract the subject Common Name from the client certificate, and use it
	// as the MQTT client ID.
	var clientID string
	tlsConfig.BuildNameToCertificate()
	for clientID, _ = range tlsConfig.NameToCertificate {
		break
	}

	topicPrefix := fmt.Sprintf("devices/%s", clientID)

	// Connect to MQTT broker.
	err = mq.Connect(mq.Config{
		Server:      mqttBrokerFlag,
		ClientID:    clientID,
		TLSConfig:   tlsConfig,
		WillTopic:   topicPrefix + "/status",
		WillRetain:  true,
		WillPayload: []byte("disconnected"),
	})
	if err != nil {
		return err
	}

	var callLock sync.Mutex
	calls := make(map[string]*callState)

	ctx := context.TODO()

	// Listen for incoming calls.
	topicFilter := topicPrefix + "/calls/+/remote/#"
	mq.Subscribe(topicFilter, 1, func(msg mq.Message) {
		log.Debug("Received MQTT message on topic %s: %q", msg.Topic, msg.Payload)
		callID := msg.Wildcards[0]
		what := msg.Wildcards[1]
		body := string(msg.Payload)

		// If this is a new call, invoke the handler.
		callLock.Lock()
		call, existing := calls[callID]
		if !existing {
			call = newCall(callID, fmt.Sprintf("%s/calls/%s/local", topicPrefix, callID))
			calls[callID] = call
			go handler(call.session)
		}
		callLock.Unlock()

		// Handle the message.
		switch what {
		case "sdp-offer":
			call.offerCh <- body
		case "ice-candidate":
			if len(body) == 0 {
				close(call.rcandCh)
				break
			}
			var desc, sdpMid string
			for _, line := range strings.Split(body, "\n") {
				if line == "" {
					continue
				} else if strings.HasPrefix(line, "candidate:") {
					desc = line
				} else if strings.HasPrefix(line, "mid:") {
					sdpMid = line[4:]
				} else {
					log.Warn("Invalid 'ice-candidate' payload: %q", body)
				}
			}
			if c, err := ice.ParseCandidate(desc, sdpMid); err != nil {
				log.Warn("Invalid ICE candidate (%q, %q): %v", desc, sdpMid, err)
			} else {
				call.rcandCh <- c
			}
		default:
			log.Warn("Unrecognized MQTT topic level: %s", what)
		}
	})
	defer mq.Unsubscribe(topicFilter)

	mq.Publish(topicPrefix+"/status", 1, []byte("connected"))

	<-ctx.Done()
	return nil
}

// Creates a new call object.
func newCall(id, topicPrefix string) *callState {
	offerCh := make(chan string)
	rcandCh := make(chan ice.Candidate)
	session := &Session{
		Context:          context.Background(),
		Offer:            offerCh,
		RemoteCandidates: rcandCh,
		SendAnswer: func(sdp string) error {
			mq.Publish(topicPrefix+"/sdp-answer", 0, []byte(sdp))
			return nil
		},
		SendLocalCandidate: func(c *ice.Candidate) error {
			var payload bytes.Buffer
			if c != nil {
				fmt.Fprintf(&payload, "%s\nmid:%s\n", c.String(), c.Mid())
			}
			mq.Publish(topicPrefix+"/ice-candidate", 0, payload.Bytes())
			return nil
		},
	}

	return &callState{
		id:      id,
		offerCh: offerCh,
		rcandCh: rcandCh,
		session: session,
	}
}

type callState struct {
	id      string
	offerCh chan string
	rcandCh chan ice.Candidate
	session *Session
}
