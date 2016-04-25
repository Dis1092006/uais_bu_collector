package worker

import (
	"time"
	"net/http"
	"uais_bu_collector/log"
	"strconv"
)

type WebServiceWorker struct {
	BaseWorker
	URL         string
	Login       string
	Password    string
	Req         *http.Request
}

func NewWebServiceWorker(id WorkerID, state bool, url string, login string, password string, interval time.Duration, channel chan Command, req *http.Request) *WebServiceWorker {
	worker := WebServiceWorker {
		BaseWorker: BaseWorker{
			ID: id,
			LastStateTime: time.Now(),
			State: state,
			Interval: interval,
			CommandChan: channel,
		},
		URL: url,
		Login: login,
		Password: password,
		Req: req,
	}
	worker.Req.SetBasicAuth(login, password)
	return &worker
}

func (worker *WebServiceWorker) GetID() WorkerID {
	return worker.ID
}

func (worker *WebServiceWorker) GetLastStateTime() time.Time {
	return worker.LastStateTime
}

func (worker *WebServiceWorker) SetLastStateTime(time time.Time) {
	worker.LastStateTime = time
}

func (worker *WebServiceWorker) GetState() bool {
	return worker.State
}

func (worker *WebServiceWorker) SetState(state bool) {
	worker.State = state
}

func (worker *WebServiceWorker) GetCommandChan() chan Command {
	return worker.CommandChan
}

func (worker *WebServiceWorker) GetInterval() time.Duration {
	return worker.Interval
}

//----------------------------------------------------------------------------------------------------------------------
// Проверка подключения к web-сервису
// возвращает true — если сервис доступен, false, если нет и текст сообщения
//----------------------------------------------------------------------------------------------------------------------
func (worker *WebServiceWorker) Check() *CheckResult {
	// Подготовка результата работы функции проверки
	var checkResult *CheckResult = new(CheckResult)

	// Засечка времени
	checkTime := time.Now()

	// Попытка подключения
	//req, _ := http.NewRequest("GET", url, nil)
	//req.SetBasicAuth(login, password)
	client := &http.Client{}
	resp, err := client.Do(worker.Req)

	// Контроль длительности замера
	checkDuration := time.Since(checkTime)

	// Анализ результатов попытки подключения
	log.Infof("Проверка подключения к адресу: %s", worker.URL)
	if err != nil {
		checkResult.Status = ""
		checkResult.Error = err.Error()
		log.Errorf("Ошибка! %v", err)
	} else {
		defer resp.Body.Close()
		checkResult.Status = strconv.Itoa(resp.StatusCode)
		checkResult.Error = ""
		log.Infof("Успешно. Длительность запроса: %.3f секунд", checkDuration.Seconds())
	}

	// Заполнение результата проверки подключения
	checkResult.CheckTime = checkTime.Format(time.RFC3339)
	checkResult.CheckDuration = checkDuration
	checkResult.Address = worker.URL
	log.Debugf("%+v", checkResult)

	return checkResult
}

