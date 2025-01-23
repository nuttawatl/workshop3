package config

import (
	"fmt"
	"log"
	"sync"

	"github.com/caarlos0/env/v10"
)

// Config is a struct that contains all configuration for the application
// NOTE: struct name should be in lowercase and field name should be in uppercase
// you can group the configuration by adding new struct
// Example:
//
//	type Config struct {
//			...
//			GCP gcp  // no need to add tag `env` for struct here.
//	}
//
// then create gcp struct with tag `env` for each field
//
//	type gcp struct {
//		ProjectID string `env:"GCP_PROJECT_ID"`
//	}
//
// you can add field without grouping them by adding new field with tag `env`
// Example:
//
//	type Config struct {
//		...
//		AppName string `env:"APP_NAME"`
//	}
type Config struct {
	ProjectID  string `env:"FIREBASE_PROJECT_ID"`
	Email      string `env:"FIREBASE_CLIENT_EMAIL"`
	PrivateKey string `env:"FIREBASE_PRIVATE_KEY"`
	PORT       string `env:"SERVER_PORT"`
}

var (
	once sync.Once
	conf Config
)

func prefix(e string) string {
	if e == "" {
		return ""
	}

	return fmt.Sprintf("%s_", e)
}

func C() Config {
	once.Do(func() {
		var err error
		conf, err = parseEnv[Config](env.Options{})
		if err != nil {
			log.Fatal(err)
		}
	})

	return conf
}

func parseEnv[T any](opts env.Options) (T, error) {
	var t T

	if err := env.Parse(&t); err != nil {
		return t, err
	}

	return t, nil
}
