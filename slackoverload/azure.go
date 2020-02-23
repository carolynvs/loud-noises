package slackoverload

import (
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"
)

func getAzureAuth(resource string) (autorest.Authorizer, error) {
	a, err := auth.NewAuthorizerFromEnvironmentWithResource(resource)
	if err != nil {
		return nil, errors.Wrap(err, "error loading azure auth from environment variables")
	}

	return a, nil
}
