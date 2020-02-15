package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/google/uuid"
	_ "github.com/google/uuid"
	"github.com/nlopes/slack"
	"github.com/pkg/errors"
)

const (
	PresenceAway     = "away"
	PresenceActive   = "auto"
	SlackOAuthURL    = "https://slack.com/api/oauth.v2.access"
	OAuthRedirectURL = "https://cmd.slackoverload.com/oauth"
)

type Presence string

type Action struct {
	Presence    Presence `json:"presence"`
	StatusText  string   `json:"status-text,omitempty"`
	StatusEmoji string   `json:"status-emoji,omitempty"`
	DnD         bool     `json:"dnd,omitempty"`
	Duration    string   `json:"duration,omitempty"`
}

func (a Action) ParseDuration() (time.Duration, error) {
	if a.Duration == "" {
		return time.Duration(0), nil
	}

	r := regexp.MustCompile(`^(\d+)([wd])$`)
	match := r.FindStringSubmatch(a.Duration)
	if len(match) == 0 {
		return time.ParseDuration(a.Duration)
	}

	num, err := strconv.Atoi(match[1])
	if err != nil {
		return time.Duration(0), err
	}

	var unit time.Duration
	switch match[2] {
	case "d":
		unit = day
	case "w":
		unit = week
	}
	return time.Duration(num) * unit, nil
}

func (a Action) DurationInMinutes() int64 {
	d, _ := a.ParseDuration()
	return int64(d.Seconds())
}

type ActionTemplate struct {
	Name   string `json:"name"`
	TeamId string `json:"team"`
	Action `json:"action"`
}

func (t ActionTemplate) ToString() string {
	statusText := ""
	if t.StatusText != "" {
		statusText = fmt.Sprintf(" %s", t.StatusText)
	}

	emojiText := ""
	if t.StatusEmoji != "" {
		emojiText = fmt.Sprintf(" (%s)", t.StatusEmoji)
	}

	dndText := ""
	if t.DnD {
		dndText = fmt.Sprintf(" DND")
	}

	durationText := ""
	if t.Duration != "" {
		durationText = fmt.Sprintf(" for %s", t.Duration)
	}

	return fmt.Sprintf("%s = %s%s%s%s", t.Name, statusText, emojiText, dndText, durationText)
}

type ClearStatusRequest struct {
	SlackPayload
	Global bool
}

type ListTriggersRequest struct {
	SlackPayload
	Global bool
}

type TriggerRequest struct {
	SlackPayload
	Name string
}

type CreateTriggerRequest struct {
	SlackPayload
	Definition string
}

type SlackPayload struct {
	SlackId  string
	UserName string
	TeamId   string
	TeamName string
}

type OAuthRequest struct {
	UserId    string
	AuthGrant string
}

type OAuthResponse struct {
	Team OAuthTeam `json:"team"`
	User OAuthUser `json:"authed_user"`
}

type OAuthTeam struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type OAuthUser struct {
	Id          string `json:"id"`
	Scopes      string `json:"scope"`
	AccessToken string `json:"access_token"`
}

func ClearStatus(r ClearStatusRequest) error {
	fmt.Printf("Clearing Status for %s(%s) on %s(%s)\n",
		r.UserName, r.SlackId, r.TeamName, r.TeamId)

	action := Action{
		Presence: PresenceActive,
	}
	return updateSlackStatus(r.SlackPayload, action)
}

func ListTriggers(r ListTriggersRequest) (slack.Msg, error) {
	fmt.Printf("Listing Triggers for %s(%s) on %s(%s)\n",
		r.UserName, r.SlackId, r.TeamName, r.TeamId)

	userId, err := getCurrentUserId(r.SlackId)
	if err != nil {
		return slack.Msg{}, err
	}

	client, err := NewStorageClient()
	if err != nil {
		return slack.Msg{}, err
	}

	userDir := userId + "/"
	blobNames, err := client.listContainer("triggers", userDir)
	if err != nil {
		return slack.Msg{}, err
	}

	triggers := make([]string, len(blobNames))
	for i, blobName := range blobNames {
		triggerName := strings.TrimPrefix(blobName, userDir)
		trigger, err := getTrigger(userId, triggerName)
		if err != nil {
			return slack.Msg{}, err
		}

		triggers[i] = trigger.ToString()
	}

	msg := slack.Msg{
		ResponseType: slack.ResponseTypeEphemeral,
		Text:         strings.Join(triggers, "\n"),
	}

	return msg, nil
}

func Trigger(r TriggerRequest) error {
	fmt.Printf("Triggering %s from %s(%s) on %s(%s)\n",
		r.Name, r.UserName, r.SlackId, r.TeamName, r.TeamId)

	userId, err := getCurrentUserId(r.SlackId)
	if err != nil {
		return err
	}

	action, err := getTrigger(userId, r.Name)
	if err != nil {
		return err
	}

	return updateSlackStatus(r.SlackPayload, action.Action)
}

func RefreshOAuthToken(r OAuthRequest) (string, error) {
	clientId, _, err := getSecret("slack-client-id")
	if err != nil {
		return "", err
	}
	clientSecret, _, err := getSecret("slack-client-secret")
	if err != nil {
		return "", err
	}

	data := url.Values{
		"client_id":     {clientId},
		"client_secret": {clientSecret},
		"code":          {r.AuthGrant},
	}
	response, err := http.DefaultClient.PostForm(SlackOAuthURL, data)
	if err != nil {
		return "", errors.Wrap(err, "error requesting oauth token")
	}

	var tr OAuthResponse
	err = json.NewDecoder(response.Body).Decode(&tr)
	if err != nil {
		return "", errors.Wrap(err, "error unmarshaling oauth token response")
	}

	var userId string
	existingUser, err := getSlackToken(tr.User.Id)
	if err == nil && existingUser.UserId != "" {
		userId = existingUser.UserId
	} else {
		newId, err := uuid.NewRandom()
		if err != nil {
			return "", errors.Wrapf(err, "error generating user id for slack user %s on team %s (%s)", tr.User.Id, tr.Team.Name, tr.Team.Id)
		}
		userId = newId.String()
	}

	t := SlackToken{
		UserId:      userId,
		SlackId:     tr.User.Id,
		AccessToken: tr.User.AccessToken,
		TeamId:      tr.Team.Id,
		Scopes:      tr.User.Scopes,
	}
	err = setSlackToken(t)
	return userId, errors.Wrapf(err, "error saving oauth token for %s on %s(%s)", tr.User.Id, tr.Team.Name, tr.Team.Id)
}

func updateSlackStatus(payload SlackPayload, action Action) error {
	token, err := getSlackToken(payload.SlackId)
	if err != nil {
		return err
	}

	api := slack.New(token.AccessToken, slack.OptionDebug(*debugFlag))

	err = api.SetUserPresence(string(action.Presence))
	if err != nil {
		err = errors.Wrap(err, "could not set presence")
		return err
	}

	err = api.SetUserCustomStatus(action.StatusText, action.StatusEmoji, action.DurationInMinutes())
	if err != nil {
		return errors.Wrap(err, "could not set status")
	}

	if !action.DnD {
		dndState, err := api.GetDNDInfo(&payload.SlackId)
		if err != nil {
			return errors.Wrapf(err, "could not retrieve user's current DND state")
		}
		if dndState.SnoozeEnabled {
			_, err = api.EndSnooze()
			if err != nil {
				return errors.Wrap(err, "could not end do not disturb")
			}
		}
	} else if action.DnD {
		_, err = api.SetSnooze(int(action.DurationInMinutes()))
		if err != nil {
			return errors.Wrap(err, "could not set do not disturb")
		}
	}
	return nil
}

// CreateTrigger accepts a trigger definition and saves it
func CreateTrigger(r CreateTriggerRequest) (slack.Msg, error) {
	fmt.Printf("CreateTrigger %s from %s(%s) on %s(%s)\n",
		r.Definition, r.UserName, r.SlackId, r.TeamName, r.TeamId)

	userId, err := getCurrentUserId(r.SlackId)
	if err != nil {
		return slack.Msg{}, err
	}

	tmpl, err := parseTemplate(r.Definition)
	if err != nil {
		return slack.Msg{}, err
	}

	tmpl.TeamId = r.TeamId
	tmplB, err := json.Marshal(tmpl)
	if err != nil {
		return slack.Msg{}, errors.Wrapf(err, "error marshaling trigger %s for %s(%s) on %s(%s): %#v",
			tmpl.Name, r.UserName, r.SlackId, r.TeamName, r.TeamId, tmpl)
	}

	client, err := NewStorageClient()
	if err != nil {
		return slack.Msg{}, err
	}

	key := path.Join(userId, tmpl.Name)
	err = client.setBlob("triggers", key, tmplB)
	if err != nil {
		return slack.Msg{}, err
	}

	msg := slack.Msg{
		Type: slack.ResponseTypeEphemeral,
		Text: fmt.Sprintf("Created trigger %s", tmpl.Name),
	}
	return msg, nil
}

func getTrigger(userId string, name string) (ActionTemplate, error) {
	client, err := NewStorageClient()
	if err != nil {
		return ActionTemplate{}, err
	}

	key := path.Join(userId, name)
	b, err := client.getBlob("triggers", key)
	if err != nil {
		if strings.Contains(err.Error(), "BlobNotFound") {
			return ActionTemplate{}, errors.Errorf("trigger %s not registered", name)
		}
		return ActionTemplate{}, err
	}

	var action ActionTemplate
	err = json.Unmarshal(b, &action)
	if err != nil {
		return ActionTemplate{}, errors.Wrapf(err, "error unmarshaling trigger %s: %s", name, string(b))
	}

	return action, nil
}

func getCurrentUserId(slackId string) (string, error) {
	slackToken, err := getSlackToken(slackId)
	if err != nil {
		return "", err
	}
	return slackToken.UserId, nil
}

// parseAction definition into an Action
// Example:
// vacation = vacay (🌴) DND for 1w
// name = vacation
// status = vacay
// emoji = 🌴
// DND = Yes
// duration = 1w
func parseTemplate(def string) (ActionTemplate, error) {
	// Test out at https://regex101.com/r/8v180Z/5
	const pattern = `^([\w-_]+)[ ]?=(?:[ ]?(.+))?[ ]+\((.*)\)( DND)?(?: for (\d[wdhms]+))?$`
	r := regexp.MustCompile(pattern)
	match := r.FindStringSubmatch(def)
	if len(match) == 0 {
		return ActionTemplate{}, errors.Errorf("Invalid trigger definition %q. Try /create-trigger vacation = I'm on a boat! (⛵️) DND for 1w", def)
	}

	template := ActionTemplate{
		Name: match[1],
		Action: Action{
			Presence:    PresenceAway,
			StatusText:  match[2],
			StatusEmoji: match[3],
			DnD:         match[4] != "",
			Duration:    match[5],
		},
	}

	_, err := template.ParseDuration()
	if err != nil {
		return ActionTemplate{}, errors.Errorf("invalid duration in trigger definition %q, here are some examples: 15m, 1h, 2d, 1w", template.Duration)
	}

	return template, nil
}

const (
	day  = 24 * time.Hour
	week = 7 * day
)

type SlackToken struct {
	UserId      string
	SlackId     string
	AccessToken string
	TeamId      string
	Scopes      string
}

func getSlackToken(slackId string) (SlackToken, error) {
	accessToken, tags, err := getSecret("oauth-" + slackId)

	getTag := func(key string) string {
		value, ok := tags[key]
		if !ok {
			return ""
		}
		return *value
	}

	t := SlackToken{
		UserId:      getTag("user"),
		SlackId:     slackId,
		AccessToken: accessToken,
		TeamId:      getTag("team"),
		Scopes:      getTag("scopes"),
	}

	return t, err
}

func setSlackToken(t SlackToken) error {
	tags := map[string]*string{
		"user":   &t.UserId,
		"team":   &t.TeamId,
		"scopes": &t.Scopes,
	}
	return setSecret("oauth-"+t.SlackId, t.AccessToken, tags)
}

func getSecret(key string) (string, map[string]*string, error) {
	client, err := getKeyVaultClient()
	if err != nil {
		return "", nil, errors.Wrap(err, "could not authenticate to Azure using ambient environment")
	}

	// Timebox getting the secret because a bad client or auth will hang forever
	cxt, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	result, err := client.GetSecret(cxt, vaultURL, key, "")
	if err != nil {
		defer cancel()
		return "", nil, errors.Wrapf(err, "could not load secret %q from vault", key)
	}

	return *result.Value, result.Tags, nil
}

func setSecret(key string, value string, tags map[string]*string) error {
	client, err := getKeyVaultClient()
	if err != nil {
		return errors.Wrap(err, "could not authenticate to Azure using ambient environment")
	}

	// Timebox getting the secret because a bad client or auth will hang forever
	cxt, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	_, err = client.SetSecret(cxt, vaultURL, key, keyvault.SecretSetParameters{
		Value: &value,
		Tags:  tags,
	})
	if err != nil {
		defer cancel()
		return errors.Wrapf(err, "error saving secret %s", key)
	}

	return nil
}
