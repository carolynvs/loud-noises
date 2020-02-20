package main

import (
	"flag"
	"log"

	"github.com/carolynvs/slackoverload/slackoverload"
)

func main() {
	h := slackoverload.SlackHandler{}

	debugFlag := flag.Bool("debug", false, "Print debug statements")
	flag.Parse()
	h.Debug = *debugFlag

	err := h.Init()
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(h.Run())
}
