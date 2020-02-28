package slackoverload

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/pkg/errors"
)

const vaultURL = "https://slackoverload.vault.azure.net"

type Secrets struct {
	Client keyvault.BaseClient
}

func NewSecretsClient() (Secrets, error) {
	authorizer, err := getAzureAuth("https://vault.azure.net")
	if err != nil {
		return Secrets{}, err
	}

	client := keyvault.New()
	client.Authorizer = authorizer

	return Secrets{Client: client}, nil
}

func (s *Secrets) GetSlackSigningSecret() (string, error) {
	value, _, err := s.GetSecret("slack-signing-secret")
	return value, err
}

func (s *Secrets) GetSessionKey() (string, error) {
	value, _, err := s.GetSecret("session-key")
	return value, err
}

func (s *Secrets) GetSlackClientId() (string, error) {
	value, _, err := s.GetSecret("slack-client-id")
	return value, err
}

func (s *Secrets) GetSlackClientSecret() (string, error) {
	value, _, err := s.GetSecret("slack-client-secret")
	return value, err
}

func (s *Secrets) GetSecret(key string) (string, map[string]*string, error) {
	cxt, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	result, err := s.Client.GetSecret(cxt, vaultURL, key, "")
	if err != nil {
		defer cancel()
		return "", nil, errors.Wrapf(err, "could not load secret %q from vault", key)
	}

	return *result.Value, result.Tags, nil
}

func (s *Secrets) SetSecret(key string, value string, tags map[string]*string) error {
	// Timebox getting the secret because a bad client or auth will hang forever
	cxt, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	_, err := s.Client.SetSecret(cxt, vaultURL, key, keyvault.SecretSetParameters{
		Value: &value,
		Tags:  tags,
	})
	if err != nil {
		defer cancel()
		return errors.Wrapf(err, "error saving secret %s", key)
	}

	return nil
}
