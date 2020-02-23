package slackoverload

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/nlopes/slack"
	"github.com/pkg/errors"
)

type SlackHandler struct {
	SessionStore
	App
}

func (h *SlackHandler) Init() error {
	fmt.Println("Initializing...")

	http.HandleFunc("/health", h.HandleHealth)
	http.HandleFunc("/oauth", h.HandleOAuth)
	http.HandleFunc("/link-slack", h.HandleLinkSlack)
	http.HandleFunc("/list-triggers", h.HandleListTriggers)
	http.HandleFunc("/trigger", h.HandleTrigger)
	http.HandleFunc("/create-trigger", h.HandleCreateTrigger)
	http.HandleFunc("/delete-trigger", h.HandleDeleteTrigger)
	http.HandleFunc("/clear-status", h.HandleClearStatus)

	secrets, err := NewSecretsClient()
	if err != nil {
		return err
	}

	err = h.SessionStore.Init(secrets)
	if err != nil {
		return err
	}

	return h.App.Init(secrets)
}

func (h *SlackHandler) Run() error {
	fmt.Println("Ready!")
	return http.ListenAndServe(":80", nil)
}

func (h *SlackHandler) HandleHealth(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(200)
}

func (h *SlackHandler) HandleLinkSlack(writer http.ResponseWriter, request *http.Request) {
	payload := h.getSlackPayload(request)
	msg, err := h.LinkSlack(payload)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	h.ReturnResponse(writer, msg)
}

func (h *SlackHandler) HandleOAuth(writer http.ResponseWriter, request *http.Request) {
	session, err := h.SessionStore.GetCurrentSession(request, writer)
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

	userId, err = h.RefreshOAuthToken(or)
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

func (h *SlackHandler) HandleListTriggers(writer http.ResponseWriter, request *http.Request) {
	tr := ListTriggersRequest{
		SlackPayload: h.getSlackPayload(request),
	}

	response, err := h.ListTriggers(tr)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	h.ReturnResponse(writer, response)
}

func (h *SlackHandler) HandleTrigger(writer http.ResponseWriter, request *http.Request) {
	tr := TriggerRequest{
		SlackPayload: h.getSlackPayload(request),
		Name:         request.FormValue("text"),
	}

	msg, err := h.Trigger(tr)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	h.ReturnResponse(writer, msg)
}

func (h *SlackHandler) HandleCreateTrigger(writer http.ResponseWriter, request *http.Request) {
	cr := CreateTriggerRequest{
		SlackPayload: h.getSlackPayload(request),
		Definition:   request.FormValue("text"),
	}

	response, err := h.CreateTrigger(cr)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	h.ReturnResponse(writer, response)
}

func (h *SlackHandler) HandleDeleteTrigger(writer http.ResponseWriter, request *http.Request) {
	cr := DeleteTriggerRequest{
		SlackPayload: h.getSlackPayload(request),
		Name:         request.FormValue("text"),
	}

	response, err := h.DeleteTrigger(cr)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	h.ReturnResponse(writer, response)
}

func (h *SlackHandler) HandleClearStatus(writer http.ResponseWriter, request *http.Request) {
	tr := ClearStatusRequest{
		SlackPayload: h.getSlackPayload(request),
	}

	msg, err := h.ClearStatus(tr)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	h.ReturnResponse(writer, msg)
}

func (h *SlackHandler) ReturnResponse(writer http.ResponseWriter, msg slack.Msg) {
	if h.Debug {
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

func (h *SlackHandler) ReturnError(writer http.ResponseWriter, err error) {
	log.Printf("%v\n", err)
	writer.Header().Set("Content-type", "application/json")
	writer.WriteHeader(200)
	slackErr, err := h.buildSlackError(err)
	if err != nil {
		err = errors.Wrap(err, "could not handle and return an error to Slack")
		log.Printf("%v\n", err)
		writer.Write([]byte("an error occurred"))
		return
	}
	writer.Write(slackErr)
}

func (h *SlackHandler) buildSlackError(err error) ([]byte, error) {
	response := slack.Msg{
		Text:         err.Error(),
		ResponseType: slack.ResponseTypeEphemeral,
	}

	return json.Marshal(response)
}

func (h *SlackHandler) getSlackPayload(request *http.Request) SlackPayload {
	return SlackPayload{
		SlackId:  request.FormValue("user_id"),
		UserName: request.FormValue("user_name"),
		TeamId:   request.FormValue("team_id"),
		TeamName: request.FormValue("team_domain"),
	}
}
