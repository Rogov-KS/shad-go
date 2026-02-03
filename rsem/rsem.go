//go:build !solution

package rsem

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type Semaphore struct {
	rdb redis.UniversalClient
}

func NewSemaphore(rdb redis.UniversalClient) *Semaphore {
	return &Semaphore{rdb: rdb}
}

// Acquire semaphore associated with key. No more than limit processes can hold semaphore at the same time.
func (s *Semaphore) Acquire(
	ctx context.Context,
	key string,
	limit int,
) (release func() error, err error) {
	// TTL для robustness - автоматическое освобождение при смерти процесса
	const ttlSeconds = 1

	// Генерируем уникальный идентификатор для этого процесса
	processID := make([]byte, 16)
	if _, err := rand.Read(processID); err != nil {
		return nil, err
	}
	processIDStr := hex.EncodeToString(processID)
	processKey := key + ":" + processIDStr

	acquireScript := redis.NewScript(`
		-- Проверяем, существует ли уже ключ для этого процесса
		if redis.call('EXISTS', KEYS[2]) == 1 then
			-- Ключ уже существует, обновляем TTL
			redis.call('EXPIRE', KEYS[2], ARGV[2])
			return 0
		end
		
		-- Подсчитываем количество активных процессов
		-- Используем SCAN для подсчета ключей с паттерном
		-- Важно: проверяем TTL > 0, чтобы исключить истекшие ключи
		local count = 0
		local cursor = "0"
		repeat
			local result = redis.call('SCAN', cursor, 'MATCH', KEYS[1] .. ":*", 'COUNT', 100)
			cursor = result[1]
			local keys = result[2]
			for i = 1, #keys do
				-- Проверяем, что ключ существует и имеет положительный TTL
				if redis.call('EXISTS', keys[i]) == 1 then
					local ttl = redis.call('TTL', keys[i])
					if ttl > 0 then
						count = count + 1
					end
				end
			end
		until cursor == "0"
		
		-- Проверяем, можем ли мы получить семафор
		-- Важно: проверяем count < limit, а не count <= limit
		if count < tonumber(ARGV[1]) then
			-- Создаем ключ для этого процесса с TTL
			-- Используем SET с NX (только если не существует) для атомарности
			local created = redis.call('SET', KEYS[2], '1', 'EX', ARGV[2], 'NX')
			if created then
				-- Ключ создан, проверяем еще раз количество активных процессов
				-- чтобы убедиться, что мы не превысили лимит
				local count2 = 0
				local cursor2 = "0"
				repeat
					local result2 = redis.call('SCAN', cursor2, 'MATCH', KEYS[1] .. ":*", 'COUNT', 100)
					cursor2 = result2[1]
					local keys2 = result2[2]
					for i = 1, #keys2 do
						if redis.call('EXISTS', keys2[i]) == 1 then
							local ttl2 = redis.call('TTL', keys2[i])
							if ttl2 > 0 then
								count2 = count2 + 1
							end
						end
					end
				until cursor2 == "0"
				
				if count2 <= tonumber(ARGV[1]) then
					return 0
				else
					-- Превысили лимит, удаляем созданный ключ
					redis.call('DEL', KEYS[2])
					return 1
				end
			else
				-- Ключ был создан другим процессом между проверкой и созданием
				-- Проверяем еще раз
				if redis.call('EXISTS', KEYS[2]) == 1 then
					redis.call('EXPIRE', KEYS[2], ARGV[2])
					return 0
				end
				return 1
			end
		else
			-- Все слоты заняты
			return 1
		end
	`)

	// Пытаемся получить семафор в цикле с ожиданием
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Проверяем контекст перед каждой попыткой
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		result, err := acquireScript.Run(ctx, s.rdb, []string{key, processKey}, limit, ttlSeconds).Int()
		if err != nil {
			// Если контекст был отменен во время выполнения скрипта, возвращаем ошибку контекста
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}

		if result == 0 {
			// Успешно получили семафор
			break
		}

		// Семафор занят, ждем и пробуем снова
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			// Продолжаем цикл и пробуем снова
		}
	}

	var releaseOnce sync.Once
	released := make(chan struct{})
	refreshStop := make(chan struct{})

	doRelease := func() {
		releaseOnce.Do(func() {
			close(released)
			close(refreshStop)
			// Удаляем ключ процесса
			_ = s.rdb.Del(context.Background(), processKey).Err()
		})
	}

	release = func() error {
		doRelease()
		return nil
	}

	// Запускаем горутину для периодического обновления TTL
	// Обновляем TTL каждые 300ms, чтобы ключ не истекал
	go func() {
		ticker := time.NewTicker(time.Millisecond * 300)
		defer ticker.Stop()
		for {
			select {
			case <-refreshStop:
				return
			case <-ticker.C:
				// Обновляем TTL ключа процесса
				_ = s.rdb.Expire(context.Background(), processKey, time.Second*time.Duration(ttlSeconds)).Err()
			}
		}
	}()

	// Запускаем горутину для автоматического освобождения при отмене контекста
	go func() {
		select {
		case <-ctx.Done():
			// Контекст отменен, автоматически освобождаем семафор
			doRelease()
		case <-released:
			// Семафор уже освобожден явно
		}
	}()

	return release, nil
}
