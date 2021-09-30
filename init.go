package main

import (
	"encoding/json"
	"errors"
	"os"
)

type AppConfig struct {
	Environment     string `json:"environment"`
	TelegramToken   string `json:"telegram_token"`
	ProjectID       string `json:"project_id"`
	UsersCollection string `json:"users_collection"`
}

func (appConfig *AppConfig) LoadConfiguration(file string) error {
	configFile, err := os.Open(file)
	if err != nil {
		return err
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&appConfig)
	for _, val := range []string{appConfig.Environment, appConfig.TelegramToken, appConfig.ProjectID, appConfig.UsersCollection} {
		if val == "" {
			return errors.New("all field of the configutation file are required")
		}
	}
	return nil
}
