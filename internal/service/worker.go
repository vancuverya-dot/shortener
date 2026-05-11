package service

import (
	"context"
	"sync"
)

type DeleteTask struct {
	ShortURLs []string
	UserID    string
}

type Worker struct {
	tasks  chan DeleteTask
	done   chan struct{}
	delete func(ctx context.Context, shortURLs []string, userID string) error
}

func NewWorker(deleteFunc func(ctx context.Context, shortURLs []string, userID string) error) *Worker {
	w := &Worker{
		tasks:  make(chan DeleteTask, 1024),
		done:   make(chan struct{}),
		delete: deleteFunc,
	}
	go w.run()
	return w
}

func (w *Worker) Add(task DeleteTask) {
	w.tasks <- task
}

func (w *Worker) Stop() {
	close(w.done)
}

func (w *Worker) run() {
	for {
		select {
		case <-w.done:
			return
		case task := <-w.tasks:
			allURLs := task.ShortURLs
			userID := task.UserID
		drain:
			for {
				select {
				case t := <-w.tasks:
					if t.UserID == userID {
						allURLs = append(allURLs, t.ShortURLs...)
					} else {
						w.tasks <- t
						break drain
					}
				default:
					break drain
				}
			}

			ctx := context.Background()
			w.delete(ctx, allURLs, userID)
		}
	}
}

func FanIn(done <-chan struct{}, channels ...<-chan DeleteTask) <-chan DeleteTask {
	out := make(chan DeleteTask)
	var wg sync.WaitGroup

	output := func(ch <-chan DeleteTask) {
		defer wg.Done()
		for task := range ch {
			select {
			case out <- task:
			case <-done:
				return
			}
		}
	}

	wg.Add(len(channels))
	for _, ch := range channels {
		go output(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
