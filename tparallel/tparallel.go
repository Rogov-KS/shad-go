//go:build !solution

package tparallel

import (
	"sync"
)

type T struct {
	isEnd         chan struct{}
	isParallel    chan struct{}
	wg            sync.WaitGroup // Для ожидания всех (особенно паралельных) тестов
	waitParallel  *sync.Cond     // Для реализации t.Parallel
	cWaitParallel *sync.Cond     // Для пробуждения запущенных паралельных под-тестов после последовательных тестов
}

func New() *T {
	t := &T{
		isEnd:      make(chan struct{}),
		isParallel: make(chan struct{}),
	}
	var mu, cmu sync.Mutex
	t.waitParallel = sync.NewCond(&mu)
	t.cWaitParallel = sync.NewCond(&cmu)
	return t
}

func (t *T) Parallel() {
	close(t.isParallel)
	if t.waitParallel != nil {
		t.waitParallel.L.Lock()
		defer t.waitParallel.L.Unlock()
		t.waitParallel.Wait()
	}
}

func (t *T) Run(subtest func(t *T)) {
	t.wg.Add(1)

	subT := New()
	subT.waitParallel = t.cWaitParallel

	go func() {
		subtest(subT)
		subT.cWaitParallel.Broadcast()
		subT.wg.Wait()
		t.wg.Done()
		close(subT.isEnd)
	}()

	select {
	case <-subT.isEnd:
		// Функция под-теста завершилась
	case <-subT.isParallel:
		// Под-тест стал параллельным, продолжаем
	}
}

func Run(topTests []func(t *T)) {
	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	tLst := make([]*T, 0)

	// Запускаем все top-level тесты последовательно
	for _, test := range topTests {
		t := New()
		t.waitParallel = cond

		tLst = append(tLst, t)

		t.wg.Add(1)
		go func() {
			test(t)
			t.wg.Done()
			close(t.isEnd)
		}()

		select {
		case <-t.isEnd:
			// Тест завершился последовательно
		case <-t.isParallel:
			// Тест стал параллельным, продолжаем к следующему
		}
	}

	cond.Broadcast()
	for _, t := range tLst {
		t.wg.Wait()
	}
}
