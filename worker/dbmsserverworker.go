package worker

import (
	"time"
	"uais_bu_collector/log"
	"fmt"
	"database/sql"
	_ "github.com/denisenkom/go-mssqldb"
)

type DBMSServerWorker struct {
	BaseWorker
	Address     string
	User        string
	Password    string
	Port        int
}

func NewDBMSServerWorker(id WorkerID, state bool, address string, user string, password string, port int, interval time.Duration, channel chan Command) *DBMSServerWorker {
	return &DBMSServerWorker{
		BaseWorker: BaseWorker{
			ID: id,
			LastStateTime: time.Now(),
			State: state,
			Interval: interval,
			CommandChan: channel,
		},
		Address: address,
		User: user,
		Password: password,
		Port: port,
	}
}

func (worker *DBMSServerWorker) GetID() WorkerID {
	return worker.ID
}

func (worker *DBMSServerWorker) GetLastStateTime() time.Time {
	return worker.LastStateTime
}

func (worker *DBMSServerWorker) GetState() bool {
	return worker.State
}

func (worker *DBMSServerWorker) SetState(state bool) {
	worker.State = state
}

func (worker *DBMSServerWorker) SetLastStateTime(time time.Time) {
	worker.LastStateTime = time
}

func (worker *DBMSServerWorker) GetCommandChan() chan Command {
	return worker.CommandChan
}

func (worker *DBMSServerWorker) GetInterval() time.Duration {
	return worker.Interval
}

func (worker *DBMSServerWorker) Check() *CheckResult {
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

	// Подготовка запроса
	stmt, err := conn.Prepare("DBCC TRACESTATUS (1224);")
	if err != nil {
		log.Fatal("Prepare failed:", err.Error())
	}
	defer stmt.Close()

	// Запрос данных
	row := stmt.QueryRow()
	var traceFlag string
	var status byte
	var global byte
	var session byte
	err = row.Scan(&traceFlag, &status, &global, &session)
	if err != nil {
		log.Fatal("Scan failed:", err.Error())
	}
	//log.Debug("server:%s\n", server)
	log.Debugf("traceFlag:%s\n", traceFlag)
	log.Debugf("status:%d\n", status)
	log.Debugf("global:%d\n", global)
	log.Debugf("session:%d\n", session)

	// Контроль длительности запроса
	checkDuration := time.Since(checkTime)

	// Заполнение результата проверки подключения
	checkResult.CheckTime = checkTime.Format(time.RFC3339)
	checkResult.CheckDuration = checkDuration
	checkResult.Address = worker.Address
	if status == 1 {
		checkResult.Status = "ok"
	} else {
		checkResult.Status = "fail"
	}

	return checkResult
}