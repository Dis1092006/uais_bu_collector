package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"uais_bu_collector/helper"
	"uais_bu_collector/log"
	"uais_bu_collector/workmanager"
)

var (
	cfg *helper.Config
	//startTime		= time.Now().Round(time.Second)

	//эти переменные заполняются линкером.
	//чтобы их передать надо компилировать программу с ключами
	//go build -ldflags "-X main.buildtime '2015-12-22' -X main.version 'v1.0'"
	version   = "debug build"
	buildtime = "n/a"
)

//----------------------------------------------------------------------------------------------------------------------
// Основная функция программы
//----------------------------------------------------------------------------------------------------------------------
func main() {
	var err error

	// Загрузка конфигурации
	cfg, err = helper.ReloadConfig(helper.ConfigFileName)
	if err != nil {
		if err != helper.ErrNotModified {
			fmt.Errorf("Не удалось загрузить %s: %s", helper.ConfigFileName, err)
			return
		}
	}

	// Инициализация логгера
	if err := log.InitLogger(cfg); err != nil {
		fmt.Errorf(err.Error())
		return
	}

	log.Infof("Версия: %s. Собрано %s", version, buildtime)
	log.Debugf("Конфигурация: %#v", cfg)

	// Запуск рабочего цикла
	workmanager.Startup(cfg)

	// Контроль завершения программы по Ctrl-C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, syscall.SIGTERM)
	for {
		select {
		case <-sigChan:
			workmanager.Shutdown()
			return
		}
	}
}
