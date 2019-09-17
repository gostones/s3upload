package internal

import (
	"sync/atomic"
)

type Counter int64

func (r *Counter) Increment(n int64) int64 {
	return atomic.AddInt64((*int64)(r), n)
}

func (r *Counter) Decrement(n int64) int64 {
	return atomic.AddInt64((*int64)(r), -n)
}

func (r *Counter) Get() int64 {
	return atomic.LoadInt64((*int64)(r))
}

func (r *Counter) Reset() {
	atomic.StoreInt64((*int64)(r), 0)
}
