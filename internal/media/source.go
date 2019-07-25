package media

import (
	"sync"
)

type Source interface {
	NewConsumer() *BufferConsumer

	Close() error
}

// A BufferConsumer receives output buffers from a media Source.
type BufferConsumer struct {
	bufCh chan *SharedBuffer

	stop func()
}

// NextBuffer returns a channel providing the next shared buffer. The channel
// will be closed when the source is exhausted.
func (bc *BufferConsumer) NextBuffer() <-chan *SharedBuffer {
	return bc.bufCh
}

// Stop receiving buffers. Once stopped, furthers calls to Stop() do nothing.
func (bc *BufferConsumer) Stop() {
	if bc == nil || bc.stop == nil {
		return
	}
	bc.stop()
	bc.stop = nil
}

// baseSource is a helper to manage consumers of a Source.
type baseSource struct {
	consumers map[int]*BufferConsumer
	nextID    int
	sync.Mutex
}

func (s *baseSource) newConsumer() (*BufferConsumer, int) {
	s.Lock()
	defer s.Unlock()

	if s.consumers == nil {
		s.consumers = make(map[int]*BufferConsumer)
	}

	id := s.nextID
	bufCh := make(chan *SharedBuffer)
	consumer := &BufferConsumer{
		bufCh: bufCh,
		stop: func() {
			s.Lock()
			delete(s.consumers, id)
			close(bufCh)
			s.Unlock()
		},
	}
	s.consumers[id] = consumer
	s.nextID++
	return consumer, len(s.consumers)
}

func (s *baseSource) numConsumers() int {
	return len(s.consumers)
}

func (s *baseSource) putBuffer(buf *SharedBuffer) {
	s.Lock()
	defer s.Unlock()

	for _, consumer := range s.consumers {
		consumer.bufCh <- buf
	}
}
