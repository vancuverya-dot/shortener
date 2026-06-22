package observer

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type FileObserver struct {
	mu   sync.Mutex
	path string
}

func NewFileObserver(path string) *FileObserver {
	return &FileObserver{path: path}
}

func (f *FileObserver) Handle(event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("audit: marshal event: %w", err)
	}

	file, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("audit: open file %q: %w", f.path, err)
	}
	defer file.Close()

	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("audit: write to file %q: %w", f.path, err)
	}

	return nil
}
