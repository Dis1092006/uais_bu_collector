package worker

import (
	"time"
)

// Тип - идентификатор рабочего потока
type WorkerID int

// Тип - команда управления рабочим потоком
type Command bool

// Тип - результат проверки
type CheckResult struct {
	CheckTime     string        `json:"time"`
	CheckDuration time.Duration `json:"duration"`
	Address       string        `json:"address"`
	Status        string        `json:"status"`
	Error         string        `json:"error"`
}

// Базовый тип - рабочий поток
type BaseWorker struct {
	ID            WorkerID
	LastStateTime time.Time
	State         bool
	Interval      time.Duration
	CommandChan   chan Command
}

type Worker interface {
	Check()         *CheckResult
	GetID()            WorkerID
	GetDBId()           int
	GetDBName()         string
	GetLastStateTime() time.Time
	SetLastStateTime(time time.Time)
	GetState()         bool
	SetState(state bool)
	GetCommandChan()   chan Command
	GetInterval()      time.Duration
}

