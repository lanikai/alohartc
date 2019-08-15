package media

import (
	"sync"
)

// A loopFunc is a long-running function, e.g. a read loop. It should terminate
// promptly when the quit channel is closed.
type loopFunc func(quit <-chan struct{})

// A singletonLoop is a wrapper for a long-running function that should only run
// in a single goroutine at any given time. Each call to start() counts as a
// "vote" in favor of running the function (and stop() removes a vote). The
// function is actually started when the vote count goes from 0 to 1, and
// terminated when the count goes from 1 to 0. Callers must ensure that each
// start() call is matched by a corresponding stop().
type singletonLoop struct {
	// The long-running function.
	run loopFunc

	// Votes in favor of running the loop.
	votes int

	// Closed when stop() is requested, to trigger run loop exit.
	quit chan struct{}

	// Closed when run loop actually terminates.
	terminated chan struct{}

	sync.Mutex
}

func newSingletonLoop(run loopFunc) *singletonLoop {
	return &singletonLoop{
		run: run,
	}
}

func (loop *singletonLoop) start() {
	loop.Lock()
	defer loop.Unlock()

	loop.votes++

	if loop.votes > 1 {
		loop.assertRunning()
		return
	}

	if loop.quit != nil || loop.terminated != nil {
		panic("singletonLoop: already running")
	}
	loop.quit = make(chan struct{})
	loop.terminated = make(chan struct{})

	go func() {
		log.Debug("Starting singleton loop: %v", loop.run)
		loop.run(loop.quit)
		// Close terminated channel to unblock stop().
		close(loop.terminated)
	}()
}

func (loop *singletonLoop) stop() {
	loop.Lock()
	defer loop.Unlock()

	loop.assertRunning()

	loop.votes--
	if loop.votes < 0 {
		panic("singletonLoop: negative vote count")
	}
	if loop.votes == 0 {
		log.Debug("Stopping singleton loop: %v", loop.run)
		close(loop.quit)
		<-loop.terminated

		loop.quit = nil
		loop.terminated = nil
	}

}

func (loop *singletonLoop) assertRunning() {
	if loop.quit == nil || loop.terminated == nil {
		panic("singletonLoop: not running")
	}
}
