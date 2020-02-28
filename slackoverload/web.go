package slackoverload

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/nlopes/slack"
	"github.com/pkg/errors"
)

type SlackHandler struct {
	SessionStore
	App

	signingSecret string
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

	h.signingSecret, err = secrets.GetSlackSigningSecret()
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
	payload, err := h.getSlackPayload(writer, request)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

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
	payload, err := h.getSlackPayload(writer, request)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	r := ListTriggersRequest{SlackPayload: payload}
	response, err := h.ListTriggers(r)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	h.ReturnResponse(writer, response)
}

func (h *SlackHandler) HandleTrigger(writer http.ResponseWriter, request *http.Request) {
	payload, err := h.getSlackPayload(writer, request)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	r := TriggerRequest{SlackPayload: payload}
	msg, err := h.Trigger(r)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	h.ReturnResponse(writer, msg)
}

func (h *SlackHandler) HandleCreateTrigger(writer http.ResponseWriter, request *http.Request) {
	payload, err := h.getSlackPayload(writer, request)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	r := CreateTriggerRequest{SlackPayload: payload}
	response, err := h.CreateTrigger(r)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	h.ReturnResponse(writer, response)
}

func (h *SlackHandler) HandleDeleteTrigger(writer http.ResponseWriter, request *http.Request) {
	payload, err := h.getSlackPayload(writer, request)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	r := DeleteTriggerRequest{SlackPayload: payload}
	response, err := h.DeleteTrigger(r)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	h.ReturnResponse(writer, response)
}

func (h *SlackHandler) HandleClearStatus(writer http.ResponseWriter, request *http.Request) {
	payload, err := h.getSlackPayload(writer, request)
	if err != nil {
		h.ReturnError(writer, err)
		return
	}

	r := ClearStatusRequest{SlackPayload: payload}
	msg, err := h.ClearStatus(r)
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

func (h *SlackHandler) getSlackPayload(writer http.ResponseWriter, request *http.Request) (SlackPayload, error) {
	verifier, err := slack.NewSecretsVerifier(request.Header, h.signingSecret)
	if err != nil {
		return SlackPayload{}, errors.New("SlackOverload cannot receive requests right now")
	}

	request.Body = ioutil.NopCloser(io.TeeReader(request.Body, &verifier))
	s, err := slack.SlashCommandParse(request)
	if err != nil {
		return SlackPayload{}, errors.New("SlackOverload and Slack are not talking the same language right now. We will have to try again later. Sorry!")
	}

	if err = verifier.Ensure(); err != nil {
		writer.WriteHeader(http.StatusUnauthorized)
		return SlackPayload{}, errors.New("Unauthorized message sent to SlackOverload. Rejected.")
	}

	return SlackPayload{
		SlackId:  s.UserID,
		UserName: s.UserName,
		TeamId:   s.TeamID,
		TeamName: s.TeamDomain,
		Text:     s.Text,
	}, nil
}
