package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	azureauth "github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/nlopes/slack"
	"github.com/pkg/errors"
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

/*
token=gIkuvaNzQIHg97ATvDxqgjtO
&team_id=T0001
&team_domain=example
&enterprise_id=E0001
&enterprise_name=Globular%20Construct%20Inc
&channel_id=C2147483705
&channel_name=test
&user_id=U2147483697
&user_name=Steve
&command=/weather
&text=94070
&response_url=https://hooks.slack.com/commands/1234/5678
&trigger_id=13345224609.738474920.8088930838d88f008e0
*/

var debugFlag *bool

func main() {
	debugFlag = flag.Bool("debug", false, "Print debug statements")
	flag.Parse()

	http.HandleFunc("/slack/cmd/trigger", HandleTrigger)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func HandleTrigger(writer http.ResponseWriter, request *http.Request) {
	name := request.FormValue("text")
	err := Trigger(name)
	if err != nil {
		writer.WriteHeader(200)
		writer.Write(buildSlackError(err))
		return
	}

	writer.WriteHeader(200)
}

func Trigger(name string) error {
	fmt.Printf("Triggering %s\n", name)

	token, err := getSlackToken()
	if err != nil {
		fmt.Printf("%v\n", err)
		return err
	}

	actions := map[string]Action{
		"farts": {
			Presence:    PresenceAway,
			StatusText:  "Pony Farts!",
			StatusEmoji: "🦄",
			DndMinutes:  minutes(60),
		},
		"reset": {
			Presence:    PresenceActive,
			StatusText:  "",
			StatusEmoji: "",
			DndMinutes:  nil,
		},
	}

	action, ok := actions[name]
	if !ok {
		err := errors.Errorf("could not trigger %s, action not registered\n", name)
		fmt.Printf("%v\n", err)
		return err
	}

	api := slack.New(token)
	if *debugFlag == true {
		api.SetDebug(true)
	}

	err = api.SetUserPresence(string(action.Presence))
	if err != nil {
		err = errors.Wrap(err, "could not set presence")
		fmt.Printf("%v\n", err)
		return err
	}

	err = api.SetUserCustomStatus(action.StatusText, action.StatusEmoji)
	if err != nil {
		err = errors.Wrap(err, "could not set status")
		fmt.Printf("%v\n", err)
		return err
	}

	if action.DndMinutes == nil {
		_, err = api.EndSnooze()
		if err != nil {
			err = errors.Wrap(err, "could not end do not disturb")
			fmt.Printf("%v\n", err)
			return err
		}
	} else {
		_, err = api.SetSnooze(*action.DndMinutes)
		if err != nil {
			err = errors.Wrap(err, "could not set do not disturb")
			fmt.Printf("%v\n", err)
			return err
		}
	}

	return nil
}

func buildSlackError(err error) []byte {
	response := map[string]string{
		"response_type": "ephemeral",
		"text":          err.Error(),
	}

	b, _ := json.Marshal(response)
	return b
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
