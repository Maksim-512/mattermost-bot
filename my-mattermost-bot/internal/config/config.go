package config

import (
	"net/url"
	"os"
)

type Config struct {
	MattermostUserName string
	MattermostTeamName string
	MattermostToken    string
	MattermostChannel  string
	MattermostServer   *url.URL
	TarantoolAddress   string // Адрес подключения к Tarantool
}

func LoadConfig() *Config {
	var settings Config

	settings.MattermostTeamName = os.Getenv("MY_TEAM")
	if settings.MattermostTeamName == "" {
		panic("MY_TEAM environment variable not set")
	}

	settings.MattermostUserName = os.Getenv("MY_USERNAME")
	if settings.MattermostUserName == "" {
		panic("MY_USERNAME environment variable not set")
	}

	settings.MattermostToken = os.Getenv("MY_TOKEN")
	if settings.MattermostToken == "" {
		panic("MY_TOKEN environment variable not set")
	}

	settings.MattermostChannel = os.Getenv("MY_CHANNEL")
	if settings.MattermostChannel == "" {
		panic("MY_CHANNEL environment variable not set")
	}

	settings.MattermostServer, _ = url.Parse(os.Getenv("MY_SERVER"))
	if settings.MattermostServer == nil {
		panic("MY_SERVER environment variable not set")
	}

	settings.TarantoolAddress = os.Getenv("TARANTOOL_ADDRESS")
	if settings.TarantoolAddress == "" {
		panic("TARANTOOL_ADDRESS environment variable not set")
	}

	return &settings
}
