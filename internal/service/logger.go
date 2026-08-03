package service

import (
	"sync"

	"go.uber.org/zap"
)

var (
	Log  *zap.SugaredLogger
	once sync.Once
)

func InitConsoleLogger() {
	once.Do(func() {

		baseLogger, _ := zap.NewDevelopment()
		Log = baseLogger.Sugar()
	})
}

func SyncConsoleLogger() {
	if Log != nil {
		_ = Log.Sync()
	}
}
