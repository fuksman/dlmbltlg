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
	CheckDelay    int    `json:"check_delay"`
}

func (appConfig *AppConfig) LoadConfiguration() error {
	file := os.Getenv("DLMBLTLG")
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
	if appConfig.CheckDelay <= 0 {
		return errors.New("all fields of the configutation file are required")
	}
	for _, val := range []string{appConfig.Environment, appConfig.TelegramToken, appConfig.ProjectID} {
		if val == "" {
			return errors.New("all fields of the configutation file are required")
		}
	}
	return nil
}
