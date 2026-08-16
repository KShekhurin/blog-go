package processors

import (
	"time"
)

type Backoff interface {
	NextDelay() time.Duration
	AccumulateAdverse()
	ClearAdverse()
}

type ExponentialBackOff struct {
	base         int
	coefficient  time.Duration
	maxDelay     time.Duration
	adverseCount int
}

func NewExponentialBackOff(base int, coefficient, maxDelay time.Duration) Backoff {
	return &ExponentialBackOff{
		base:        base,
		coefficient: coefficient,
		maxDelay:    maxDelay,
	}
}

func (b *ExponentialBackOff) NextDelay() time.Duration {
	if b.adverseCount == 0 {
		return 0
	}

	delay := b.coefficient
	for i := 1; i < b.adverseCount; i++ {
		delay *= time.Duration(b.base)
		if delay >= b.maxDelay {
			delay = b.maxDelay
			break
		}
	}

	return delay
}

func (b *ExponentialBackOff) AccumulateAdverse() {
	b.adverseCount++
}

func (b *ExponentialBackOff) ClearAdverse() {
	b.adverseCount = 0
}
