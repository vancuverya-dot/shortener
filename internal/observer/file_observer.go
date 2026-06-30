package observer

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// FileObserver — наблюдатель, дописывающий события аудита в конец файла,
// каждое событие на новой строке. Файл держится открытым всё время жизни
// наблюдателя и закрывается явным вызовом Close при завершении программы.
type FileObserver struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileObserver открывает файл по указанному пути и создаёт наблюдателя.
// Возвращает ошибку, если файл не удалось открыть.
// Вызывающий код обязан вызвать Close при завершении работы.
func NewFileObserver(path string) (*FileObserver, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("audit: open file %q: %w", path, err)
	}
	return &FileObserver{file: file}, nil
}

// Handle сериализует событие в JSON и дописывает его в файл новой строкой.
// Под мьютексом выполняется только сама операция записи.
func (f *FileObserver) Handle(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("audit: marshal event: %w", err)
	}
	data = append(data, '\n')

	f.mu.Lock()
	_, err = f.file.Write(data)
	f.mu.Unlock()

	if err != nil {
		return fmt.Errorf("audit: write to file: %w", err)
	}
	return nil
}

// Close закрывает файл аудита. Должен вызываться при завершении программы.
func (f *FileObserver) Close() error {
	return f.file.Close()
}
