package main

import (
	"flag"
	"os"

	"github.com/nlopes/slack"
)

func main() {
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		panic("SLACK_TOKEN not set")
	}

	api := slack.New(token)

	debugFlag := flag.Bool("debug", false, "Print debug statements")
	flag.Parse()
	if *debugFlag == true {
		api.SetDebug(true)
	}

	err := api.SetUserPresence("away")
	if err != nil {
		panic(err)
	}

	err = api.SetUserCustomStatus("brb", ":dash:")
	if err != nil {
		panic(err)
	}

	const day = 86400
	_, err = api.SetSnooze(day)
	if err != nil {
		panic(err)
	}
}
