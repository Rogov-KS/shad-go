//go:build !solution

package lrucache

import "container/list"

// entry представляет элемент кэша с ключом и значением
type entry struct {
	key   int
	value int
}

// LRUCache реализует интерфейс Cache
type LRUCache struct {
	capacity int
	cache    map[int]*list.Element // map для быстрого доступа к элементам списка
	list     *list.List            // двусвязный список для отслеживания порядка доступа
}

// Get возвращает значение по ключу и обновляет его access time
func (l *LRUCache) Get(key int) (int, bool) {
	elem, ok := l.cache[key]
	if !ok {
		return 0, false
	}

	// Перемещаем элемент в конец списка (делаем его самым новым)
	l.list.MoveToBack(elem)

	return elem.Value.(*entry).value, true
}

// Set обновляет или добавляет значение по ключу
func (l *LRUCache) Set(key, value int) {
	// Если capacity <= 0, ничего не делаем
	if l.capacity <= 0 {
		return
	}

	// Если ключ уже существует, обновляем значение и перемещаем в конец
	if elem, ok := l.cache[key]; ok {
		elem.Value.(*entry).value = value
		l.list.MoveToBack(elem)
		return
	}

	// Если достигнут лимит, удаляем самый старый элемент (head списка)
	if l.list.Len() >= l.capacity {
		oldest := l.list.Front()
		if oldest != nil {
			oldEntry := oldest.Value.(*entry)
			delete(l.cache, oldEntry.key)
			l.list.Remove(oldest)
		}
	}

	// Добавляем новый элемент в конец списка
	newEntry := &entry{key: key, value: value}
	elem := l.list.PushBack(newEntry)
	l.cache[key] = elem
}

// Range вызывает функцию f для всех элементов в порядке возрастания access time
func (l *LRUCache) Range(f func(key, value int) bool) {
	for elem := l.list.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*entry)
		if !f(entry.key, entry.value) {
			return
		}
	}
}

// Clear удаляет все элементы из кэша
func (l *LRUCache) Clear() {
	l.cache = make(map[int]*list.Element, l.capacity)
	l.list = list.New()
}

// New создает новый LRU cache с заданной емкостью
func New(cap int) Cache {
	return &LRUCache{
		capacity: cap,
		cache:    make(map[int]*list.Element, cap),
		list:     list.New(),
	}
}
