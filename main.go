package main

import (
	"flag"
	"os"

	"github.com/nlopes/slack"
)

type Presence string

const (
	PresenceAway   = "away"
	PresenceActive = "auto"
)

type Action struct {
	Presence    Presence
	StatusText  string
	StatusEmoji string
	DndMinutes  *int
}

func main() {
	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		panic("SLACK_TOKEN not set")
	}

	action := Action{
		Presence:    PresenceAway,
		StatusText:  "Pony Farts!",
		StatusEmoji: "🦄",
		DndMinutes:  minutes(60),
	}

	api := slack.New(token)

	debugFlag := flag.Bool("debug", false, "Print debug statements")
	flag.Parse()
	if *debugFlag == true {
		api.SetDebug(true)
	}

	err := api.SetUserPresence(string(action.Presence))
	if err != nil {
		panic(err)
	}

	err = api.SetUserCustomStatus(action.StatusText, action.StatusEmoji)
	if err != nil {
		panic(err)
	}

	if action.DndMinutes == nil {
		_, err = api.EndSnooze()
		if err != nil {
			panic(err)
		}
	} else {
		_, err = api.SetSnooze(*action.DndMinutes)
		if err != nil {
			panic(err)
		}
	}
}

func minutes(value int) *int {
	return &value
}
