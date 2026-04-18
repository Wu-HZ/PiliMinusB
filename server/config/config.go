package config

import (
	"encoding/json"
	"os"
	"sync"
)

type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	JWT      JWTConfig      `json:"jwt"`
	Sauc     SaucConfig     `json:"sauc"`
}

type ServerConfig struct {
	Port string `json:"port"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

type JWTConfig struct {
	Secret string `json:"secret"`
}

type SaucConfig struct {
	AppKey                 string `json:"app_key"`
	AccessKey              string `json:"access_key"`
	WSURL                  string `json:"ws_url"`
	RealtimeWSURL          string `json:"realtime_ws_url"`
	SegmentDuration        int    `json:"seg_duration"`
	TimeoutSec             int    `json:"timeout_sec"`
	RealtimeTimeoutSec     int    `json:"realtime_timeout_sec"`
	TranscribeConcurrency  int    `json:"transcribe_concurrency"`
}

// DSN returns the MySQL data source name.
func (d *DatabaseConfig) DSN() string {
	return d.User + ":" + d.Password + "@tcp(" + d.Host + ":" + d.Port + ")/" + d.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
}

var (
	cfg  *Config
	once sync.Once
)

func Get() *Config {
	once.Do(func() {
		cfg = &Config{
			Server:   ServerConfig{Port: "8080"},
			Database: DatabaseConfig{Host: "127.0.0.1", Port: "3306", User: "root", Password: "", DBName: "piliminusb"},
			JWT:      JWTConfig{Secret: "change-me-to-a-random-secret"},
			Sauc: SaucConfig{
				WSURL:                 "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream",
				RealtimeWSURL:         "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel",
				SegmentDuration:       200,
				TimeoutSec:            7200,
				RealtimeTimeoutSec:    1800,
				TranscribeConcurrency: 3,
			},
		}
		data, err := os.ReadFile("config.json")
		if err == nil {
			json.Unmarshal(data, cfg)
		}
	})
	return cfg
}
