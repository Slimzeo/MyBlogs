package service

import (
	"sync"
	"sync/atomic"

	"myblog/internal/model"
)

// HitCounter batches article view increments in memory and flushes them to the
// database periodically. This replaces the original per-request DB write (and
// its buggy single global "article:hits" cache key) with a per-article atomic
// accumulator, so a viral article under heavy concurrent traffic costs almost
// nothing until the batched flush.
type HitCounter struct {
	states     sync.Map // cid -> *hitState
	flushEvery int
	flushQueue chan int
	stop       chan struct{}
	stopped    sync.WaitGroup
	closeOnce  sync.Once
	svc        *Service
}

type hitState struct {
	pending   atomic.Int64
	persisted atomic.Int64
	queued    atomic.Bool
}

// NewHitCounter builds a counter that flushes a cid once it accumulates
// flushEvery views (mirrors WebConst.HIT_EXCEED semantics), and also supports
// a periodic FlushAll for the remainder.
func (s *Service) NewHitCounter(flushEvery int) *HitCounter {
	if flushEvery < 1 {
		flushEvery = 10
	}
	counter := &HitCounter{
		flushEvery: flushEvery,
		flushQueue: make(chan int, 4096),
		stop:       make(chan struct{}),
		svc:        s,
	}
	counter.stopped.Add(1)
	go counter.flushWorker()
	return counter
}

// Incr records one view for cid. When the per-article pending count reaches the
// threshold it is flushed to the DB immediately (hits = hits + delta).
func (h *HitCounter) Incr(cid int) {
	if cid == 0 {
		return
	}
	state := h.state(cid)
	if state.pending.Add(1) >= int64(h.flushEvery) && state.queued.CompareAndSwap(false, true) {
		select {
		case h.flushQueue <- cid:
		default:
			state.queued.Store(false)
		}
	}
}

func (h *HitCounter) Observe(cid, persisted int) {
	state := h.state(cid)
	for {
		current := state.persisted.Load()
		if current >= int64(persisted) || state.persisted.CompareAndSwap(current, int64(persisted)) {
			return
		}
	}
}

func (h *HitCounter) Current(cid int) int {
	state := h.state(cid)
	return int(state.persisted.Load() + state.pending.Load())
}

// Pending returns the not-yet-persisted view count for cid (used to display an
// up-to-date hit number without hitting the DB).
func (h *HitCounter) Pending(cid int) int {
	return int(h.state(cid).pending.Load())
}

// FlushAll persists every pending counter. Called periodically and on shutdown
// so no views are lost.
func (h *HitCounter) FlushAll() {
	h.states.Range(func(key, value any) bool {
		state := value.(*hitState)
		h.flush(key.(int), state, int(state.pending.Swap(0)))
		return true
	})
}

// Close waits for the asynchronous writer and persists the final remainder.
func (h *HitCounter) Close() {
	h.closeOnce.Do(func() {
		close(h.stop)
		h.stopped.Wait()
		h.FlushAll()
	})
}

func (h *HitCounter) flushWorker() {
	defer h.stopped.Done()
	for {
		select {
		case cid := <-h.flushQueue:
			value, exists := h.states.Load(cid)
			if exists {
				state := value.(*hitState)
				h.flush(cid, state, int(state.pending.Swap(0)))
				state.queued.Store(false)
			}
		case <-h.stop:
			return
		}
	}
}

func (h *HitCounter) state(cid int) *hitState {
	value, _ := h.states.LoadOrStore(cid, &hitState{})
	return value.(*hitState)
}

func (h *HitCounter) flush(cid int, state *hitState, delta int) {
	if delta <= 0 {
		return
	}
	err := h.svc.db.Model(&model.Content{}).Where("cid = ?", cid).
		Update("hits", gormExprAdd("hits", delta)).Error
	if err != nil {
		state.pending.Add(int64(delta))
		return
	}
	state.persisted.Add(int64(delta))
}
