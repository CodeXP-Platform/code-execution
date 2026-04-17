package system

import "time"

type Clock struct{}

func NewClock() Clock {
	return Clock{}
}

func (Clock) Now() time.Time {
	return time.Now().UTC()
}
