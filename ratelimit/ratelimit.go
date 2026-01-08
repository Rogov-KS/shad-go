//go:build !solution

package ratelimit

import (
	"context"
	"errors"
	"time"
)

// Limiter is precise rate limiter with context support.
type Limiter struct {
	requestCh chan acquireRequest
	stopCh    chan struct{}
	maxCount  int
	interval  time.Duration
}

type acquireRequest struct {
	ctx      context.Context
	response chan error
}

var ErrStopped = errors.New("limiter stopped")

// NewLimiter returns limiter that throttles rate of successful Acquire() calls
// to maxSize events at any given interval.
func NewLimiter(maxCount int, interval time.Duration) *Limiter {
	l := &Limiter{
		requestCh: make(chan acquireRequest),
		stopCh:    make(chan struct{}),
		maxCount:  maxCount,
		interval:  interval,
	}

	go l.run()
	return l
}

func (l *Limiter) run() {
	// Очередь временных меток разрешенных запросов
	var timestamps []time.Time
	var waiting []acquireRequest
	now := time.Now

	// Если interval == 0, разрешаем все запросы сразу
	if l.interval == 0 {
		for {
			select {
			case req := <-l.requestCh:
				select {
				case <-req.ctx.Done():
					req.response <- req.ctx.Err()
				default:
					req.response <- nil
				}
			case <-l.stopCh:
				// Обрабатываем все оставшиеся запросы
				for {
					select {
					case req := <-l.requestCh:
						select {
						case <-req.ctx.Done():
							req.response <- req.ctx.Err()
						default:
							req.response <- ErrStopped
						}
					default:
						return
					}
				}
			}
		}
	}

	// Периодически очищаем старые временные метки
	cleanupInterval := l.interval / 10
	if cleanupInterval < time.Millisecond*100 {
		cleanupInterval = time.Millisecond * 100
	}
	cleanupTicker := time.NewTicker(cleanupInterval)
	defer cleanupTicker.Stop()

	for {
		currentTime := now()

		// Удаляем временные метки старше interval
		cutoff := currentTime.Add(-l.interval)
		for len(timestamps) > 0 && timestamps[0].Before(cutoff) {
			timestamps = timestamps[1:]
		}

		// Пробуждаем ожидающих, если есть место
		for len(waiting) > 0 && len(timestamps) < l.maxCount {
			req := waiting[0]
			waiting = waiting[1:]

			select {
			case <-req.ctx.Done():
				req.response <- req.ctx.Err()
				continue
			default:
			}

			timestamps = append(timestamps, currentTime)
			req.response <- nil
		}

		select {
		case req := <-l.requestCh:
			// Проверяем контекст сразу
			select {
			case <-req.ctx.Done():
				req.response <- req.ctx.Err()
				continue
			default:
			}

			// Удаляем старые временные метки перед проверкой
			cutoff = currentTime.Add(-l.interval)
			for len(timestamps) > 0 && timestamps[0].Before(cutoff) {
				timestamps = timestamps[1:]
			}

			if len(timestamps) < l.maxCount {
				timestamps = append(timestamps, currentTime)
				req.response <- nil
			} else {
				waiting = append(waiting, req)
			}

		case <-cleanupTicker.C:
			// Периодическая очистка - уже сделано выше в цикле
			continue

		case <-l.stopCh:
			// Отправляем ошибку всем ожидающим
			for _, req := range waiting {
				select {
				case <-req.ctx.Done():
					req.response <- req.ctx.Err()
				default:
					req.response <- ErrStopped
				}
			}
			// Обрабатываем все оставшиеся запросы в канале
			// Используем таймаут, чтобы не зависнуть навсегда
			for {
				select {
				case req := <-l.requestCh:
					select {
					case <-req.ctx.Done():
						req.response <- req.ctx.Err()
					default:
						req.response <- ErrStopped
					}
				case <-time.After(time.Millisecond * 100):
					// Если нет новых запросов за 100ms, выходим
					return
				}
			}
		}
	}
}

func (l *Limiter) Acquire(ctx context.Context) error {
	req := acquireRequest{
		ctx:      ctx,
		response: make(chan error, 1),
	}

	// Отправляем запрос в управляющую горутину
	select {
	case l.requestCh <- req:
		// Запрос отправлен, ждем ответа
		select {
		case err := <-req.response:
			return err
		case <-ctx.Done():
			// Контекст отменен во время ожидания ответа
			// Управляющая горутина проверит контекст при обработке
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-l.stopCh:
		return ErrStopped
	}
}

func (l *Limiter) Stop() {
	close(l.stopCh)
}
