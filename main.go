package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"github.com/nlopes/slack"
	"github.com/pkg/errors"
)

var debugFlag *bool

var sessionStore = SessionStore{}

func main() {
	err := sessionStore.Init()
	if err != nil {
		log.Fatal(err)
	}

	debugFlag = flag.Bool("debug", false, "Print debug statements")
	flag.Parse()

	http.HandleFunc("/health", HandleHealth)
	http.HandleFunc("/oauth", HandleOAuth)
	http.HandleFunc("/link-slack", HandleLinkSlack)
	http.HandleFunc("/list-triggers", HandleListTriggers)
	http.HandleFunc("/trigger", HandleTrigger)
	http.HandleFunc("/create-trigger", HandleCreateTrigger)
	http.HandleFunc("/delete-trigger", HandleDeleteTrigger)
	http.HandleFunc("/clear-status", HandleClearStatus)

	log.Fatal(http.ListenAndServe(":80", nil))
}

func HandleHealth(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(200)
}

func HandleLinkSlack(writer http.ResponseWriter, request *http.Request) {
	payload := getSlackPayload(request)
	msg, err := LinkSlack(payload)
	if err != nil {
		ReturnError(writer, err)
		return
	}

	ReturnResponse(writer, msg)
}

func HandleOAuth(writer http.ResponseWriter, request *http.Request) {
	session, err := sessionStore.GetCurrentSession(request, writer)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}

	userId := request.FormValue("state")
	if userId == "" {
		userId = session.GetUserId()
	}
	or := OAuthRequest{
		AuthGrant: request.FormValue("code"),
		UserId:    userId,
	}

	userId, err = RefreshOAuthToken(or)
	if err != nil {
		// TODO: serve an error page
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	session.SetUserId(userId)
	err = session.Save()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}

	http.Redirect(writer, request, "https://slackoverload.com/quickstart", 302)
}

func HandleListTriggers(writer http.ResponseWriter, request *http.Request) {
	tr := ListTriggersRequest{
		SlackPayload: getSlackPayload(request),
	}

	response, err := ListTriggers(tr)
	if err != nil {
		ReturnError(writer, err)
		return
	}

	ReturnResponse(writer, response)
}

func HandleTrigger(writer http.ResponseWriter, request *http.Request) {
	tr := TriggerRequest{
		SlackPayload: getSlackPayload(request),
		Name:         request.FormValue("text"),
	}

	msg, err := Trigger(tr)
	if err != nil {
		ReturnError(writer, err)
		return
	}

	ReturnResponse(writer, msg)
}

func HandleCreateTrigger(writer http.ResponseWriter, request *http.Request) {
	cr := CreateTriggerRequest{
		SlackPayload: getSlackPayload(request),
		Definition:   request.FormValue("text"),
	}

	response, err := CreateTrigger(cr)
	if err != nil {
		ReturnError(writer, err)
		return
	}

	ReturnResponse(writer, response)
}

func HandleDeleteTrigger(writer http.ResponseWriter, request *http.Request) {
	cr := DeleteTriggerRequest{
		SlackPayload: getSlackPayload(request),
		Name:         request.FormValue("text"),
	}

	response, err := DeleteTrigger(cr)
	if err != nil {
		ReturnError(writer, err)
		return
	}

	ReturnResponse(writer, response)
}

func HandleClearStatus(writer http.ResponseWriter, request *http.Request) {
	tr := ClearStatusRequest{
		SlackPayload: getSlackPayload(request),
	}

	msg, err := ClearStatus(tr)
	if err != nil {
		ReturnError(writer, err)
		return
	}

	ReturnResponse(writer, msg)
}

func ReturnResponse(writer http.ResponseWriter, msg slack.Msg) {
	if *debugFlag {
		log.Printf("%s\n", msg.Text)
	}
	writer.Header().Set("Content-type", "application/json")
	writer.WriteHeader(200)
	b, err := json.Marshal(msg)
	if err != nil {
		err = errors.Wrapf(err, "error marshaling message to return to Slack, %#v", msg)
		log.Printf("%v\n", err)
		writer.Write([]byte("an error occurred"))
		return
	}

	writer.Write(b)
}

func ReturnError(writer http.ResponseWriter, err error) {
	log.Printf("%v\n", err)
	writer.Header().Set("Content-type", "application/json")
	writer.WriteHeader(200)
	slackErr, err := buildSlackError(err)
	if err != nil {
		err = errors.Wrap(err, "could not handle and return an error to Slack")
		log.Printf("%v\n", err)
		writer.Write([]byte("an error occurred"))
		return
	}
	writer.Write(slackErr)
}

func buildSlackError(err error) ([]byte, error) {
	response := slack.Msg{
		Text:         err.Error(),
		ResponseType: slack.ResponseTypeEphemeral,
	}

	return json.Marshal(response)
}

func getSlackPayload(request *http.Request) SlackPayload {
	return SlackPayload{
		SlackId:  request.FormValue("user_id"),
		UserName: request.FormValue("user_name"),
		TeamId:   request.FormValue("team_id"),
		TeamName: request.FormValue("team_domain"),
	}
}
