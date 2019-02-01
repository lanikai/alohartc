package ice

import (
	"fmt"
)

type Session struct {
	streams []*DataStream

//	state RunState
}

//type RunState int
//const (
//	Running   RunState = iota
//	Completed
//	Failed
//)

func NewSession() *Session {
	return &Session{
		streams: nil,
//		state: Running,
	}
}

func (s *Session) AddDataStream(mid string, component int, username, localPassword, remotePassword string) {
	ds := newDataStream(mid, component, username, localPassword, remotePassword)
	s.streams = append(s.streams, ds)
}

func (s *Session) getDataStream(mid string) (*DataStream, error) {
	for _, ds := range s.streams {
		if ds.mid == mid {
			return ds, nil
		}
	}
	return nil, fmt.Errorf("No data stream with mid=%s", mid)
}

func (s *Session) AddRemoteCandidate(desc, mid string) error {
	if desc == "" {
		// TODO: This should signal end of trickling.
		return nil
	}

	c := Candidate{mid: mid}
	if err := parseCandidateSDP(desc, &c); err != nil {
		return err
	}

	if ds, err := s.getDataStream(mid); err != nil {
		return err
	} else {
		ds.addRemoteCandidate(c)
	}
	return nil
}

func (s *Session) EstablishConnection() error {
	return nil
}
