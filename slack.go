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
	Presence    Presence
	StatusText  string
	StatusEmoji string
	DnD         bool
	Duration    int64
}

type ActionTemplate struct {
	Name   string
	TeamId string
	Action
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
	if t.Duration != 0 {
		durationText = fmt.Sprintf(" for %s", time.Duration(t.Duration).String())
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
	UserId   string
	UserName string
	TeamId   string
	TeamName string
}

type OAuthRequest struct {
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
		r.UserName, r.UserId, r.TeamName, r.TeamId)

	action := Action{
		Presence: PresenceActive,
	}
	return updateSlackStatus(r.SlackPayload, action)
}

func ListTriggers(r ListTriggersRequest) (slack.Msg, error) {
	fmt.Printf("Listing Triggers for %s(%s) on %s(%s)\n",
		r.UserName, r.UserId, r.TeamName, r.TeamId)

	client, err := NewStorageClient()
	if err != nil {
		return slack.Msg{}, err
	}

	userDir := r.UserId + "/"
	blobNames, err := client.listContainer("triggers", userDir)
	if err != nil {
		return slack.Msg{}, err
	}

	triggers := make([]string, len(blobNames))
	for i, blobName := range blobNames {
		triggerName := strings.TrimPrefix(blobName, userDir)
		trigger, err := getTrigger(r.UserId, triggerName)
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
		r.Name, r.UserName, r.UserId, r.TeamName, r.TeamId)

	action, err := getTrigger(r.UserId, r.Name)
	if err != nil {
		return err
	}

	return updateSlackStatus(r.SlackPayload, action.Action)
}

func RefreshOAuthToken(r OAuthRequest) error {
	clientId, err := getSecret("slack-client-id")
	if err != nil {
		return err
	}
	clientSecret, err := getSecret("slack-client-secret")
	if err != nil {
		return err
	}

	data := url.Values{
		"client_id":     {clientId},
		"client_secret": {clientSecret},
		"code":          {r.AuthGrant},
	}
	response, err := http.DefaultClient.PostForm(SlackOAuthURL, data)
	if err != nil {
		return errors.Wrap(err, "error requesting oauth token")
	}

	var tr OAuthResponse
	err = json.NewDecoder(response.Body).Decode(&tr)
	if err != nil {
		return errors.Wrap(err, "error unmarshaling oauth token response")
	}

	err = setSlackToken(tr.User.Id, tr.Team.Id, tr.User.AccessToken, tr.User.Scopes)
	return errors.Wrapf(err, "error saving oauth token for %s on %s(%s)", tr.User.Id, tr.Team.Name, tr.Team.Id)
}

func updateSlackStatus(payload SlackPayload, action Action) error {
	token, err := getSlackToken(payload.UserId)
	if err != nil {
		return err
	}

	api := slack.New(token, slack.OptionDebug(*debugFlag))

	err = api.SetUserPresence(string(action.Presence))
	if err != nil {
		err = errors.Wrap(err, "could not set presence")
		return err
	}

	err = api.SetUserCustomStatus(action.StatusText, action.StatusEmoji, action.Duration)
	if err != nil {
		return errors.Wrap(err, "could not set status")
	}

	if !action.DnD {
		dndState, err := api.GetDNDInfo(&payload.UserId)
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
		_, err = api.SetSnooze(int(action.Duration))
		if err != nil {
			return errors.Wrap(err, "could not set do not disturb")
		}
	}
	return nil
}

// CreateTrigger accepts a trigger definition and saves it
func CreateTrigger(r CreateTriggerRequest) (slack.Msg, error) {
	fmt.Printf("CreateTrigger %s from %s(%s) on %s(%s)\n",
		r.Definition, r.UserName, r.UserId, r.TeamName, r.TeamId)

	tmpl, err := parseTemplate(r.Definition)
	if err != nil {
		return slack.Msg{}, err
	}

	tmpl.TeamId = r.TeamId

	tmplB, err := json.Marshal(tmpl)
	if err != nil {
		return slack.Msg{}, errors.Wrapf(err, "error marshaling trigger %s for %s(%s) on %s(%s): %#v",
			tmpl.Name, r.UserName, r.UserId, r.TeamName, r.TeamId, tmpl)
	}

	client, err := NewStorageClient()
	if err != nil {
		return slack.Msg{}, err
	}

	key := path.Join(r.UserId, tmpl.Name)
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

	duration, err := parseDuration(match[5])
	if err != nil {
		return ActionTemplate{}, errors.Errorf("invalid duration in trigger definition %q, here are some examples: 15m, 1h, 2d, 1w", match[5])
	}

	template := ActionTemplate{
		Name: match[1],
		Action: Action{
			Presence:    PresenceAway,
			StatusText:  match[2],
			StatusEmoji: match[3],
			DnD:         match[4] != "",
			Duration:    int64(duration.Minutes()),
		},
	}
	return template, nil
}

const (
	day  = 24 * time.Hour
	week = 7 * day
)

func parseDuration(value string) (time.Duration, error) {
	if value == "" {
		return time.Duration(0), nil
	}

	r := regexp.MustCompile(`^(\d+)([wd])$`)
	match := r.FindStringSubmatch(value)
	if len(match) == 0 {
		return time.ParseDuration(value)
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

func getSlackToken(userId string) (string, error) {
	return getSecret("oauth-" + userId)
}

func setSlackToken(userId string, teamId string, token string, scopes string) error {
	tags := map[string]*string{
		"scopes": &scopes,
		"team":   &teamId,
	}
	return setSecret("oauth-"+userId, token, tags)
}

func getSecret(key string) (string, error) {
	client, err := getKeyVaultClient()
	if err != nil {
		return "", errors.Wrap(err, "could not authenticate to Azure using ambient environment")
	}

	// Timebox getting the secret because a bad client or auth will hang forever
	cxt, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	result, err := client.GetSecret(cxt, vaultURL, key, "")
	if err != nil {
		defer cancel()
		return "", errors.Wrapf(err, "could not load secret %q from vault", key)
	}

	return *result.Value, nil
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
