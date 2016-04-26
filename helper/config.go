package helper

import (
	"errors"
	"io/ioutil"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

const ConfigFileName = "uais_bu_collector.yaml"

var (
	configModTime  int64
	ErrNotModified = errors.New("Not modified")
)

// Структура настроек для web-сервиса, который будет мониториться
type WebService struct {
	Address       string        `yaml:"address"`
	Login         string        `yaml:"login"`
	Password      string        `yaml:"password"`
	Enabled       bool          `yaml:"enabled"`
	CheckInterval time.Duration `yaml:"check_interval"`
}

// Структура настроек для сервера СУБД, который будет мониториться
type DBMSServer struct {
	Address         string          `yaml:"address"`
	User            string          `yaml:"login"`
	Password        string          `yaml:"password"`
	Port            int             `yaml:"port"`
	Enabled         bool            `yaml:"enabled"`
	CheckInterval   time.Duration   `yaml:"check_interval"`
}

// Структура настроек для базы данных, которая будет мониториться
type Database struct {
	Address         string          `yaml:"address"`
	DBId            int             `yaml:"id"`
	DBName          string          `yaml:"name"`
	User            string          `yaml:"login"`
	Password        string          `yaml:"password"`
	Port            int             `yaml:"port"`
	Enabled         bool            `yaml:"enabled"`
	CheckInterval   time.Duration   `yaml:"check_interval"`
}

// Структура для считывания конфигурационного файла
type Config struct {
	ReloadConfigInterval    int             `yaml:"reload_config_interval"`
	LogLevel                string          `yaml:"log_level"`
	LogFilename             string          `yaml:"log_filename"`
	DataStorageURL          string          `yaml:"data_storage_url"`
	WebServices             []WebService    `yaml:"web_services"`
	DBMSServers             []DBMSServer    `yaml:"dbms_servers"`
	Databases               []Database      `yaml:"databases"`
}

//----------------------------------------------------------------------------------------------------------------------
// Загрузка конфигурации из указанного файла
//----------------------------------------------------------------------------------------------------------------------
func ReadConfig(ConfigName string) (x *Config, err error) {
	var file []byte
	if file, err = ioutil.ReadFile(ConfigName); err != nil {
		return nil, err
	}
	x = new(Config)
	if err = yaml.Unmarshal(file, x); err != nil {
		return nil, err
	}
	if x.LogLevel == "" {
		x.LogLevel = "Debug"
	}
	return x, nil
}

//----------------------------------------------------------------------------------------------------------------------
// Проверка времени изменения конфигурационного файла и перезагрузка его, если он изменился
// Возврат errNotModified если изменений нет
//----------------------------------------------------------------------------------------------------------------------
func ReloadConfig(configName string) (cfg *Config, err error) {
	info, err := os.Stat(configName)
	if err != nil {
		return nil, err
	}
	if configModTime != info.ModTime().UnixNano() {
		configModTime = info.ModTime().UnixNano()
		cfg, err = ReadConfig(configName)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	return nil, ErrNotModified
}
