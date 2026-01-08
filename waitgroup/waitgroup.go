//go:build !solution

package waitgroup

import "fmt"

// A WaitGroup waits for a collection of goroutines to finish.
// The main goroutine calls Add to set the number of
// goroutines to wait for. Then each of the goroutines
// runs and calls Done when finished. At the same time,
// Wait can be used to block until all goroutines have finished.
type WaitGroup struct {
	cnt int
	// канал размера 1 - если в канале что-то есть, то ждать не надо
	// если ничего нет, то ждём (wait) когда появиться
	ch chan struct{}
	// mutex для эксклюзивной работы с cnt
	mu chan struct{}
}

// New creates WaitGroup.
func New() *WaitGroup {
	w := WaitGroup{
		ch: make(chan struct{}, 1),
		mu: make(chan struct{}, 1),
	}
	w.mu <- struct{}{}
	w.ch <- struct{}{}
	return &w
}

// Add adds delta, which may be negative, to the WaitGroup counter.
// If the counter becomes zero, all goroutines blocked on Wait are released.
// If the counter goes negative, Add panics.
//
// Note that calls with a positive delta that occur when the counter is zero
// must happen before a Wait. Calls with a negative delta, or calls with a
// positive delta that start when the counter is greater than zero, may happen
// at any time.
// Typically this means the calls to Add should execute before the statement
// creating the goroutine or other event to be waited for.
// If a WaitGroup is reused to wait for several independent sets of events,
// new Add calls must happen after all previous Wait calls have returned.
// See the WaitGroup example.
func (wg *WaitGroup) Add(delta int) {
	if delta == 0 {
		return
	}
	<-wg.mu
	oldCnt := wg.cnt
	wg.cnt += delta
	if wg.cnt < 0 {
		fmt.Print("Try to panic\n")
		panic("negative WaitGroup counter")
	}
	// Если счетчик стал больше нуля (был 0 или меньше), нужно убрать токен из канала
	if oldCnt == 0 && wg.cnt > 0 {
		select {
		case <-wg.ch:
		default:
		}
	}
	// Если счетчик стал нулем, нужно положить токен в канал
	if wg.cnt == 0 {
		select {
		case wg.ch <- struct{}{}:
		default:
		}
	}
	wg.mu <- struct{}{}
}

// Done decrements the WaitGroup counter by one.
func (wg *WaitGroup) Done() {
	wg.Add(-1)
}

// Wait blocks until the WaitGroup counter is zero.
func (wg *WaitGroup) Wait() {
	<-wg.mu
	if wg.cnt == 0 {
		wg.mu <- struct{}{}
		return
	}
	wg.mu <- struct{}{}
	<-wg.ch
	// После пробуждения нужно вернуть токен обратно в канал
	// Но только если счетчик действительно равен нулю
	<-wg.mu
	if wg.cnt == 0 {
		select {
		case wg.ch <- struct{}{}:
		default:
		}
	}
	wg.mu <- struct{}{}
}

func (wg *WaitGroup) Print(s string, args ...interface{}) {
	fmt.Printf(s, args...)
}
