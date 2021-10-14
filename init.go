package main

import (
	"encoding/json"
	"errors"
	"os"
)

type AppConfig struct {
	Environment   string `json:"environment"`
	TelegramToken string `json:"telegram_token"`
	ProjectID     string `json:"project_id"`
}

func (appConfig *AppConfig) LoadConfiguration(file string) error {
	if file == "" {
		return errors.New("should be DLMBLTLG environment variable set to the path to app config")
	}
	configFile, err := os.Open(file)
	if err != nil {
		return err
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&appConfig)
	for _, val := range []string{appConfig.Environment, appConfig.TelegramToken, appConfig.ProjectID} {
		if val == "" {
			return errors.New("all field of the configutation file are required")
		}
	}
	return nil
}
