package service

import (
	"context"
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
		tasks:  make(chan DeleteTask, 1),
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
			batch := map[string][]string{
				task.UserID: task.ShortURLs,
			}
		drain:
			for {
				select {
				case t := <-w.tasks:
					batch[t.UserID] = append(batch[t.UserID], t.ShortURLs...)
				default:
					break drain
				}
			}

			ctx := context.Background()
			for userID, urls := range batch {
				w.delete(ctx, urls, userID)
			}
		}
	}
}
