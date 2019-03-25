// +build oahu

package signaling

import (
	"bytes"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"strings"
	"sync"

	"github.com/lanikai/alohartc/internal/ice"
	"github.com/lanikai/oahu/api/mq"
)

var (
	mqttBrokerFlag string
	certFlag       string
	keyFlag        string
)

func init() {
	flag.StringVar(&mqttBrokerFlag, "mqttbroker", "127.0.0.1:8883", "MQTT broker address")
	flag.StringVar(&certFlag, "cert", "cert.pem", "Client certificate for connecting to MQTT broker")
	flag.StringVar(&keyFlag, "key", "key.pem", "Private key corresponding to client certificate")
	
	NewClient = newMQTTSignaler
}

type mqttSignaler struct {
	handler SessionHandler

	clientID  string
	tlsConfig *tls.Config

	calls    map[string]*call
	callLock sync.Mutex

	ctx    context.Context
	cancel func()
}

func newMQTTSignaler(handler SessionHandler) (Client, error) {
	// Load certificate and key.
	cert, err := tls.LoadX509KeyPair(certFlag, keyFlag)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	// Extract Common Name from the client certificate, and use it as the client ID.
	var clientID string
	tlsConfig.BuildNameToCertificate()
	for clientID, _ = range tlsConfig.NameToCertificate {
		break
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &mqttSignaler{
		handler:   handler,
		clientID:  clientID,
		tlsConfig: tlsConfig,
		calls:     make(map[string]*call),
		ctx:       ctx,
		cancel:    cancel,
	}
	return s, nil
}

func (s *mqttSignaler) Listen() error {
	err := mq.Connect(mq.Config{
		Server:    mqttBrokerFlag,
		ClientID:  s.clientID,
		TLSConfig: s.tlsConfig,
	})
	if err != nil {
		return err
	}

	topicPrefix := fmt.Sprintf("devices/%s", s.clientID)

	// Listen for incoming calls.
	mq.Subscribe(topicPrefix+"/calls/+/remote/#", 1, func(topic *mq.TopicMatch, payload []byte) {
		log.Debug("Received MQTT message on topic '%s': %q", topic.Name, payload)
		callID := topic.Wildcards[0]
		what := topic.Wildcards[1]

		call := s.getOrCreateCall(callID)
		switch what {
		case "sdp-offer":
			call.offerCh <- string(payload)
		case "ice-candidate":
			if len(payload) == 0 {
				close(call.rcandCh)
				break
			}
			var desc, sdpMid string
			for _, line := range strings.Split(string(payload), "\n") {
				if line == "" {
					continue
				} else if strings.HasPrefix(line, "candidate:") {
					desc = line
				} else if strings.HasPrefix(line, "mid:") {
					sdpMid = line[4:]
				} else {
					log.Warn("Invalid 'ice-candidate' payload: %q", payload)
				}
			}
			if c, err := ice.ParseCandidate(desc, sdpMid); err != nil {
				log.Warn("Invalid ICE candidate: %v", err)
			} else {
				call.rcandCh <- c
			}
		default:
			log.Warn("Unrecognized MQTT topic level: %s", what)
		}
	})

	<-s.ctx.Done()
	return s.ctx.Err()
}

func (s *mqttSignaler) Shutdown() error {
	s.cancel()
	return nil
}

func (s *mqttSignaler) getOrCreateCall(id string) *call {
	s.callLock.Lock()
	defer s.callLock.Unlock()

	if call, ok := s.calls[id]; ok {
		return call
	}

	offerCh := make(chan string)
	rcandCh := make(chan ice.Candidate)
	topicPrefix := fmt.Sprintf("devices/%s/calls/%s/local", s.clientID, id)
	session := &Session{
		Context:          context.Background(),
		Offer:            offerCh,
		RemoteCandidates: rcandCh,
		SendAnswer: func(sdp string) error {
			mq.Publish(topicPrefix+"/sdp-answer", []byte(sdp))
			return nil
		},
		SendLocalCandidate: func(c ice.Candidate) error {
			var payload bytes.Buffer
			fmt.Fprintf(&payload, "%s\nmid:%s\n", c.String(), c.Mid())
			mq.Publish(topicPrefix+"/ice-candidate", payload.Bytes())
			return nil
		},
	}
	go s.handler(session)

	call := &call{
		id:      id,
		offerCh: offerCh,
		rcandCh: rcandCh,
		session: session,
	}
	s.calls[id] = call
	return call
}

type call struct {
	id string

	offerCh chan string
	rcandCh chan ice.Candidate
	session *Session
}
