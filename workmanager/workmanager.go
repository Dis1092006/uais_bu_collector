package workmanager

import (
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
	"uais_bu_collector/helper"
	"uais_bu_collector/log"
	"uais_bu_collector/worker"
	"database/sql"
)

// Тип - результат проверки
type DatabaseCheckResult struct {
	DatabaseId  int     `json:"database_id"`
	FileName    string  `json:"file_name"`
	BackupDate  string  `json:"backup_date"`
	BackupType  string  `json:"backup_type"`
	BackupSize  int     `json:"backup_size"`
}

// Тип - cписок рабочих потоков
type WorkersList []worker.Worker

// workManager - синглтон, контролирует запуск и остановку рабочих потоков.
type workManager struct {
	Workers         WorkersList
	Shutdown        int32
	ShutdownChannel chan string
}

var (
	wm               workManager // Reference to the singleton
	aliveWorkerChan  chan worker.WorkerID
	workerIDSequence worker.WorkerID = 0
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
	aliveWorkerChan = make(chan worker.WorkerID)

	// Первоначальная инициализация списка рабочих потоков
	log.Debugf("len(cfg.Services) = %d", len(cfg.WebServices))
	workManager.InitWorkers(cfg)

	// Запуск рабочих потоков
	counter := 0
	for counter < len(cfg.WebServices) {
		go workManager.CheckWebService(workManager.Workers[counter], aliveWorkerChan, cfg.DataStorageURL)
		counter++
	}
	for counter < (len(cfg.WebServices) + len(cfg.DBMSServers)) {
		go workManager.CheckDBMSServer(workManager.Workers[counter], aliveWorkerChan, cfg.DataStorageURL)
		counter++
	}
	for counter < (len(cfg.WebServices) + len(cfg.DBMSServers) + len(cfg.Databases)) {
		go workManager.CheckDatabase(workManager.Workers[counter], aliveWorkerChan, cfg.DataStorageURL)
		counter++
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
				counter := 0
				for counter < len(cfg.WebServices) {
					go workManager.CheckWebService(workManager.Workers[counter], aliveWorkerChan, cfg.DataStorageURL)
					counter++
				}
				for counter < (len(cfg.WebServices) + len(cfg.DBMSServers)) {
					go workManager.CheckDBMSServer(workManager.Workers[counter], aliveWorkerChan, cfg.DataStorageURL)
					counter++
				}
				for counter < (len(cfg.WebServices) + len(cfg.DBMSServers) + len(cfg.Databases)) {
					go workManager.CheckDatabase(workManager.Workers[counter], aliveWorkerChan, cfg.DataStorageURL)
					counter++
				}
			}

		// Контрольный сигнал от рабочего потока.
		case workerID := <-aliveWorkerChan:
			log.Debugf("Контрольный сигнал от рабочего потока: %+v", workerID)
			// Обновить данные о рабочем потоке.
			for i := 0; i < len(workManager.Workers); i++ {
				if workManager.Workers[i].GetID() == workerID {
					// Сохранить время получения контрольного сигнала.
					workManager.Workers[i].SetLastStateTime(time.Now())
				}
			}
		}
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Инициализация рабочих потоков
//----------------------------------------------------------------------------------------------------------------------
func (workManager *workManager) InitWorkers(cfg *helper.Config) {
	workManager.Workers = make(WorkersList, len(cfg.WebServices) + len(cfg.DBMSServers) + len(cfg.Databases))
	counter := 0
	for _, service := range cfg.WebServices {
		workerIDSequence = workerIDSequence + 1
		Req, _ := http.NewRequest("GET", service.Address, nil)
		workManager.Workers[counter] = worker.NewWebServiceWorker(
			workerIDSequence,
			service.Enabled,
			service.CheckInterval * time.Second,
			make(chan worker.Command),
			service.Address,
			service.Login,
			service.Password,
			Req)
		counter = counter + 1
	}
	for _, server := range cfg.DBMSServers {
		workerIDSequence = workerIDSequence + 1
		workManager.Workers[counter] = worker.NewDBMSServerWorker(
			workerIDSequence,
			server.Enabled,
			server.CheckInterval * time.Second,
			make(chan worker.Command),
			server.Address,
			server.User,
			server.Password,
			server.Port)
		counter = counter + 1
	}
	for _, database := range cfg.Databases {
		workerIDSequence = workerIDSequence + 1
		workManager.Workers[counter] = worker.NewDatabaseWorker(
			workerIDSequence,
			database.Enabled,
			database.CheckInterval * time.Second,
			make(chan worker.Command),
			database.Address,
			database.DBId,
			database.DBName,
			database.User,
			database.Password,
			database.Port)
		counter = counter + 1
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Закрытие рабочих потоков
//----------------------------------------------------------------------------------------------------------------------
func (workManager *workManager) CloseWorkers() {
	for i := 0; i < len(workManager.Workers); i++ {
		log.Debugf("workingLoop, закрытие рабочего потока с номером %d", workManager.Workers[i].GetID())
		if workManager.Workers[i].GetState() {
			workManager.Workers[i].GetCommandChan() <- true
			log.Debug("workingLoop, закрытие рабочих потоков, послана команда в поток")
			<-workManager.Workers[i].GetCommandChan()
			log.Debug("workingLoop, закрытие рабочих потоков, получена команда из потока")
			close(workManager.Workers[i].GetCommandChan())
			workManager.Workers[i].SetState(false)
		}
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Проверка работоспособности указанного web-сервиса и отправка результата в data storage
//----------------------------------------------------------------------------------------------------------------------
func (workManager *workManager) CheckWebService(worker worker.Worker, outerChan chan worker.WorkerID, dataStorageURL string) {

	if worker == nil {
		log.Debug("worker == nil")
		return
	}

	log.Debugf("CheckWebService [%d], запущен рабочий поток, интервал %d секунд", worker.GetID(), int(worker.GetInterval().Seconds()))

	wait := worker.GetInterval()

	// Рабочий цикл
	for {
		log.Debugf("CheckWebService [%d], ожидание %.3f секунд", worker.GetID(), wait.Seconds())

		if !worker.GetState() {
			log.Infof("CheckWebService [%d], выход из неактивного рабочего потока!", worker.GetID())
			return
		}

		select {
		case <-worker.GetCommandChan():
			log.Infof("CheckWebService [%d], выключение рабочего потока!", worker.GetID())
			worker.GetCommandChan() <- true
			return

		case <-time.After(wait):
			log.Debugf("CheckWebService [%d], завершение ожидания", worker.GetID())
			if !worker.GetState() {
				log.Info("CheckWebService [%d], выход из неактивного рабочего потока!")
				return
			}
			break
		}

		// Контроль необходимости закрытия
		if workManager.Shutdown == 1 {
			log.Debugf("CheckWebService [%d], workManager.Shutdown == 1", worker.GetID())
			return
		}

		// Mark the starting time
		startTime := time.Now()

		// Рабочая проверка
		checkResult := worker.Check()

		// Отправить результат проверки сборщику данных
		putResult(worker.GetID(), dataStorageURL + "/imd", checkResult)

		// Отправить контрольный сигнал
		outerChan <- worker.GetID()

		// Mark the ending time
		endTime := time.Now()

		// Calculate the amount of time to wait to start workManager again.
		duration := endTime.Sub(startTime)
		log.Debugf("CheckWebService [%d], Длительность выполнения рабочей проверки: %.3f секунд", worker.GetID(), duration.Seconds())
		wait = worker.GetInterval() - duration
		log.Debugf("CheckWebService [%d], Следующее ожидание: %.3f секунд", worker.GetID(), wait.Seconds())
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Мониторинг параметров сервера СУБД
//----------------------------------------------------------------------------------------------------------------------
func (workManager *workManager) CheckDBMSServer(worker worker.Worker, outerChan chan worker.WorkerID, dataStorageURL string) {

	if worker == nil {
		log.Debug("worker == nil")
		return
	}

	log.Debugf("CheckDBMSServer [%d], запущен рабочий поток, интервал %d секунд", worker.GetID(), int(worker.GetInterval().Seconds()))

	wait := worker.GetInterval()

	// Рабочий цикл
	for {
		log.Debugf("CheckDBMSServer [%d], ожидание %.3f секунд", worker.GetID(), wait.Seconds())

		if !worker.GetState() {
			log.Infof("CheckDBMSServer [%d], выход из неактивного рабочего потока!", worker.GetID())
			return
		}

		select {
		case <-worker.GetCommandChan():
			log.Infof("CheckDBMSServer [%d], выключение рабочего потока!", worker.GetID())
			worker.GetCommandChan() <- true
			return

		case <-time.After(wait):
			log.Debugf("CheckDBMSServer [%d], завершение ожидания", worker.GetID())
			if !worker.GetState() {
				log.Info("CheckDBMSServer [%d], выход из неактивного рабочего потока!")
				return
			}
			break
		}

		// Контроль необходимости закрытия
		if workManager.Shutdown == 1 {
			log.Debugf("CheckDBMSServer [%d], workManager.Shutdown == 1", worker.GetID())
			return
		}

		// Mark the starting time
		startTime := time.Now()

		// Рабочая проверка
		checkResult := worker.Check()

		log.Debugf("%+v", checkResult)

		//// Отправить результат проверки сборщику данных
		//response, err := makeRequest("POST", dataStorageURL + "/dbms", checkResult)
		//if err != nil {
		//	log.Errorf("CheckDBMSServer [%d], Ошибка отправки данных в data storage: %v", worker.GetID(), err)
		//} else {
		//	defer response.Body.Close()
		//}
		//log.Debugf("CheckDBMSServer [%d], Результат отправки данных в data storage: %+v", worker.GetID(), response)

		// Отправить контрольный сигнал
		outerChan <- worker.GetID()

		// Mark the ending time
		endTime := time.Now()

		// Calculate the amount of time to wait to start workManager again.
		duration := endTime.Sub(startTime)
		log.Debugf("CheckDBMSServer [%d], Длительность выполнения рабочей проверки: %.3f секунд", worker.GetID(), duration.Seconds())
		wait = worker.GetInterval() - duration
		log.Debugf("CheckDBMSServer [%d], Следующее ожидание: %.3f секунд", worker.GetID(), wait.Seconds())
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Мониторинг параметров базы данных
//----------------------------------------------------------------------------------------------------------------------
func (workManager *workManager) CheckDatabase(w interface{}, outerChan chan worker.WorkerID, dataStorageURL string) {

	if w == nil {
		log.Debug("worker == nil")
		return
	}

	var databaseWorker *worker.DatabaseWorker
	databaseWorker = w.(*worker.DatabaseWorker)

	log.Debugf("CheckDatabase [%d], запущен рабочий поток, интервал %d секунд", databaseWorker.GetID(), int(databaseWorker.GetInterval().Seconds()))

	wait := databaseWorker.GetInterval()

	// Рабочий цикл
	for {
		log.Debugf("CheckDatabase [%d], ожидание %.3f секунд", databaseWorker.GetID(), wait.Seconds())

		if !databaseWorker.GetState() {
			log.Infof("CheckDatabase [%d], выход из неактивного рабочего потока!", databaseWorker.GetID())
			return
		}

		select {
		case <-databaseWorker.GetCommandChan():
			log.Infof("CheckDatabase [%d], выключение рабочего потока!", databaseWorker.GetID())
			databaseWorker.GetCommandChan() <- true
			return

		case <-time.After(wait):
			log.Debugf("CheckDatabase [%d], завершение ожидания", databaseWorker.GetID())
			if !databaseWorker.GetState() {
				log.Info("CheckDatabase [%d], выход из неактивного рабочего потока!")
				return
			}
			break
		}

		// Контроль необходимости закрытия
		if workManager.Shutdown == 1 {
			log.Debugf("CheckDatabase [%d], workManager.Shutdown == 1", databaseWorker.GetID())
			return
		}

		// Mark the starting time
		startTime := time.Now()

		// Рабочая проверка
		//checkResult := worker.Check()
		checkResult := DatabaseCheck(databaseWorker)

		log.Debugf("%+v", checkResult)

		// Отправить результат проверки сборщику данных
		//var result DatabaseCheckResult
		//result.DatabaseId = worker.GetDBId()
		//result.FileName = checkResult.Status
		putDatabaseCheckResult(databaseWorker.GetID(), dataStorageURL + "/backups/last", checkResult)

		// Отправить контрольный сигнал
		outerChan <- databaseWorker.GetID()

		// Mark the ending time
		endTime := time.Now()

		// Calculate the amount of time to wait to start workManager again.
		duration := endTime.Sub(startTime)
		log.Debugf("CheckDatabase [%d], Длительность выполнения рабочей проверки: %.3f секунд", databaseWorker.GetID(), duration.Seconds())
		wait = databaseWorker.GetInterval() - duration
		log.Debugf("CheckDatabase [%d], Следующее ожидание: %.3f секунд", databaseWorker.GetID(), wait.Seconds())
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Отправка результата проверки сборщику данных
//----------------------------------------------------------------------------------------------------------------------
func putResult(workerID worker.WorkerID, dataStorageURL string, result *worker.CheckResult) {
	response, err := makeRequest("POST", dataStorageURL, result)
	if err != nil {
		log.Errorf("CheckWebService [%d], Ошибка отправки данных в data storage: %v", workerID, err)
	} else {
		defer response.Body.Close()
	}
	log.Debugf("CheckWebService [%d], Результат отправки данных в data storage: %+v", workerID, response)
}

//----------------------------------------------------------------------------------------------------------------------
// Получение информации о последнем архиве базы данных
// возвращает true — если сервис доступен, false, если нет и текст сообщения
//----------------------------------------------------------------------------------------------------------------------
func DatabaseCheck(worker *worker.DatabaseWorker) *DatabaseCheckResult {
	// Подготовка результата работы функции проверки
	var checkResult *DatabaseCheckResult = new(DatabaseCheckResult)

	// Подключение к серверу
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;encrypt=disable", worker.Address, worker.User, worker.Password, worker.Port)
	conn, err := sql.Open("mssql", connString)
	if err != nil {
		log.Fatal("Open connection failed:", err.Error())
	}
	defer conn.Close()

	// Текст запроса
	query := `
		-------------------------------------------------------------------------------------------
		--Most Recent Database Backup for Each Database - Detailed
		-------------------------------------------------------------------------------------------
		SELECT
		   A.last_db_backup_date,
		   B.type,
		   B.backup_size,
		   B.physical_device_name
		FROM
		   (
		   SELECT
		       msdb.dbo.backupset.database_name,
		       MAX(msdb.dbo.backupset.backup_finish_date) AS last_db_backup_date
		   FROM    msdb.dbo.backupmediafamily
		       INNER JOIN msdb.dbo.backupset ON msdb.dbo.backupmediafamily.media_set_id = msdb.dbo.backupset.media_set_id
		   WHERE
		       msdb.dbo.backupset.database_name = ?1
		   GROUP BY
		       msdb.dbo.backupset.database_name
		   ) AS A

		   LEFT JOIN

		   (
		   SELECT
		   msdb.dbo.backupset.database_name,
		   msdb.dbo.backupset.backup_start_date,
		   msdb.dbo.backupset.backup_finish_date,
		   msdb.dbo.backupset.expiration_date,
		   msdb.dbo.backupset.backup_size,
		   msdb.dbo.backupmediafamily.logical_device_name,
		   msdb.dbo.backupmediafamily.physical_device_name,
		   msdb.dbo.backupset.name AS backupset_name,
		   msdb.dbo.backupset.type,
		   msdb.dbo.backupset.description
		FROM   msdb.dbo.backupmediafamily
		   INNER JOIN msdb.dbo.backupset ON msdb.dbo.backupmediafamily.media_set_id = msdb.dbo.backupset.media_set_id
		   ) AS B
		   ON A.[database_name] = B.[database_name] AND A.[last_db_backup_date] = B.[backup_finish_date]
		ORDER BY
		   A.database_name
   `
	// Запрос данных
	row := conn.QueryRow(query, worker.DBName)
	var last_db_backup_date string
	var backup_type string
	var backup_size int
	var physical_device_name string
	err = row.Scan(&last_db_backup_date, &backup_type, &backup_size, &physical_device_name)
	if err != nil {
		log.Fatal("Scan failed:", err.Error())
	}
	//log.Debug("server:%s\n", server)
	log.Debugf("last_db_backup_date:%v\n", last_db_backup_date)
	log.Debugf("backup_type:%v\n", backup_type)
	log.Debugf("backup_size:%v\n", backup_size)
	log.Debugf("physical_device_name:%v\n", physical_device_name)

	// Заполнение результата проверки подключения
	checkResult.DatabaseId = worker.DBId
	checkResult.FileName = physical_device_name
	checkResult.BackupDate = last_db_backup_date
	checkResult.BackupType = backup_type
	checkResult.BackupSize = backup_size

	return checkResult
}

//----------------------------------------------------------------------------------------------------------------------
// Отправка результата проверки сборщику данных
//----------------------------------------------------------------------------------------------------------------------
func putDatabaseCheckResult(workerID worker.WorkerID, dataStorageURL string, result *DatabaseCheckResult) {
	response, err := makeRequest("PUT", dataStorageURL, result)
	if err != nil {
		log.Errorf("CheckDatabase [%d], Ошибка отправки данных в data storage: %v", workerID, err)
	} else {
		defer response.Body.Close()
	}
	log.Debugf("CheckDatabase [%d], Результат отправки данных в data storage: %+v", workerID, response)
}
