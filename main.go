package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/nlopes/slack"
	"github.com/pkg/errors"
)

type Presence string

const (
	PresenceAway   = "away"
	PresenceActive = "auto"
	vaultURL       = "https://slackoverload.vault.azure.net/"
)

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
	http.HandleFunc("/slack/cmd/create-trigger", HandleCreateTrigger)
	http.HandleFunc("/slack/cmd/clear-status", HandleClearStatus)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func HandleTrigger(writer http.ResponseWriter, request *http.Request) {
	tr := TriggerRequest{
		SlackPayload: getSlackPayload(request),
		Name:         request.FormValue("text"),
	}

	err := Trigger(tr)
	if err != nil {
		HandleError(writer, err)
		return
	}

	writer.WriteHeader(200)
}

func HandleCreateTrigger(writer http.ResponseWriter, request *http.Request) {
	cr := CreateTriggerRequest{
		SlackPayload: getSlackPayload(request),
		Definition:   request.FormValue("text"),
	}

	err := CreateTrigger(cr)
	if err != nil {
		HandleError(writer, err)
		return
	}

	writer.WriteHeader(200)
}

func HandleClearStatus(writer http.ResponseWriter, request *http.Request) {
	tr := ClearStatusRequest{
		SlackPayload: getSlackPayload(request),
	}

	err := ClearStatus(tr)
	if err != nil {
		HandleError(writer, err)
		return
	}

	writer.WriteHeader(200)
}

func HandleError(writer http.ResponseWriter, err error) {
	fmt.Printf("%v\n", err)
	writer.Header().Set("Content-type", "application/json")
	writer.WriteHeader(200)
	jErr := buildSlackError(err)
	fmt.Printf("%s\n", string(jErr))
	writer.Write(jErr)
}

func buildSlackError(err error) []byte {
	response := map[string]string{
		"response_type": "ephemeral",
		"text":          err.Error(),
	}

	b, _ := json.Marshal(response)
	return b
}

func getSlackPayload(request *http.Request) SlackPayload {
	return SlackPayload{
		UserId:   request.FormValue("user_id"),
		UserName: request.FormValue("user_name"),
		TeamId:   request.FormValue("team_id"),
		TeamName: request.FormValue("team_domain"),
	}
}

type ClearStatusRequest struct {
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

func ClearStatus(r ClearStatusRequest) error {
	fmt.Printf("Clearing Status for %s(%s) on %s(%s)\n",
		r.UserName, r.UserId, r.TeamName, r.TeamId)

	action := Action{
		Presence: PresenceActive,
	}
	return updateSlackStatus(r.SlackPayload, action)
}

func Trigger(r TriggerRequest) error {
	fmt.Printf("Triggering %s from %s(%s) on %s(%s)\n",
		r.Name, r.UserName, r.UserId, r.TeamName, r.TeamId)

	action, err := getTrigger(r)
	if err != nil {
		return err
	}

	return updateSlackStatus(r.SlackPayload, action)
}

func updateSlackStatus(payload SlackPayload, action Action) error {
	token, err := getSlackToken()
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
func CreateTrigger(r CreateTriggerRequest) error {
	fmt.Printf("CreateTrigger %s from %s(%s) on %s(%s)\n",
		r.Definition, r.UserName, r.UserId, r.TeamName, r.TeamId)

	tmpl, err := parseTemplate(r.Definition)
	if err != nil {
		return err
	}

	tmpl.TeamId = r.TeamId

	tmplB, err := json.Marshal(tmpl)
	if err != nil {
		return errors.Wrapf(err, "error marshaling trigger %s for %s(%s) on %s(%s): %#v",
			tmpl.Name, r.UserName, r.UserId, r.TeamName, r.TeamId, tmpl)
	}

	client, err := NewStorageClient()
	if err != nil {
		return err
	}

	key := path.Join(r.UserId, tmpl.Name)
	return client.setBlob("triggers", key, tmplB)
}

func getTrigger(r TriggerRequest) (Action, error) {
	client, err := NewStorageClient()
	if err != nil {
		return Action{}, err
	}

	key := path.Join(r.UserId, r.Name)
	b, err := client.getBlob("triggers", key)
	if err != nil {
		if strings.Contains(errors.Cause(err).Error(), "BlobNotFound") {
			return Action{}, errors.Errorf("trigger %s not registered", r.Name)
		}
		return Action{}, err
	}

	var action Action
	err = json.Unmarshal(b, &action)
	if err != nil {
		return Action{}, errors.Wrapf(err, "error unmarshaling trigger %s: %s", r.Name, string(b))
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

func getSlackToken() (string, error) {
	client, err := getKeyVaultClient()
	if err != nil {
		fmt.Println("Loading slack token from env var...")
		token := os.Getenv("SLACK_TOKEN")
		if token == "" {
			return "", fmt.Errorf("could not authenticate using ambient environment: %s", err.Error())
		}
		return token, nil
	}

	fmt.Println("Loading slack token from vault...")
	grr, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	result, err := client.GetSecret(grr, vaultURL, "slack-token", "")
	if err != nil {
		defer cancel()
		return "", fmt.Errorf("could not load slack token from vault: %s", err)
	}

	return *result.Value, nil
}

func getKeyVaultClient() (keyvault.BaseClient, error) {
	authorizer, err := getAzureAuth()
	if err != nil {
		return keyvault.BaseClient{}, err
	}

	client := keyvault.New()
	client.Authorizer = authorizer

	return client, nil
}

func getAzureAuth() (autorest.Authorizer, error) {
	fmt.Println("Loading azure auth from magic...")
	a, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return nil, errors.Wrap(err, "error loading azure auth from environment variables")
	}

	return a, nil
}

type Storage struct {
	Account    string
	credential azblob.Credential
	pipeline   pipeline.Pipeline
}

func NewStorageClient() (Storage, error) {
	// See https://docs.microsoft.com/en-us/azure/storage/common/storage-auth-aad-app
	msiEndpoint, _ := adal.GetMSIEndpoint()
	resource := "https://slackoverload.blob.core.windows.net"
	spToken, err := adal.NewServicePrincipalTokenFromMSI(msiEndpoint, resource)
	if err != nil {
		return Storage{}, errors.Wrap(err, "error building azure token request")
	}
	err = spToken.EnsureFresh()
	if err != nil {
		return Storage{}, errors.Wrap(err, "error requesting azure token")
	}

	token := spToken.OAuthToken()
	s := Storage{Account: "slackoverload"}
	s.credential = azblob.NewTokenCredential(token, nil)
	s.pipeline = azblob.NewPipeline(s.credential, azblob.PipelineOptions{})

	return s, nil
}

func (s *Storage) getBlob(containerName string, blobName string) ([]byte, error) {
	containerURL, err := s.buildContainerURL(containerName)
	if err != nil {
		return nil, err
	}

	blobURL := containerURL.NewBlobURL(blobName)

	resp, err := blobURL.Download(context.Background(), 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	if err != nil {
		return nil, errors.Wrapf(err, "error initiating download of blob at %s", blobURL.String())
	}

	bodyStream := resp.Body(azblob.RetryReaderOptions{MaxRetryRequests: 20})
	buff := bytes.Buffer{}
	_, err = buff.ReadFrom(bodyStream)

	return buff.Bytes(), errors.Wrapf(err, "error reading blob body at %s", blobURL.String())
}

func (s *Storage) setBlob(containerName string, blobName string, data []byte) error {
	container, err := s.buildContainerURL(containerName)
	if err != nil {
		return err
	}

	blob := container.NewBlockBlobURL(blobName)
	opts := azblob.UploadToBlockBlobOptions{BlockSize: 64 * 1024}

	_, err = azblob.UploadBufferToBlockBlob(context.Background(), data, blob, opts)
	return errors.Wrapf(err, "error saving %s/%s", containerName, blobName)
}

func (s *Storage) buildContainerURL(containerName string) (azblob.ContainerURL, error) {
	rawURL := fmt.Sprintf("https://%s.blob.core.windows.net/%s", s.Account, containerName)
	URL, err := url.Parse(rawURL)
	if err != nil {
		return azblob.ContainerURL{}, errors.Wrapf(err, "could not parse container URL %s", rawURL)
	}

	return azblob.NewContainerURL(*URL, s.pipeline), nil
}
