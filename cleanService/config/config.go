package config

import (
	logging "cleanService/utils/logger"
	"sync"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Listen struct {
		Port string `yaml:"port"`
	} `yaml:"listen"`

	Storage StorageConfig `yaml:"storage"`
	Minio   MinioConfig   `yaml:"minio"`
	Elastic ElasticConfig `yaml:"elastic"`
	Kafka   KafkaConfig   `yaml:"kafka"`
}

type StorageConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Username string `yaml:"username"`
	Dbname   string `yaml:"dbname"`
	Password string `yaml:"password"`
	Sslmode  string `yaml:"sslmode"`
}

type MinioConfig struct {
	Host     string `yaml:"host"`
	Bucket   string `yaml:"bucket"`
	Rootuser string `yaml:"rootuser"`
	Password string `yaml:"password"`
	Ssl      bool   `yaml:"ssl"`
}

type ElasticConfig struct {
	Addresses []string `yaml:"addresses"`
	Index     string   `yaml:"index"`
}

type KafkaConfig struct {
	Address string `yaml:"address"`
	Topic   string `yaml:"topic"`
	Group   string `yaml:"group"`
}

var instance *Config
var once sync.Once

func GetConfig(filename string, logger *logging.Logger) *Config {
	once.Do(func() {
		logger.Info("read application configuration")
		instance = &Config{}
		if err := cleanenv.ReadConfig("config.yml", instance); err != nil {
			help, _ := cleanenv.GetDescription(instance, nil)
			logger.Info(help)
			logger.Fatal(err)
		}
	})
	return instance
}
