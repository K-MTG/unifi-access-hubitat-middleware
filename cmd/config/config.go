package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  *Server  `yaml:"server"`
	UAC     *UAC     `yaml:"uac"`
	Hubitat *Hubitat `yaml:"hubitat"`
	Doors   []Door   `yaml:"doors"`
}

type Server struct {
	BaseURL   string `yaml:"base_url"`
	AuthToken string `yaml:"auth_token"`
}

type UAC struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

type Hubitat struct {
	BaseURL     string `yaml:"base_url"`
	AccessToken string `yaml:"access_token"`
}

type Door struct {
	UacID            string `yaml:"uac_id"`
	HubitatContactID string `yaml:"hubitat_contact_id"`
	HubitatLockID    string `yaml:"hubitat_lock_id"`
	HubitatSwitchID  string `yaml:"hubitat_switch_id"`
}

func LoadConfig(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
