package media

import (
	"sync"
)

type Source interface {
	StartReceiving() <-chan *SharedBuffer
	StopReceiving(bufCh <-chan *SharedBuffer)

	Close() error
}

// baseSource is a helper to manage consumers of a Source.
type baseSource struct {
	receivers []chan *SharedBuffer
	sync.Mutex
}

func (s *baseSource) startRecv() <-chan *SharedBuffer {
	s.Lock()
	defer s.Unlock()

	r := make(chan *SharedBuffer)
	s.receivers = append(s.receivers, r)
	return r
}

func (s *baseSource) stopRecv(bufCh <-chan *SharedBuffer) int {
	s.Lock()
	defer s.Unlock()

	// Find and delete bufCh from receivers list.
	// See https://github.com/golang/go/wiki/SliceTricks
	n := len(s.receivers)
	for i, r := range s.receivers {
		if r == bufCh {
			copy(s.receivers[i:], s.receivers[i+1:])
			s.receivers[n-1] = nil
			s.receivers = s.receivers[:n-1]
			close(r)
			break
		}
	}
	return len(s.receivers)
}

func (s *baseSource) numReceivers() int {
	return len(s.receivers)
}

func (s *baseSource) putBuffer(buf []byte, release func()) {
	s.Lock()
	defer s.Unlock()

	for _, r := range s.receivers {
		r <- NewSharedBuffer(buf, release)
	}
}
