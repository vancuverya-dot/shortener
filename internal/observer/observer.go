package observer

import (
	"time"
)

// Action — тип действия, по которому формируется событие аудита.
type Action string

const (
	ActionShorten Action = "shorten"
	ActionFollow  Action = "follow"
)

// Event — событие аудита, отправляемое всем наблюдателям.
type Event struct {
	Timestamp int64  `json:"ts"`
	Action    Action `json:"action"`
	UserID    string `json:"user_id"`
	URL       string `json:"url"`
}

// NewEvent создаёт событие аудита с текущей временной меткой.
func NewEvent(action Action, userID, url string) Event {
	return Event{
		Timestamp: time.Now().Unix(),
		Action:    action,
		UserID:    userID,
		URL:       url,
	}
}

// Observer — наблюдатель, получающий события аудита.
type Observer interface {
	Handle(event Event) error
}

// asyncObserver оборачивает Observer, добавляя асинхронную обработку
// через буферизованный канал и фоновую горутину-воркер.
// Горутина запускается один раз при создании и живёт до вызова stop.
type asyncObserver struct {
	inner  Observer
	queue  chan Event
	done   chan struct{}
	logger func(err error)
}

// newAsyncObserver запускает фоновую горутину для наблюдателя inner.
// bufSize — размер буфера очереди; события, пришедшие при заполненном
// буфере, молча отбрасываются (drop), чтобы не блокировать хэндлер.
// logger вызывается при ошибке Handle; передайте nil, чтобы игнорировать ошибки.
func newAsyncObserver(inner Observer, bufSize int, logger func(err error)) *asyncObserver {
	a := &asyncObserver{
		inner:  inner,
		queue:  make(chan Event, bufSize),
		done:   make(chan struct{}),
		logger: logger,
	}
	go a.run()
	return a
}

func (a *asyncObserver) run() {
	for {
		select {
		case event := <-a.queue:
			if err := a.inner.Handle(event); err != nil && a.logger != nil {
				a.logger(err)
			}
		case <-a.done:
			// Дочитываем оставшиеся события перед выходом.
			for {
				select {
				case event := <-a.queue:
					if err := a.inner.Handle(event); err != nil && a.logger != nil {
						a.logger(err)
					}
				default:
					return
				}
			}
		}
	}
}

func (a *asyncObserver) stop() {
	close(a.done)
}

// send неблокирующе кладёт событие в очередь.
// Если буфер заполнен, событие отбрасывается.
func (a *asyncObserver) send(event Event) {
	select {
	case a.queue <- event:
	default:
	}
}

// Subject — издатель событий аудита.
// Каждый подписанный наблюдатель получает события асинхронно
// через собственную горутину и буферизованный канал, поэтому
// медленный наблюдатель не блокирует ни хэндлер, ни других наблюдателей.
// Количество горутин фиксировано: по одной на каждый Subscribe.
type Subject struct {
	observers []*asyncObserver
}

// NewSubject создаёт пустой издатель событий аудита.
func NewSubject() *Subject {
	return &Subject{}
}

// Subscribe подписывает наблюдателя на события аудита.
// bufSize задаёт глубину очереди событий для этого наблюдателя;
// logger вызывается при ошибке Handle.
func (s *Subject) Subscribe(o Observer, bufSize int, logger func(err error)) {
	s.observers = append(s.observers, newAsyncObserver(o, bufSize, logger))
}

// Notify неблокирующе отправляет событие всем подписанным наблюдателям.
// Возврат происходит немедленно, не дожидаясь обработки.
func (s *Subject) Notify(event Event) {
	for _, o := range s.observers {
		o.send(event)
	}
}

// Stop останавливает все фоновые горутины наблюдателей,
// дочитывая оставшиеся в очередях события.
func (s *Subject) Stop() {
	for _, o := range s.observers {
		o.stop()
	}
}
