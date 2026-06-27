package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/vancuverya-dot/shortener/internal/service"
)

// URLPair описывает одну запись соответствия короткого и оригинального URL
// для сериализации в файловое хранилище в формате JSON.
type URLPair struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// SerializeToFile сохраняет содержимое urlMap в файл filename в формате JSON
// (список объектов URLPair с отступами). Используется для персистентности
// сервиса в режиме без подключения к базе данных. Возвращает ошибку, если
// не удалось сериализовать данные или записать файл на диск.
func SerializeToFile(filename string, urlMap map[string]string) error {

	service.Log.Infof("Сохранение данных в файл: %s\n", filename)

	pairs := make([]URLPair, 0, len(urlMap))
	for shortURL, originalURL := range urlMap {
		pairs = append(pairs, URLPair{
			ShortURL:    shortURL,
			OriginalURL: originalURL,
		})
	}

	jsonData, err := json.MarshalIndent(pairs, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка сериализации JSON: %w", err)
	}

	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("ошибка записи в файл: %w", err)
	}

	return nil
}

// DeserializeFromFile читает файл filename, созданный SerializeToFile,
// и возвращает карту соответствий короткого URL оригинальному.
// Если файл не существует или пуст, возвращает пустую карту без ошибки —
// это штатный случай первого запуска сервиса. Возвращает ошибку только
// при сбое чтения файла или некорректном формате JSON.
func DeserializeFromFile(filename string) (map[string]string, error) {

	jsonData, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("ошибка чтения файла: %w", err)
	}

	if len(jsonData) == 0 {
		return make(map[string]string), nil
	}

	var pairs []URLPair
	err = json.Unmarshal(jsonData, &pairs)
	if err != nil {
		return nil, fmt.Errorf("ошибка десериализации JSON: %w", err)
	}

	urlMap := make(map[string]string)
	for _, pair := range pairs {
		urlMap[pair.ShortURL] = pair.OriginalURL
	}

	return urlMap, nil
}
