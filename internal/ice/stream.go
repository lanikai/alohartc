package ice

// A data stream is the final product of ICE: a connection over which data can
// be exchanged with the remote peer. It implements `net.Conn` so that it can
// be easily passed to other components that expect such an object.

import (
	//"net"
	"sync"
)

type DataStream struct {
	// Media ID
	mid string

	component int

	// Concatenation of local and remote `ice-ufrag` option
	username string

	// Local and remote `ice-pwd` options
	localPassword  string
	remotePassword string

	candidateLock sync.Mutex

	localCandidates  []Candidate
	remoteCandidates []Candidate

	checklist Checklist
}

func newDataStream(mid string, component int, username, localPassword, remotePassword string) *DataStream {
	return &DataStream{
		mid: mid,
		component: component,
		username: username,
		localPassword: localPassword,
		remotePassword: remotePassword,
	}
}

func (ds *DataStream) addLocalCandidate(c Candidate) {
	ds.candidateLock.Lock()
	defer ds.candidateLock.Unlock()

	ds.localCandidates = append(ds.localCandidates, c)
	// Pair new local candidate with all existing remote candidates.
	ds.checklist.addCandidatePairs([]Candidate{c}, ds.remoteCandidates)
}

func (ds *DataStream) addRemoteCandidate(c Candidate) {
	ds.candidateLock.Lock()
	defer ds.candidateLock.Unlock()

	ds.remoteCandidates = append(ds.remoteCandidates, c)
	// Pair new remote candidate with all existing local candidates.
	ds.checklist.addCandidatePairs(ds.localCandidates, []Candidate{c})
}
