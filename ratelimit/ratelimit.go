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
		l.runUnlimited()
		return
	}

	// Периодически очищаем старые временные метки
	cleanupInterval := max(l.interval/10, time.Millisecond*100)
	cleanupTicker := time.NewTicker(cleanupInterval)
	defer cleanupTicker.Stop()

	for {
		currentTime := now()

		// Удаляем временные метки старше interval
		timestamps = l.cleanupOldTimestamps(timestamps, currentTime)

		// Пробуждаем ожидающих, если есть место
		timestamps, waiting = l.processWaitingRequests(timestamps, waiting, currentTime)

		// Обрабатываем события (новые запросы, тикер очистки или остановку)
		var shouldStop bool
		timestamps, waiting, shouldStop = l.processEvents(timestamps, waiting, currentTime, cleanupTicker)
		if shouldStop {
			return
		}
	}
}

// runUnlimited обрабатывает запросы без ограничений (когда interval == 0)
func (l *Limiter) runUnlimited() {
	for {
		select {
		case req := <-l.requestCh:
			l.sendResponse(req, nil)

		case <-l.stopCh:
			// Обрабатываем все оставшиеся запросы
			for {
				select {
				case req := <-l.requestCh:
					l.sendResponse(req, ErrStopped)
				default:
					return
				}
			}
		}
	}
}

// cleanupOldTimestamps удаляет временные метки старше interval
func (l *Limiter) cleanupOldTimestamps(timestamps []time.Time, currentTime time.Time) []time.Time {
	cutoff := currentTime.Add(-l.interval)
	for len(timestamps) > 0 && timestamps[0].Before(cutoff) {
		timestamps = timestamps[1:]
	}
	return timestamps
}

// processWaitingRequests пробуждает ожидающие запросы, если есть место
func (l *Limiter) processWaitingRequests(timestamps []time.Time, waiting []acquireRequest, currentTime time.Time) ([]time.Time, []acquireRequest) {
	for len(waiting) > 0 && len(timestamps) < l.maxCount {
		req := waiting[0]
		waiting = waiting[1:]

		if l.isContextDone(req.ctx) {
			req.response <- req.ctx.Err()
			continue
		}

		timestamps = append(timestamps, currentTime)
		req.response <- nil
	}
	return timestamps, waiting
}

// processEvents обрабатывает события: новые запросы, тикер очистки или остановку
func (l *Limiter) processEvents(timestamps []time.Time, waiting []acquireRequest, currentTime time.Time, cleanupTicker *time.Ticker) ([]time.Time, []acquireRequest, bool) {
	select {
	case req := <-l.requestCh:
		timestamps, waiting = l.handleNewRequest(req, timestamps, waiting, currentTime)
		return timestamps, waiting, false

	case <-cleanupTicker.C:
		// Периодическая очистка - уже сделано выше в цикле, просто продолжаем цикл
		return timestamps, waiting, false

	case <-l.stopCh:
		l.handleStop(waiting)
		return timestamps, waiting, true
	}
}

// handleNewRequest обрабатывает новый запрос
func (l *Limiter) handleNewRequest(req acquireRequest, timestamps []time.Time, waiting []acquireRequest, currentTime time.Time) ([]time.Time, []acquireRequest) {
	if l.isContextDone(req.ctx) {
		req.response <- req.ctx.Err()
		return timestamps, waiting
	}

	// Удаляем старые временные метки перед проверкой
	timestamps = l.cleanupOldTimestamps(timestamps, currentTime)

	if len(timestamps) < l.maxCount {
		timestamps = append(timestamps, currentTime)
		req.response <- nil
	} else {
		waiting = append(waiting, req)
	}
	return timestamps, waiting
}

// handleStop обрабатывает остановку лимитера
func (l *Limiter) handleStop(waiting []acquireRequest) {
	// Отправляем ошибку всем ожидающим
	for _, req := range waiting {
		l.sendResponse(req, ErrStopped)
	}

	// Обрабатываем все оставшиеся запросы в канале
	// Используем таймаут, чтобы не зависнуть навсегда
	for {
		select {
		case req := <-l.requestCh:
			l.sendResponse(req, ErrStopped)
		case <-time.After(time.Millisecond * 100):
			// Если нет новых запросов за 100ms, выходим
			return
		}
	}
}

// sendResponse отправляет ответ на запрос с учетом контекста
func (l *Limiter) sendResponse(req acquireRequest, err error) {
	select {
	case <-req.ctx.Done():
		req.response <- req.ctx.Err()
	default:
		req.response <- err
	}
}

// isContextDone проверяет, отменен ли контекст
func (l *Limiter) isContextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
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
