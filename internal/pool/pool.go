package pool

import "sync"

// Resetter — ограничение для типов, умеющих сбрасывать своё состояние.
type Resetter interface {
	Reset()
}

// Pool — типобезопасный пул объектов типа T. Создаем через New.
type Pool[T Resetter] struct {
	pool sync.Pool
}

// New создаёт пул. newFunc вызывается, когда пул пуст.
func New[T Resetter](newFunc func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return newFunc()
			},
		},
	}
}

// Get возвращает объект из пула или создаёт новый, если пул пуст.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put сбрасывает состояние объекта и возвращает его в пул.
func (p *Pool[T]) Put(v T) {
	v.Reset()
	p.pool.Put(v)
}
