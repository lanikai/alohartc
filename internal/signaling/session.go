package signaling

// A browser session with this device.
type Session struct {
	client *client

	id string

	receiveTopic string
	sendTopic    string

	ingress chan *Message
}

func (s *Session) ReceiveMessage() (t string, p interface{}) {
	m := <-s.ingress
	return m.T, m.P
}

func (s *Session) SendMessage(t string, p interface{}) error {
	return s.client.send(s.sendTopic, t, p)
}
