//go:build !solution

package dupcall

import (
	"context"
	"sync"
	"sync/atomic"
)

type callResult struct {
	res interface{}
	err error
}

type Call struct {
	isRunning  int32
	cntRunning int64
	cancel     context.CancelFunc
	ch         chan struct{}
	res        callResult
	mu         sync.Mutex
	cond       sync.Cond
}

func (o *Call) LazyInit(
	ctx context.Context,
	cb func(context.Context) (interface{}, error),
) chan struct{} {
	// Создаём новый канал для каждого нового вызова
	o.mu.Lock()
	o.ch = make(chan struct{})
	o.cond.Broadcast()
	o.mu.Unlock()

	copyCtx, cancel := context.WithCancel(context.Background())
	o.cancel = cancel
	go func() {
		res, err := cb(copyCtx)
		o.res = callResult{res: res, err: err}
		atomic.StoreInt32(&o.isRunning, 0)
		// Закрываем канал, чтобы все ожидающие горутины получили сигнал
		close(o.ch)
	}()
	return o.ch
}

func (o *Call) Do(
	ctx context.Context,
	cb func(context.Context) (interface{}, error),
) (result interface{}, err error) {
	atomic.AddInt64(&o.cntRunning, 1)

	// Ленивая инициализация cond с мьютексом
	o.mu.Lock()
	if o.cond.L == nil {
		o.cond = *sync.NewCond(&o.mu)
	}
	o.mu.Unlock()

	var ch chan struct{}
	if atomic.CompareAndSwapInt32(&o.isRunning, 0, 1) {
		ch = o.LazyInit(ctx, cb)
	} else {
		o.mu.Lock()
		for o.ch == nil {
			o.cond.Wait()
		}
		ch = o.ch
		o.mu.Unlock()
	}

	// Важно: сначала проверяем результат, потом отмену
	// Это гарантирует, что если результат готов, мы его получим
	select {
	case <-ch:
		// Канал закрыт, результат готов в o.res
		cnt := atomic.AddInt64(&o.cntRunning, -1)
		if cnt == 0 {
			if o.cancel != nil {
				o.cancel()
			}
		}
		return o.res.res, o.res.err
	case <-ctx.Done():
		cnt := atomic.AddInt64(&o.cntRunning, -1)
		if cnt == 0 {
			if o.cancel != nil {
				o.cancel()
			}
		}
		// Проверяем, может результат уже готов
		select {
		case <-ch:
			// Канал закрыт, результат готов в o.res
			return o.res.res, o.res.err
		default:
			return nil, ctx.Err()
		}
	}
}
