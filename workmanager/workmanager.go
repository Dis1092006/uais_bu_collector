package workmanager

import (
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
	"uais_bu_collector/helper"
	"uais_bu_collector/log"
)

// Тип - идентификатор рабочего потока
type WorkerID int

// Тип - команда управления рабочим потоком
type Command bool

// Тип - рабочий поток
type Worker struct {
	ID            WorkerID
	LastStateTime time.Time
	State         bool
	URL           string
	Login         string
	Password      string
	Interval      time.Duration
	CommandChan   chan Command
	Req           *http.Request
}

// Тип - cписок рабочих потоков
type WorkersList []*Worker

// workManager - синглтон, контролирует запуск и остановку рабочих потоков.
type workManager struct {
	Workers         WorkersList
	Shutdown        int32
	ShutdownChannel chan string
}

type CheckResult struct {
	CheckTime     string        `json:"time"`
	CheckDuration time.Duration `json:"duration"`
	Address       string        `json:"address"`
	StatusCode    int           `json:"status"`
	Error         string        `json:"error"`
}

var (
	wm               workManager // Reference to the singleton
	aliveWorkerChan  chan WorkerID
	workerIDSequence WorkerID = 0
)

//----------------------------------------------------------------------------------------------------------------------
// Startup приводит менеджер в рабочее состояние.
//----------------------------------------------------------------------------------------------------------------------
func Startup(cfg *helper.Config) error {
	var err error
	defer CatchPanic(&err, "main", "workmanager.Startup")

	log.Info("workmanager.Startup, Started")

	// Create the work manager to get the program going
	wm = workManager{
		Shutdown:        0,
		ShutdownChannel: make(chan string),
	}

	// Запуск рабочего цикла
	go wm.WorkingLoop(cfg)

	log.Info("workmanager.Startup, Completed")
	return err
}

//----------------------------------------------------------------------------------------------------------------------
// Shutdown аккуратно завершает работу менеджера.
//----------------------------------------------------------------------------------------------------------------------
func Shutdown() error {
	var err error
	log.Info("workmanager.Shutdown, Started")

	defer CatchPanic(&err, "main", "workmanager.Shutdown")

	// Shutdown the program
	log.Info("workmanager.Shutdown, Info : Shutting Down")
	atomic.CompareAndSwapInt32(&wm.Shutdown, 0, 1)

	log.Info("workmanager.Shutdown, Info : Shutting Down Work Timer")
	wm.ShutdownChannel <- "Down"
	<-wm.ShutdownChannel

	close(wm.ShutdownChannel)

	log.Info("workmanager.Shutdown, Completed")
	return err
}

//----------------------------------------------------------------------------------------------------------------------
// CatchPanic используется для отлова и отображения паник
//----------------------------------------------------------------------------------------------------------------------
func CatchPanic(err *error, goRoutine string, function string) {
	if r := recover(); r != nil {
		// Capture the stack trace
		buf := make([]byte, 10000)
		runtime.Stack(buf, false)

		log.Errorf(goRoutine, function, "PANIC Defered [%v] : Stack Trace : %v", r, string(buf))

		if err != nil {
			*err = fmt.Errorf("%v", r)
		}
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Вечный рабочий цикл
//----------------------------------------------------------------------------------------------------------------------
func (workManager *workManager) WorkingLoop(cfg *helper.Config) {
	log.Debugf("workingLoop, ожидание %d секунд", time.Duration(cfg.ReloadConfigInterval))

	// Поток для контроля работоспособности рабочих потоков.
	aliveWorkerChan = make(chan WorkerID)

	// Первоначальная инициализация списка рабочих потоков
	log.Debugf("len(cfg.Services) = %d", len(cfg.WebServices))
	workManager.InitWorkers(cfg)

	// Запуск рабочих потоков
	for i := 0; i < len(workManager.Workers); i++ {
		go workManager.CheckWebService(workManager.Workers[i], aliveWorkerChan, cfg.DataStorageURL)
	}

	// Включение тикера
	t := time.Tick(time.Duration(cfg.ReloadConfigInterval) * time.Second)

	for {
		log.Debug("workingLoop, очередной цикл")
		select {
		case <-workManager.ShutdownChannel:
			log.Info("workingLoop, закрытие рабочих потоков")
			workManager.CloseWorkers()
			log.Info("workingLoop, выключение контрольного потока")
			workManager.ShutdownChannel <- "Down"
			return

		case <-t:
			// Срабатывание таймера.
			log.Debug("workingLoop, срабатывание таймера")
			// Контроль необходимости закрытия.
			if workManager.Shutdown == 1 {
				log.Debug("workingLoop, workManager.Shutdown == 1")
				return
			}
			// Перезагрузка конфигурации
			cfgTmp, err := helper.ReloadConfig(helper.ConfigFileName)
			if err != nil {
				if err != helper.ErrNotModified {
					log.Fatalf("Не удалось загрузить %s: %s", helper.ConfigFileName, err)
				} else {
					log.Debugf("workingLoop, конфигурация не изменилась")
					// ToDo - контроль рабочих потоков от которых давно не было подтверждения работоспособности
				}
			} else {
				log.Info("Перезагружен конфигурационный файл")
				if err := log.InitLogger(cfgTmp); err != nil {
					log.Error(err)
				} else {
					cfg = cfgTmp
				}

				// ToDo - пересоздать тикер при изменении cfg.ReloadConfigInterval

				// Закрыть предыдущие рабочие потоки.
				workManager.CloseWorkers()

				// Создать новый набор рабочих потоков
				workManager.InitWorkers(cfg)

				// Запуск рабочих потоков
				for i := 0; i < len(workManager.Workers); i++ {
					log.Debugf("workingLoop, запуск рабочего потока с номером %d", workManager.Workers[i].ID)
					go workManager.CheckWebService(workManager.Workers[i], aliveWorkerChan, cfg.DataStorageURL)
				}
			}

		// Контрольный сигнал от рабочего потока.
		case workerID := <-aliveWorkerChan:
			log.Debugf("Контрольный сигнал от рабочего потока: %+v", workerID)
			// Обновить данные о рабочем потоке.
			for i := 0; i < len(workManager.Workers); i++ {
				if workManager.Workers[i].ID == workerID {
					// Сохранить время получения контрольного сигнала.
					workManager.Workers[i].LastStateTime = time.Now()
				}
			}
		}
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Инициализация рабочих потоков
//----------------------------------------------------------------------------------------------------------------------
func (workManager *workManager) InitWorkers(cfg *helper.Config) {
	workManager.Workers = make(WorkersList, len(cfg.WebServices))
	for i, service := range cfg.WebServices {
		//		if service.Enabled != true {
		//			continue
		//		}
		workerIDSequence = workerIDSequence + 1
		workManager.Workers[i] = new(Worker)
		workManager.Workers[i].ID = workerIDSequence
		workManager.Workers[i].State = service.Enabled
		workManager.Workers[i].URL = service.Address
		workManager.Workers[i].Login = service.Login
		workManager.Workers[i].Password = service.Password
		workManager.Workers[i].Interval = service.CheckInterval * time.Second
		workManager.Workers[i].CommandChan = make(chan Command)
		workManager.Workers[i].Req, _ = http.NewRequest("GET", workManager.Workers[i].URL, nil)
		workManager.Workers[i].Req.SetBasicAuth(workManager.Workers[i].Login, workManager.Workers[i].Password)
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Закрытие рабочих потоков
//----------------------------------------------------------------------------------------------------------------------
func (workManager *workManager) CloseWorkers() {
	for i := 0; i < len(workManager.Workers); i++ {
		log.Debugf("workingLoop, закрытие рабочего потока с номером %d", workManager.Workers[i].ID)
		if workManager.Workers[i].State {
			workManager.Workers[i].CommandChan <- true
			log.Debug("workingLoop, закрытие рабочих потоков, послана команда в поток")
			<-workManager.Workers[i].CommandChan
			log.Debug("workingLoop, закрытие рабочих потоков, получена команда из потока")
			close(workManager.Workers[i].CommandChan)
			workManager.Workers[i].State = false
		}
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Проверка работоспособности указанного web-сервиса и отправка результата в data storage
//----------------------------------------------------------------------------------------------------------------------
func (workManager *workManager) CheckWebService(worker *Worker, outerChan chan WorkerID, dataStorageURL string) {

	if worker == nil {
		log.Debug("worker == nil")
		return
	}

	log.Debugf("checkWebService [%d], запущен рабочий поток, интервал %d секунд", worker.ID, int(worker.Interval.Seconds()))

	wait := worker.Interval

	// Рабочий цикл
	for {
		log.Debugf("checkWebService [%d], ожидание %.3f секунд", worker.ID, wait.Seconds())

		if !worker.State {
			log.Infof("checkWebService [%d], выход из неактивного рабочего потока!", worker.ID)
			return
		}

		select {
		case <-worker.CommandChan:
			log.Infof("checkWebService [%d], выключение рабочего потока!", worker.ID)
			worker.CommandChan <- true
			return

		case <-time.After(wait):
			log.Debugf("checkWebService [%d], завершение ожидания", worker.ID)
			if !worker.State {
				log.Info("checkWebService [%d], выход из неактивного рабочего потока!")
				return
			}
			break
		}

		// Контроль необходимости закрытия
		if workManager.Shutdown == 1 {
			log.Debugf("checkWebService [%d], workManager.Shutdown == 1", worker.ID)
			return
		}

		// Mark the starting time
		startTime := time.Now()

		// Рабочая проверка
		//checkResult := check(worker.URL, worker.Login, worker.Password)
		checkResult := check(worker.URL, worker.Req)

		// Отправить результат проверки сборщику данных
		response, err := makeRequest("POST", dataStorageURL, checkResult)
		if err != nil {
			log.Errorf("checkWebService [%d], Ошибка отправки данных в data storage: %v", worker.ID, err)
		} else {
			defer response.Body.Close()
		}
		log.Debugf("checkWebService [%d], Результат отправки данных в data storage: %+v", worker.ID, response)

		// Отправить контрольный сигнал
		outerChan <- worker.ID

		// Mark the ending time
		endTime := time.Now()

		// Calculate the amount of time to wait to start workManager again.
		duration := endTime.Sub(startTime)
		log.Debugf("checkWebService [%d], Длительность выполнения рабочей проверки: %.3f секунд", worker.ID, duration.Seconds())
		wait = worker.Interval - duration
		log.Debugf("checkWebService [%d], Следующее ожидание: %.3f секунд", worker.ID, wait.Seconds())
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Проверка подключения к web-сервису
// возвращает true — если сервис доступен, false, если нет и текст сообщения
//----------------------------------------------------------------------------------------------------------------------
//func check(url string, login string, password string) *CheckResult {
func check(url string, req *http.Request) *CheckResult {
	// Подготовка результата работы функции проверки
	var checkResult *CheckResult = new(CheckResult)

	// Засечка времени
	checkTime := time.Now()

	// Попытка подключения
	//req, _ := http.NewRequest("GET", url, nil)
	//req.SetBasicAuth(login, password)
	client := &http.Client{}
	resp, err := client.Do(req)

	// Контроль длительности замера
	checkDuration := time.Since(checkTime)

	// Анализ результатов попытки подключения
	log.Infof("Проверка подключения к адресу: %s", url)
	if err != nil {
		checkResult.StatusCode = 0
		checkResult.Error = err.Error()
		log.Errorf("Ошибка! %v", err)
	} else {
		defer resp.Body.Close()
		checkResult.StatusCode = resp.StatusCode
		checkResult.Error = ""
		log.Infof("Успешно. Длительность запроса: %.3f секунд", checkDuration.Seconds())
	}

	// Заполнение результата проверки подключения
	checkResult.CheckTime = checkTime.Format(time.RFC3339)
	checkResult.CheckDuration = checkDuration
	checkResult.Address = url
	log.Debugf("%+v", checkResult)

	return checkResult
}
