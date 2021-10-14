package oras

import (
	"context"

	extension "github.com/distribution/distribution/v3/registry/extension"
	repositoryextension "github.com/distribution/distribution/v3/registry/extension/repository"
)

func newOrasExtension(ctx context.Context, options map[string]interface{}) ([]extension.Route, error) {
	return []extension.Route{
		{
			Namespace:  "oras",
			Extension:  "artifacts",
			Component:  "referrers",
			Dispatcher: referrersDispatcher,
		},
	}, nil
}

func init() {
	repositoryextension.Register("oras", newOrasExtension)
}
