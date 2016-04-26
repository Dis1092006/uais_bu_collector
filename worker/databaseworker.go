package worker

import (
	"time"
	"fmt"
	"database/sql"
	"uais_bu_collector/log"
	_ "github.com/denisenkom/go-mssqldb"
)

type DatabaseWorker struct {
	BaseWorker
	Address     string
	Name        string
	User        string
	Password    string
	Port        int
}

func NewDatabaseWorker(id WorkerID, state bool, interval time.Duration, channel chan Command, address string, name string, user string, password string, port int) *DatabaseWorker {
	worker := DatabaseWorker {
		BaseWorker: BaseWorker{
			ID: id,
			LastStateTime: time.Now(),
			State: state,
			Interval: interval,
			CommandChan: channel,
		},
		Address: address,
		Name: name,
		User: user,
		Password: password,
		Port: port,
	}
	return &worker
}

func (worker *DatabaseWorker) GetID() WorkerID {
	return worker.ID
}

func (worker *DatabaseWorker) GetName() string {
	return worker.Name
}

func (worker *DatabaseWorker) GetLastStateTime() time.Time {
	return worker.LastStateTime
}

func (worker *DatabaseWorker) SetLastStateTime(time time.Time) {
	worker.LastStateTime = time
}

func (worker *DatabaseWorker) GetState() bool {
	return worker.State
}

func (worker *DatabaseWorker) SetState(state bool) {
	worker.State = state
}

func (worker *DatabaseWorker) GetCommandChan() chan Command {
	return worker.CommandChan
}

func (worker *DatabaseWorker) GetInterval() time.Duration {
	return worker.Interval
}

//----------------------------------------------------------------------------------------------------------------------
// Проверка подключения к web-сервису
// возвращает true — если сервис доступен, false, если нет и текст сообщения
//----------------------------------------------------------------------------------------------------------------------
func (worker *DatabaseWorker) Check() *CheckResult {
	// Подготовка результата работы функции проверки
	var checkResult *CheckResult = new(CheckResult)

	// Засечка времени
	checkTime := time.Now()

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
		       msdb..backupset.type = 'D'
		       AND msdb.dbo.backupset.database_name = ?1
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
		   msdb.dbo.backupset.description
		FROM   msdb.dbo.backupmediafamily
		   INNER JOIN msdb.dbo.backupset ON msdb.dbo.backupmediafamily.media_set_id = msdb.dbo.backupset.media_set_id
		WHERE  msdb..backupset.type = 'D'
		   ) AS B
		   ON A.[database_name] = B.[database_name] AND A.[last_db_backup_date] = B.[backup_finish_date]
		ORDER BY
		   A.database_name
   `
	// Запрос данных
	row := conn.QueryRow(query, worker.Name)
	var last_db_backup_date string
	var backup_size int
	var physical_device_name string
	err = row.Scan(&last_db_backup_date, &backup_size, &physical_device_name)
	if err != nil {
		log.Fatal("Scan failed:", err.Error())
	}
	//log.Debug("server:%s\n", server)
	log.Debugf("last_db_backup_date:%v\n", last_db_backup_date)
	log.Debugf("backup_size:%v\n", backup_size)
	log.Debugf("physical_device_name:%v\n", physical_device_name)

	// Контроль длительности запроса
	checkDuration := time.Since(checkTime)

	// Заполнение результата проверки подключения
	checkResult.CheckTime = checkTime.Format(time.RFC3339)
	checkResult.CheckDuration = checkDuration
	checkResult.Address = worker.Address
	checkResult.Status = physical_device_name

	return checkResult
}


