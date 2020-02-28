package slackoverload

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nlopes/slack"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const (
	PresenceAway   = "away"
	PresenceActive = "auto"
	SlackOAuthURL  = "https://slack.com/api/oauth.v2.access"
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

type DeleteTriggerRequest struct {
	SlackPayload
	Name string
}

type MuteChannelRequest struct {
	SlackPayload
	Duration string
}

type SlackPayload struct {
	SlackId     string `json:"slack-id"`
	UserName    string `json:"slack-user"`
	TeamId      string `json:"team-id"`
	TeamName    string `json:"team-name"`
	ChannelId   string `json:"channel-id"`
	ChannelName string `json:"channel-name"`
}

type UnmuteChannelScheduledRequest struct {
	ScheduledRequest
}

type ScheduledRequest struct {
	SlackPayload
	ExecuteAt time.Time `json:"execute-at"`
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

type App struct {
	Debug bool
	Storage
	Secrets
}

func (a *App) Init(secrets Secrets) error {
	a.Secrets = secrets

	store, err := NewStorageClient()
	if err != nil {
		return err
	}

	a.Storage = store
	return nil
}

func (a *App) ClearStatus(r ClearStatusRequest) (slack.Msg, error) {
	fmt.Printf("%s /clear-status for %s(%s) on %s(%s)\n",
		now(), r.UserName, r.SlackId, r.TeamName, r.TeamId)

	userId, err := a.lookupUserIdFromSlackId(r.SlackId)
	if err != nil {
		return a.handleUserNotRegistered(), nil
	}

	action := Action{
		Presence: PresenceActive,
	}
	err = a.applyActionToAllSlacks(userId, action)
	if err != nil {
		return slack.Msg{}, err
	}

	msg := slack.Msg{
		Type: slack.ResponseTypeEphemeral,
		Blocks: slack.Blocks{BlockSet: []slack.Block{
			slack.SectionBlock{
				Type: slack.MBTSection,
				Text: &slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: "Your status has been cleared :boom:",
				},
			},
		}},
	}

	return msg, nil
}

func (a *App) ListTriggers(r ListTriggersRequest) (slack.Msg, error) {
	fmt.Printf("%s /list-trigger for %s(%s) on %s(%s)\n",
		now(), r.UserName, r.SlackId, r.TeamName, r.TeamId)

	userId, err := a.lookupUserIdFromSlackId(r.SlackId)
	if err != nil {
		return a.handleUserNotRegistered(), nil
	}

	userDir := userId + "/"
	blobNames, err := a.Storage.ListContainer("triggers", userDir)
	if err != nil {
		return slack.Msg{}, err
	}

	msg := slack.Msg{
		ResponseType: slack.ResponseTypeEphemeral,
		Blocks: slack.Blocks{BlockSet: []slack.Block{
			slack.SectionBlock{
				Type: slack.MBTSection,
				Text: &slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: "Here are the triggers that you have defined:",
				},
			},
			slack.NewDividerBlock(),
		}},
	}

	for _, blobName := range blobNames {
		triggerName := strings.TrimPrefix(blobName, userDir)
		trigger, err := a.getTrigger(userId, triggerName)
		if err != nil {
			return slack.Msg{}, err
		}

		triggerBlock := slack.SectionBlock{
			Type: slack.MBTSection,
			Fields: []*slack.TextBlockObject{
				{
					Type: slack.MarkdownType,
					Text: fmt.Sprintf("*Name*: %s\n*Status*: %s\n*Do Not Disturb*: %t\n*Default Duration*: %s",
						trigger.Name, trigger.StatusText, trigger.DnD, trigger.Duration),
				},
				{
					Type: slack.MarkdownType,
					Text: trigger.StatusEmoji,
				},
			},
		}
		msg.Blocks.BlockSet = append(msg.Blocks.BlockSet, triggerBlock)
	}

	return msg, nil
}

func (a *App) Trigger(r TriggerRequest) (slack.Msg, error) {
	fmt.Printf("%s /trigger %s from %s(%s) on %s(%s)\n",
		now(), r.Name, r.UserName, r.SlackId, r.TeamName, r.TeamId)

	userId, err := a.lookupUserIdFromSlackId(r.SlackId)
	if err != nil {
		return a.handleUserNotRegistered(), nil
	}

	action, err := a.getTrigger(userId, r.Name)
	if err != nil {
		return slack.Msg{}, err
	}

	err = a.applyActionToAllSlacks(userId, action.Action)
	if err != nil {
		return slack.Msg{}, err
	}

	msg := slack.Msg{
		Type: slack.ResponseTypeEphemeral,
		Blocks: slack.Blocks{BlockSet: []slack.Block{
			slack.SectionBlock{
				Type: slack.MBTSection,
				Text: &slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: fmt.Sprintf("Triggered *%s* %s", action.Name, action.StatusEmoji),
				},
			},
		}},
	}

	return msg, nil
}

func (a *App) RefreshOAuthToken(r OAuthRequest) (string, error) {
	fmt.Printf("%s /oauth from %s\n",
		now(), r.UserId)

	clientId, err := a.GetSlackClientId()
	if err != nil {
		return "", err
	}
	clientSecret, err := a.GetSlackClientSecret()
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
	existingUser, err := a.getSlackToken(tr.User.Id)
	if err == nil && existingUser.UserId != "" {
		userId = existingUser.UserId
	} else if r.UserId != "" {
		userId = r.UserId
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
	err = a.setSlackToken(t)
	if err != nil {
		return "", errors.Wrapf(err, "error saving oauth token for %s on %s(%s)", tr.User.Id, tr.Team.Name, tr.Team.Id)
	}

	user, err := a.getCurrentUser(userId)
	if err != nil {
		return "", err
	}
	user.AddSlackUser(tr.User.Id, tr.Team.Id)
	err = a.setCurrentUser(user)
	if err != nil {
		return "", errors.Wrapf(err, "error saving user mapping for %s -> %s", userId, tr.User.Id)
	}

	return userId, nil
}

func (a *App) LinkSlack(r SlackPayload) (slack.Msg, error) {
	fmt.Printf("%s /link-slack from %s(%s) on %s(%s)\n",
		now(), r.UserName, r.SlackId, r.TeamName, r.TeamId)

	userId, err := a.lookupUserIdFromSlackId(r.SlackId)
	if err != nil {
		return a.handleUserNotRegistered(), nil
	}

	oauthUrl := "https://slack.com/oauth/v2/authorize"
	scopes := "commands&user_scope=dnd:read,dnd:write,users:write,users.profile:write,read,post"
	clientId := "2413351231.504877832356"
	magiclink := fmt.Sprintf("%s?scope=%s&client_id=%s&state=%s",
		oauthUrl, scopes, clientId, userId)

	msg := slack.Msg{
		Type: slack.ResponseTypeEphemeral,
		Blocks: slack.Blocks{BlockSet: []slack.Block{
			slack.SectionBlock{
				Type: slack.MBTSection,
				Text: &slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: fmt.Sprintf("Click the link below to associate another Slack account with this account.\n\n<%s|Link Slack Account>", magiclink),
				},
			},
		}},
	}

	return msg, nil
}

func (a *App) applyActionToAllSlacks(userId string, action Action) error {
	user, err := a.getCurrentUser(userId)
	if err != nil {
		return err
	}

	var g errgroup.Group
	// TODO: collect the failed team names, right now we don't have the team name, just id
	for _, slackUser := range user.SlackUsers {
		slackId := slackUser.ID
		g.Go(func() error {
			return a.updateSlackStatus(slackId, action)
		})
	}

	return g.Wait()
}

func (a *App) updateSlackStatus(slackId string, action Action) error {
	token, err := a.getSlackToken(slackId)
	if err != nil {
		return err
	}

	fmt.Printf("Updating slack status for %s (%s) on team %s to %#v\n", token.UserId, token.SlackId, token.TeamId, action)

	api := slack.New(token.AccessToken, slack.OptionDebug(a.Debug))

	var g errgroup.Group

	g.Go(func() error {
		err = api.SetUserPresence(string(action.Presence))
		return errors.Wrap(err, "could not set presence")
	})

	g.Go(func() error {
		err = api.SetUserCustomStatus(action.StatusText, action.StatusEmoji, action.DurationInMinutes())
		return errors.Wrap(err, "could not set status")
	})

	g.Go(func() error {
		if action.DnD {
			_, err = api.SetSnooze(int(action.DurationInMinutes()))
			return errors.Wrap(err, "could not set do not disturb")
		}

		// Check if we should turn off DND
		dndState, err := api.GetDNDInfo(&slackId)
		if err != nil {
			return errors.Wrapf(err, "could not retrieve user's current DND state")
		}
		if dndState.SnoozeEnabled {
			_, err = api.EndSnooze()
			return errors.Wrap(err, "could not end do not disturb")
		}
		return nil
	})

	return g.Wait()
}

// CreateTrigger accepts a trigger definition and saves it
func (a *App) CreateTrigger(r CreateTriggerRequest) (slack.Msg, error) {
	fmt.Printf("%s /create-trigger %q from %s(%s) on %s(%s)\n",
		now(), r.Definition, r.UserName, r.SlackId, r.TeamName, r.TeamId)

	userId, err := a.lookupUserIdFromSlackId(r.SlackId)
	if err != nil {
		return a.handleUserNotRegistered(), nil
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

	key := path.Join(userId, tmpl.Name)
	err = a.Storage.SetBlob("triggers", key, tmplB)
	if err != nil {
		return slack.Msg{}, err
	}

	msg := slack.Msg{
		Type: slack.ResponseTypeEphemeral,
		Text: fmt.Sprintf("Created trigger %s", tmpl.Name),
	}
	return msg, nil
}

func (a *App) DeleteTrigger(r DeleteTriggerRequest) (slack.Msg, error) {
	fmt.Printf("%s /delete-trigger %s from %s(%s) on %s(%s)\n",
		now(), r.Name, r.UserName, r.SlackId, r.TeamName, r.TeamId)

	userId, err := a.lookupUserIdFromSlackId(r.SlackId)
	if err != nil {
		return a.handleUserNotRegistered(), nil
	}

	key := path.Join(userId, r.Name)
	err = a.Storage.DeleteBlob("triggers", key)
	if err != nil {
		if strings.Contains(err.Error(), "BlobNotFound") {
			return slack.Msg{}, errors.Errorf("Could not delete trigger %q because it is not defined", r.Name)
		}
		return slack.Msg{}, err
	}

	msg := slack.Msg{
		Type: slack.ResponseTypeEphemeral,
		Blocks: slack.Blocks{BlockSet: []slack.Block{
			slack.SectionBlock{
				Type: slack.MBTSection,
				Text: &slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: fmt.Sprintf("Deleted trigger *%s*", r.Name),
				},
			},
		}},
	}

	return msg, nil
}

func (a *App) getTrigger(userId string, name string) (ActionTemplate, error) {
	key := path.Join(userId, name)
	b, err := a.Storage.GetBlob("triggers", key)
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

func (a *App) lookupUserIdFromSlackId(slackId string) (string, error) {
	slackToken, err := a.getSlackToken(slackId)
	if err != nil {
		return "", err
	}
	return slackToken.UserId, nil
}

func (a *App) handleUserNotRegistered() slack.Msg {
	fmt.Println("User not registered")
	msg := slack.Msg{
		Type: slack.ResponseTypeEphemeral,
		Blocks: slack.Blocks{BlockSet: []slack.Block{
			slack.SectionBlock{
				Type: slack.MBTSection,
				Text: &slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: "Your account hasn't activated the Slack Overload app yet.\n\n:heavy_plus_sign:*New Users*\nFollow the <https://slackoverload.com/quickstart|QuickStart> to get started.\n\n\n:link: *Existing Users*\nRun `/link-slack` from another Slack account that is already activated to link it this account.",
				},
			},
		}},
	}

	return msg
}

func (a *App) MuteChannel(r MuteChannelRequest) (slack.Msg, error) {
	fmt.Printf("%s /mute-channel %s from %s(%s) on %s(%s)\n",
		now(), r.Duration, r.UserName, r.SlackId, r.TeamName, r.TeamId)

	token, err := a.getSlackToken(r.SlackId)
	if err != nil {
		return slack.Msg{}, err
	}

	values := url.Values{
		"token": {token.AccessToken},
	}
	body := strings.NewReader(values.Encode())
	request, _ := http.NewRequest(http.MethodPost, "https://slack.com/api/users.prefs.get", body)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return slack.Msg{}, err
	}

	defer response.Body.Close()
	result, _ := ioutil.ReadAll(response.Body)
	fmt.Println(string(result))
	return slack.Msg{Text: "found it!"}, nil
}

type User struct {
	ID         string      `json:"id"`
	SlackUsers []SlackUser `json:"slack-users"`
}

type SlackUser struct {
	ID     string `json:"id"`
	TeamID string `json:"team"`
}

func (u *User) AddSlackUser(slackId, teamId string) {
	for _, su := range u.SlackUsers {
		if su.ID == slackId {
			return
		}
	}

	u.SlackUsers = append(u.SlackUsers, SlackUser{ID: slackId, TeamID: teamId})
}

func (a *App) getCurrentUser(userId string) (User, error) {
	b, err := a.Storage.GetBlob("users", userId)
	if err != nil {
		if strings.Contains(err.Error(), "BlobNotFound") {
			return User{ID: userId}, nil
		}
		return User{}, err
	}

	var user User
	err = json.Unmarshal(b, &user)
	if err != nil {
		return User{}, errors.Wrapf(err, "error parsing user configuration for %q", userId)
	}

	return user, nil
}

func (a *App) setCurrentUser(user User) error {
	userId := user.ID
	if userId == "" {
		return errors.New("cannot save user, no ID was given")
	}

	b, err := json.Marshal(user)
	if err != nil {
		return errors.Wrapf(err, "error marshaling user\n%#v", user)
	}

	return a.Storage.SetBlob("users", userId, b)
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

func (a *App) getSlackToken(slackId string) (SlackToken, error) {
	accessToken, tags, err := a.GetSecret("oauth-" + slackId)
	if err != nil {
		return SlackToken{}, err
	}

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

	if t.UserId == "" {
		return t, errors.Errorf("Slack user %s has not authorized the Slack Overload app", slackId)
	}

	return t, nil
}

func (a *App) setSlackToken(t SlackToken) error {
	tags := map[string]*string{
		"user":   &t.UserId,
		"team":   &t.TeamId,
		"scopes": &t.Scopes,
	}
	return a.SetSecret("oauth-"+t.SlackId, t.AccessToken, tags)
}

func now() string {
	return time.Now().Format("Mon 02 Jan 2006 15:04 MST")
}
