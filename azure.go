package main

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"
)

const vaultURL = "https://slackoverload.vault.azure.net"

func getKeyVaultClient() (keyvault.BaseClient, error) {
	authorizer, err := getAzureAuth("https://vault.azure.net")
	if err != nil {
		return keyvault.BaseClient{}, err
	}

	client := keyvault.New()
	client.Authorizer = authorizer

	return client, nil
}

func getAzureAuth(resource string) (autorest.Authorizer, error) {
	fmt.Println("Loading azure auth from magic...")
	a, err := auth.NewAuthorizerFromEnvironmentWithResource(resource)
	if err != nil {
		return nil, errors.Wrap(err, "error loading azure auth from environment variables")
	}

	return a, nil
}
