package log

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"uais_bu_collector/helper"

	"github.com/Sirupsen/logrus"
)

var (
	logger   *logrus.Logger
	initOnce sync.Once
)

//----------------------------------------------------------------------------------------------------------------------
// Инициализация логгера
//----------------------------------------------------------------------------------------------------------------------
func InitLogger(cfg *helper.Config) error {

	// Инициализация объекта
	if logger == nil {
		logger = logrus.New()
	}
	// Инициализация файла лога - выполняется однократно
	initOnce.Do(func() {
		file, err := os.OpenFile(cfg.LogFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			panic(err)
		}
		logger.Out = file
	})
	// Настройка уровня логирования
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		logger.Level = logrus.DebugLevel
	case "INFO":
		logger.Level = logrus.InfoLevel
	case "ERROR":
		logger.Level = logrus.ErrorLevel
	default:
		return fmt.Errorf("Неизвестный уровень лога, %s", cfg.LogLevel)
	}
	// Настройка формата времени
	logger.Formatter = &logrus.TextFormatter{TimestampFormat: "2006-01-02 15:04:05"}
	// Информационное сообщение
	logger.Infoln("Уровень логирования", cfg.LogLevel)
	// Ошибок не было
	return nil
}

//----------------------------------------------------------------------------------------------------------------------
// Функции - обёртки
//----------------------------------------------------------------------------------------------------------------------
func Debug(args ...interface{}) {
	logger.Debug(args...)
}

func Info(args ...interface{}) {
	logger.Info(args...)
}

func Warn(args ...interface{}) {
	logger.Warn(args...)
}

func Error(args ...interface{}) {
	logger.Error(args...)
}

func Fatal(args ...interface{}) {
	logger.Fatal(args...)
}

func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	logger.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	logger.Fatalf(format, args...)
}
