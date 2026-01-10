//go:build !solution

package dupcall

import (
	"context"
	"runtime"
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
}

func (o *Call) Do(
	ctx context.Context,
	cb func(context.Context) (interface{}, error),
) (result interface{}, err error) {
	atomic.AddInt64(&o.cntRunning, 1)
	if atomic.CompareAndSwapInt32(&o.isRunning, 0, 1) {
		// Создаём новый канал для каждого нового вызова
		o.ch = make(chan struct{})
		copyCtx, cancel := context.WithCancel(context.Background())
		o.cancel = cancel
		go func() {
			res, err := cb(copyCtx)
			o.res = callResult{res: res, err: err}
			atomic.StoreInt32(&o.isRunning, 0)
			// Закрываем канал, чтобы все ожидающие горутины получили сигнал
			close(o.ch)
		}()

	}

	// Сохраняем ссылку на канал локально, чтобы избежать гонки
	// между созданием нового канала и входом в select
	ch := o.ch
	if ch == nil {
		// Если канал ещё не создан, это означает, что мы находимся
		// в очень редкой гонке между проверкой CAS и созданием канала.
		// В этом случае ждём, пока канал не будет создан
		for ch == nil {
			select {
			case <-ctx.Done():
				cnt := atomic.AddInt64(&o.cntRunning, -1)
				if cnt == 0 {
					if o.cancel != nil {
						o.cancel()
					}
				}
				return nil, ctx.Err()
			default:
				runtime.Gosched() // Передаём управление другим горутинам
				ch = o.ch
			}
		}
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
