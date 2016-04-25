package worker

import (
	"time"
)

type DatabaseWorker struct {
	BaseWorker
}

func NewDatabaseWorker(id WorkerID, state bool, interval time.Duration, channel chan Command) *DatabaseWorker {
	worker := DatabaseWorker {
		BaseWorker: BaseWorker{
			ID: id,
			LastStateTime: time.Now(),
			State: state,
			Interval: interval,
			CommandChan: channel,
		},
	}
	return &worker
}

func (worker *DatabaseWorker) GetID() WorkerID {
	return worker.ID
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

	return checkResult
}


