package signaling

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
)

type Client interface {
	Connect() error

	WaitForSession() (*Session, error)
}

// Client interface for communicating with the signaling service.
// This just wraps the MQTT client that talks to AWS IoT.
type client struct {
	config *Config

	mqttClient mqtt.Client

	sessionMap  map[string]*Session
	sessionChan chan *Session
}

func NewClient(configPath string) (Client, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	return NewClientWithConfig(config), nil
}

func NewClientWithConfig(config *Config) Client {
	return &client{
		config:      config,
		mqttClient:  nil,
		sessionMap:  make(map[string]*Session),
		sessionChan: make(chan *Session, 8),
	}
}

// Connect as a client to the AWS IoT, and subscribe to topics that we're interested in.
func (c *client) Connect() error {
	if c.mqttClient != nil {
		panic("MQTT already connected")
	}

	config := c.config
	if config.Debug {
		mqtt.DEBUG = log.New(os.Stdout, "mqtt: ", log.Lshortfile)
	}

	cert, err := tls.X509KeyPair([]byte(config.CertificatePem), []byte(config.KeyPair.PrivateKey))
	if err != nil {
		return errors.Wrap(err, "Failed to generate key-pair")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		// https://aws.amazon.com/jp/blogs/iot/mqtt-with-tls-client-authentication-on-port-443-why-it-is-useful-and-how-it-works/
		NextProtos: []string{
			"x-amzn-mqtt-ca",
		},
	}
	tlsConfig.BuildNameToCertificate()

	// Moved to 443 vs 8883 for broader compatibility, requires NextProtos to be set above
	serverURL := fmt.Sprintf("ssl://%s:443", config.ServiceEndpoint)
	log.Println("Server url:", serverURL)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(serverURL)
	opts.SetClientID(config.CertificateArn).SetTLSConfig(tlsConfig)
	//opts.SetDefaultPublishHandler(handler)

	// Sets the last will and testament to notify all clients that we disconnected
	disconnectedMsg := Message{"status", map[string]string{"status": "Disconnected"}}
	broadcastTopic := config.getBroadcastTopic()
	opts.SetWill(broadcastTopic, string(disconnectedMsg.json()), 1, false)

	// Connect to MQTT server
	mc := mqtt.NewClient(opts)
	if token := mc.Connect(); token.Wait() && token.Error() != nil {
		return errors.Wrapf(token.Error(), "Failed to connect to %s", serverURL)
	}
	c.mqttClient = mc

	log.Printf("Connected to Signaling Service as %s\n", config.DeviceId)

	// Subscribe to signaling topic - must be before sending connected event
	signalingReceivingTopic := config.getSignalingReceivingTopic()
	if token := mc.Subscribe(signalingReceivingTopic, 1, c.handleMessage); token.Wait() && token.Error() != nil {
		return errors.Wrap(token.Error(), "Failed to subscribe to signaling topic")
	}

	// Notify clients that we have connected
	connectedMsg := Message{"status", map[string]string{"status": "Connected"}}
	if token := mc.Publish(broadcastTopic, 1, false, connectedMsg.json()); token.Wait() && token.Error() != nil {
		return errors.Wrap(token.Error(), "Failed to publish that we have connected")
	}

	log.Println("Notified clients that we are connected")

	//serverReceivingTopic := c.config.getServerReceivingTopic()
	//if token := c.mqttClient.Subscribe(serverReceivingTopic, 1, mh); token.Wait() && token.Error() != nil {
	//	log.Printf("Failed to subscribe to server messaging topic: %v\n", token.Error())
	//}

	//fmt.Println("Go to the web browser client to send a message here!")

	return nil
}

func (c *client) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	msg.Ack()
	topic := msg.Topic()
	log.Println("Received message on topic:", topic)
	m := new(Message)
	json.Unmarshal(msg.Payload(), m)

	if strings.HasPrefix(msg.Topic(), fmt.Sprintf("%s-sC/", c.config.Stage)) {
		// Incoming session message.
		components := strings.Split(topic, "/")
		sid := components[4] + "/" + components[5]
		session, found := c.sessionMap[sid]
		if !found {
			session = &Session{
				client:       c,
				id:           sid,
				receiveTopic: topic,
				sendTopic:    strings.Replace(topic, "-sC", "-sH", 1),
				ingress:      make(chan *Message, 32),
			}
			// Record the session, and alert anybody waiting for new sessions.
			c.sessionMap[sid] = session
			c.sessionChan <- session
		}
		session.ingress <- m
	}

	// TODO: Handle broadcasts and server messages.
}

// Send a TP-message to the given AWS IoT topic.
func (c *client) send(topic string, t string, p interface{}) error {
	m := Message{t, p}
	token := c.mqttClient.Publish(topic, 1, false, m.json())
	token.Wait()
	return errors.Wrapf(token.Error(), "Failed to publish message to %s", topic)
}

// Wait for the browser to initiate a session (by publishing to the "-sC" topic).
func (c *client) WaitForSession() (*Session, error) {
	if c.mqttClient == nil {
		panic("MQTT not connected")
	}

	select {
	case session := <-c.sessionChan:
		return session, nil
	case <-time.After(10 * time.Minute):
		return nil, errors.New("Timed out waiting for a new session")
	}
}

func (c *client) RequestICEServers() error {
	topic := c.config.getMessageToServerTopic()
	return c.send(topic, "ice", map[string]interface{}{"expirySeconds": 120})
}

type Message struct {
	T string      `json:"t"`
	P interface{} `json:"p"`
}

func (m *Message) json() []byte {
	b, err := json.Marshal(m)
	if err != nil {
		panic(errors.Wrapf(err, "Cannot encode JSON: t=%s, p=%v", m.T, m.P))
	}
	return b
}
