package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string `json:"databaseURL"`
	UploadsPath string `json:"uploadsPath"`
	Port        string `json:"port"`
	Production  bool   `json:"production"`
}

func LoadConfig(file string) (*Config, error) {
	var config Config
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)
	return &config, nil
}
