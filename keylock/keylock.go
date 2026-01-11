//go:build !solution

// ====================================================================

// package keylock

// type Req struct {
// 	keys     []string
// 	cancel   <-chan struct{}
// 	isSucces chan bool
// }
// type KeyLock struct {
// 	cancel chan struct{}
// 	muMap  map[string]chan struct{}
// 	req    chan Req
// 	rch    chan struct{}
// }

// func New() *KeyLock {
// 	k := KeyLock{
// 		cancel: make(chan struct{}, 1),
// 		muMap:  make(map[string]chan struct{}),
// 		req:    make(chan Req),
// 		rch:    make(chan struct{}, 1),
// 	}
// 	go k.Run()
// 	return &k
// }

// func ReleaseLocks(l *KeyLock, keys []string, doRefresh bool) {
// 	for _, key := range keys {
// 		select {
// 		case l.muMap[key] <- struct{}{}:
// 		default:
// 		}
// 	}

// 	if doRefresh {
// 		select {
// 		case l.rch <- struct{}{}:
// 		default:
// 		}
// 	}
// }

// func (l *KeyLock) TryLock(curWating Req, waitings *[]Req) bool {
// 	curLocks := make([]string, 0)
// 	for _, key := range curWating.keys {
// 		_, ok := l.muMap[key]
// 		if !ok {
// 			l.muMap[key] = make(chan struct{}, 1)
// 			l.muMap[key] <- struct{}{}
// 		}

// 		select {
// 		case <-l.muMap[key]:
// 			curLocks = append(curLocks, key)
// 		case <-curWating.cancel:
// 			ReleaseLocks(l, curLocks, false)
// 			curLocks = make([]string, 0)
// 			// *waitings = append(*waitings, curWating)
// 			return false
// 		case <-curWating.cancel:
// 			ReleaseLocks(l, curLocks, false)
// 			curLocks = make([]string, 0)
// 			*waitings = append(*waitings, curWating)
// 			return false
// 		// case <-time.After(100 * time.Millisecond):
// 		default:
// 			ReleaseLocks(l, curLocks, false)
// 			curLocks = make([]string, 0)
// 			*waitings = append(*waitings, curWating)
// 			return false
// 		}
// 	}
// 	return true
// }

// func (l *KeyLock) Run() {
// 	newWaitings := make([]Req, 0)
// 	waitings := make([]Req, 0)
// 	var curWating Req
// 	var isSucces bool
// 	for {
// 		if len(newWaitings) > 0 {
// 			select {
// 			case curReq := <-l.req:
// 				newWaitings = append(newWaitings, curReq)
// 			case <-l.rch:
// 				newWaitings = append(newWaitings, waitings...)
// 				waitings = make([]Req, 0)
// 			default:
// 			}

// 		} else {
// 			select {
// 			case curReq := <-l.req:
// 				newWaitings = append(newWaitings, curReq)
// 			case <-l.rch:
// 				newWaitings = append(newWaitings, waitings...)
// 				waitings = make([]Req, 0)
// 			}
// 		}

// 		if len(newWaitings) == 0 {
// 			continue
// 		}
// 		curWating = newWaitings[0]
// 		newWaitings = newWaitings[1:]

// 		isSucces = l.TryLock(curWating, &waitings)
// 		select {
// 		case curWating.isSucces <- isSucces:
// 		default:
// 		}
// 	}
// }

// func (l *KeyLock) LockKeys(keys []string, cancel <-chan struct{}) (canceled bool, unlock func()) {
// 	req := Req{
// 		keys:     keys,
// 		cancel:   cancel,
// 		isSucces: make(chan bool, 1),
// 	}

// 	select {
// 	case l.req <- req:
// 		select {
// 		case isSucces := <-req.isSucces:
// 			canceled = !isSucces
// 		case <-cancel:
// 			canceled = true
// 		}
// 	case <-cancel:
// 		canceled = true
// 	}

// 	unlock = func() {
// 		if !canceled {
// 			ReleaseLocks(l, keys, true)
// 		}
// 	}
// 	return canceled, unlock
// }

// ====================================================================

package keylock

import (
	"sync"
	"time"
)

//	type Req struct {
//		keys     []string
//		cancel   <-chan struct{}
//		isSucces chan bool
//	}
type KeyLock struct {
	muMap map[string]chan struct{}
	mu    sync.Mutex
}

func New() *KeyLock {
	k := KeyLock{
		muMap: make(map[string]chan struct{}),
	}
	return &k
}

func ReleaseLocks(l *KeyLock, keys []string) {
	for _, key := range keys {
		select {
		case l.muMap[key] <- struct{}{}:
		default:
		}
	}
}

func (l *KeyLock) LockKeys(keys []string, cancel <-chan struct{}) (canceled bool, unlock func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	curLocks := make([]string, 0)
	for _, key := range keys {
		_, ok := l.muMap[key]
		if !ok {
			l.muMap[key] = make(chan struct{}, 1)
			l.muMap[key] <- struct{}{}
		}
		select {
		case <-l.muMap[key]:
			curLocks = append(curLocks, key)
		case <-time.After(100 * time.Millisecond):
			ReleaseLocks(l, curLocks)
			return true, nil
		case <-cancel:
			ReleaseLocks(l, curLocks)
			return true, nil
		}
	}
	unlock = func() {
		ReleaseLocks(l, curLocks)
	}
	return false, unlock
}
