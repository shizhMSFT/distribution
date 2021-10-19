package oras

import (
	"context"
	"net/http"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/reference"
	"github.com/distribution/distribution/v3/registry/extension"
	"github.com/distribution/distribution/v3/registry/storage"
	"github.com/opencontainers/go-digest"
)

type orasNamespace struct {
	distribution.Namespace
	routes           []extension.Route
	storageEnabled   bool
	referrersEnabled bool
}

type ArtifactRepository interface {
	distribution.Repository
	Referrers(ctx context.Context, digest digest.Digest, contnuationToken string) (descs []distribution.Descriptor, nextLink string, err error)
}

type artifactRepository struct {
	distribution.Repository
	storageEnabled   bool
	referrersEnabled bool
}

func newOrasNamespace(ctx context.Context, options map[string]interface{}, base distribution.Namespace) (extension.ExtendedNamespace, error) {
	_, ok := options["artifacts"]
	if !ok {
		return nil, nil
	}

	storageEnabled := false
	storageComponent, ok := options["artifacts.storageEnabled"]
	if ok {
		flag, ok := storageComponent.(bool)
		if ok {
			storageEnabled = flag
		}
	}

	// config pertaining to component can be simple bool or its own map
	referrersEnabled := false
	referrerComponent, ok := options["artifacts.referrersEnabled"]
	if ok {
		flag, ok := referrerComponent.(bool)
		if ok {
			referrersEnabled = flag
		}
	}

	return &orasNamespace{
		Namespace:        base,
		referrersEnabled: referrersEnabled,
		storageEnabled:   storageEnabled,
	}, nil
}

func (o *orasNamespace) GetRoutes() []extension.Route {
	// configure the root and repository level roots
	var routes []extension.Route

	// based on the component,enable route either at root or repository level
	if o.referrersEnabled {
		routes = []extension.Route{
			{
				Namespace:  "oras",
				Extension:  "artifacts",
				Component:  "referrers",
				Dispatcher: o.referrersDispatcher,
			},
		}
	}

	return routes
}

func init() {
	extension.Register("distribution", newOrasNamespace)
}

func (o *orasNamespace) referrersDispatcher(ctx *extension.Context, r *http.Request) http.Handler {
	// parse the request and return the appropriate handler
	return nil
}

func (ar *artifactRepository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {

	if ar.referrersEnabled || ar.storageEnabled {
		artifactManifestHandler := artifactManifestHandler{
			repository: ar,
			blobStore:  ar.Blobs(ctx),
			// other parameters required to implement referrers
		}

		options = append(options, storage.AddManifestHanlder("artifact.v1", &artifactManifestHandler))
	}

	return ar.Repository.Manifests(ctx, options...)
}

func (ar *artifactRepository) Referrers(ctx context.Context, digest digest.Digest, contnuationToken string) (descs []distribution.Descriptor, nextLink string, err error) {
	// query the linked blob store for the referrers using ar.Blobs
	// if that is not sufficient enrich ar.Blobs with custom blob store
	return []distribution.Descriptor{}, "", nil
}

func (o *orasNamespace) Repository(ctx context.Context, name reference.Named) (distribution.Repository, error) {
	repo, err := o.Repository(ctx, name)

	if err != nil {
		return nil, err
	}

	return &artifactRepository{Repository: repo, storageEnabled: o.storageEnabled, referrersEnabled: o.referrersEnabled}, nil
}
