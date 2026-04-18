package config

import (
	"errors"
	"os"
	"strings"

	rootcfg "piliminusb/config"
)

// Credentials returns the Volc ASR app_key and access_key.
// Resolution order: env (VOLC_APP_KEY / VOLC_ACCESS_KEY) overrides config.json.
func Credentials() (string, string, error) {
	appKey := strings.TrimSpace(os.Getenv("VOLC_APP_KEY"))
	accessKey := strings.TrimSpace(os.Getenv("VOLC_ACCESS_KEY"))

	if appKey == "" || accessKey == "" {
		sauc := rootcfg.Get().Sauc
		if appKey == "" {
			appKey = strings.TrimSpace(sauc.AppKey)
		}
		if accessKey == "" {
			accessKey = strings.TrimSpace(sauc.AccessKey)
		}
	}

	if appKey == "" || accessKey == "" {
		return "", "", errors.New("missing app_key or access_key: set sauc.app_key/access_key in config.json or VOLC_APP_KEY/VOLC_ACCESS_KEY env")
	}
	return appKey, accessKey, nil
}
