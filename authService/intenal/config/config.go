package config

import (
	"sync"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

const configFileName string = "config.yml"

type Config struct {
	App struct {
		Mode          string        `yaml:"mode"`
		Port          string        `yaml:"port"`
		JWTSecret     string        `yaml:"jwt_secret"`
		JWTAccessTTL  time.Duration `yaml:"jwt_access_ttl"`
		JWTRefreshTTL time.Duration `yaml:"jwt_refresh_ttl"`
	} `yaml:"app"`
	Storage StorageConfig `yaml:"storage"`
}

type StorageConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Username string `yaml:"username"`
	Dbname   string `yaml:"dbname"`
	Password string `yaml:"password"`
	Sslmode  string `yaml:"sslmode"`
}

var instance *Config
var once sync.Once

func GetConfig() *Config {
	once.Do(func() {
		instance = &Config{}
		if err := cleanenv.ReadConfig(configFileName, instance); err != nil {
			panic("config loading failed: " + err.Error())
		}
	})
	return instance
}
