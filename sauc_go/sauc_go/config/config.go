package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Auth struct {
		AppKey    string `toml:"app_key"`
		AccessKey string `toml:"access_key"`
	}
}

var (
	loadOnce sync.Once
	loadErr  error
	config   Config
)

func Credentials() (string, string, error) {
	if err := load(); err != nil {
		return "", "", err
	}
	if config.Auth.AppKey == "" || config.Auth.AccessKey == "" {
		return "", "", errors.New("missing app_key or access_key")
	}
	return config.Auth.AppKey, config.Auth.AccessKey, nil
}

func load() error {
	loadOnce.Do(func() {
		config.Auth.AppKey = strings.TrimSpace(os.Getenv("VOLC_APP_KEY"))
		config.Auth.AccessKey = strings.TrimSpace(os.Getenv("VOLC_ACCESS_KEY"))
		if config.Auth.AppKey != "" && config.Auth.AccessKey != "" {
			return
		}

		for _, path := range candidatePaths() {
			if _, err := os.Stat(path); err != nil {
				continue
			}
			if _, err := toml.DecodeFile(path, &config); err != nil {
				loadErr = err
				return
			}
			return
		}

		loadErr = errors.New("config.toml not found; set VOLC_APP_KEY and VOLC_ACCESS_KEY or create config.toml")
	})
	return loadErr
}

func candidatePaths() []string {
	wd, err := os.Getwd()
	if err != nil {
		return []string{"config.toml", filepath.Join("..", "config.toml")}
	}
	return []string{
		filepath.Join(wd, "config.toml"),
		filepath.Join(wd, "..", "config.toml"),
	}
}
