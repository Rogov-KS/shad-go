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
	o.mu.Lock()
	o.cancel = cancel
	o.mu.Unlock()

	go func() {
		res, err := cb(copyCtx)
		o.mu.Lock()
		o.res = callResult{res: res, err: err}
		o.mu.Unlock()
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
	// Ленивая инициализация cond с мьютексом
	o.mu.Lock()
	if o.cond.L == nil {
		o.cond = *sync.NewCond(&o.mu)
	}
	o.mu.Unlock()

	var ch chan struct{}
	if atomic.CompareAndSwapInt32(&o.isRunning, 0, 1) {
		atomic.AddInt64(&o.cntRunning, 1)
		ch = o.LazyInit(ctx, cb)
	} else {
		o.mu.Lock()
		for o.ch == nil {
			o.cond.Wait()
			// Проверяем контекст после каждого пробуждения
			if ctx.Err() != nil {
				o.mu.Unlock()
				return nil, ctx.Err()
			}
		}
		ch = o.ch
		o.mu.Unlock()
		// Проверяем контекст перед увеличением счетчика
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		atomic.AddInt64(&o.cntRunning, 1)
	}

	// Важно: сначала проверяем результат, потом отмену
	// Это гарантирует, что если результат готов, мы его получим
	select {
	case <-ch:
		// Канал закрыт, результат готов в o.res
		cnt := atomic.AddInt64(&o.cntRunning, -1)
		o.mu.Lock()
		res := o.res
		o.mu.Unlock()
		if cnt == 0 {
			o.mu.Lock()
			if o.cancel != nil {
				o.cancel()
				o.cancel = nil
			}
			o.mu.Unlock()
			atomic.StoreInt32(&o.isRunning, 0)
		}
		return res.res, res.err
	case <-ctx.Done():
		cnt := atomic.AddInt64(&o.cntRunning, -1)
		if cnt == 0 {
			o.mu.Lock()
			if o.cancel != nil {
				o.cancel()
				o.cancel = nil
			}
			o.mu.Unlock()
			atomic.StoreInt32(&o.isRunning, 0)
		}
		// Проверяем, может результат уже готов
		select {
		case <-ch:
			// Канал закрыт, результат готов в o.res
			o.mu.Lock()
			res := o.res
			o.mu.Unlock()
			return res.res, res.err
		default:
			return nil, ctx.Err()
		}
	}
}
