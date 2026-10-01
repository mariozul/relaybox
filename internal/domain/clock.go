package domain

import "time"

type Clock interface {
	Now() time.Time
}

type FixedClock struct {
	NowT time.Time
}

func (f FixedClock) Now() time.Time { return f.NowT }

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }
