package config

import (
	"sync"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

const configFileName string = "config.yml"

type Config struct {
	App struct {
		Mode      string `yaml:"mode"`
		Port      string `yaml:"port"`
		JWTSecret string `yaml:"jwt_secret"`
	} `yaml:"app"`

	Storage StorageConfig `yaml:"storage"`
	Minio   MinioConfig   `yaml:"minio"`
	Redis   RedisConfig   `yaml:"redis"`
	Elastic ElasticConfig `yaml:"elastic"`
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
	Addr            string        `yaml:"addr"`
	Bucket          string        `yaml:"bucket"`
	User            string        `yaml:"root_user"`
	Password        string        `yaml:"root_password"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	IdleConnTimeout time.Duration `yaml:"idle_conn_timeout"`
	MaxRetries      int           `yaml:"max_retries"`
	Ssl             bool          `yaml:"ssl"`
}

type RedisConfig struct {
	Mode         string        `yaml:"mode"`
	Addr         string        `yaml:"addr"`
	Addrs        []string      `yaml:"addrs"`
	Password     string        `yaml:"password"`
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yanl:"write_timeout"`
	PoolSize     int           `yaml:"pool_size"`
	MaxRetries   int           `yaml:"max_retries"`
}

type ElasticConfig struct {
	Mode               string   `yaml:"mode"`
	Addr               string   `yaml:"addr"`
	Addrs              []string `yaml:"addrs"`
	Username           string   `yaml:"username"`
	Password           string   `yaml:"password"`
	RetryOnStatus      []int    `yaml:"retry_on_status"`
	MaxRetries         int      `yaml:"max_retries"`
	CompressRequstBody bool     `yaml:"compress_request_body"`
	Index              string   `yaml:"index"`
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
