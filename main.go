package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	azureauth "github.com/Azure/go-autorest/autorest/azure/auth"
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

var debugFlag *bool

func main() {
	token, err := getSlackToken()
	if err != nil {
		panic(err)
	}

	action := Action{
		Presence:    PresenceAway,
		StatusText:  "Pony Farts!",
		StatusEmoji: "🦄",
		DndMinutes:  minutes(60),
	}

	api := slack.New(token)

	debugFlag = flag.Bool("debug", false, "Print debug statements")
	flag.Parse()
	if *debugFlag == true {
		api.SetDebug(true)
	}

	err = api.SetUserPresence(string(action.Presence))
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

func getSlackToken() (string, error) {
	fmt.Println("Loading azure auth from magic...")
	if clientId, ok := os.LookupEnv("MSI_USER_ASSIGNED_CLIENTID"); ok {
		fmt.Println("Found MSI_USER_ASSIGNED_CLIENTID")
		os.Setenv(azureauth.ClientID, clientId)
	}

	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		fmt.Println("Loading slack token from env var...")
		token := os.Getenv("SLACK_TOKEN")
		if token == "" {
			return "", fmt.Errorf("could not authenticate using ambient environment: %s", err.Error())
		}
		return token, nil
	}

	client := keyvault.New()
	client.Authorizer = authorizer

	fmt.Println("Loading slack token from vault...")

	vaultURL := "https://slackoverload.vault.azure.net/"
	grr, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	result, err := client.GetSecret(grr, vaultURL, "slack-token", "")
	if err != nil {
		defer cancel()
		return "", fmt.Errorf("could not load slack token from vault: %s", err)
	}

	return *result.Value, nil
}
