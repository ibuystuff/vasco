package main

// LoopTimer is a timer that keeps re-starting itself after the LoopTime.
import "time"

// Each time it restarts, it calls the LoopFunc in its own goroutine.
// You can short-circuit the loop count by calling AtMost to set the current loop
// to a (possibly) shorter time. It will then return to its normal loop time.
// The granularity (precision) of the timer is controlled by TickTime.
type LoopTimer struct {
	TickTime time.Duration
	LoopTime time.Duration
	LoopFunc func()
	t        *time.Timer
	nextLoop time.Time
}

func NewLoopTimer(tickTime, loopTime time.Duration, loopFunc func()) *LoopTimer {
	t := &LoopTimer{
		TickTime: tickTime,
		LoopTime: loopTime,
		LoopFunc: loopFunc,
		nextLoop: time.Now().Add(loopTime),
	}
	t.t = time.AfterFunc(tickTime, t.tickFunc)
	return t
}

// TickFunc is called after every tick; if the current time is after the nextLoop
// time, we call the LoopFunc() and add the LoopTime to the nextLoop time. Note this is
// NOT added to the current time -- this ensures that on average we'll be no more than
// one TickTime away from the loopTime (as long as no one calls AtMost).
func (l *LoopTimer) tickFunc() {
	if time.Now().After(l.nextLoop) {
		go l.LoopFunc()
		l.nextLoop = l.nextLoop.Add(l.LoopTime)
	}
	l.t = time.AfterFunc(l.TickTime, l.tickFunc)
}

// AtMost specifies that the timer should fire after at most the specified
// duration (this is how you short-circuit a count)
func (l *LoopTimer) AtMost(d time.Duration) {
	maxt := time.Now().Add(d)
	if l.nextLoop.After(maxt) {
		l.nextLoop = maxt
	}
}
