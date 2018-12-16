package main

import (
	"os"

	"github.com/nlopes/slack"
)

func main() {
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		panic("SLACK_TOKEN not set")
	}
	
	api := slack.New(token)
	api.SetDebug(true)

	err := api.SetUserPresence("away")
	if err != nil {
		panic(err)
	}

	err = api.SetUserCustomStatus("brb", ":dash:")
	if err != nil {
		panic(err)
	}
}
