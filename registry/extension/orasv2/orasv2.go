package orasv2

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/api/errcode"
	v2 "github.com/distribution/distribution/v3/registry/api/v2"
	"github.com/distribution/distribution/v3/registry/extension"
	"github.com/distribution/distribution/v3/registry/storage"
	"github.com/gorilla/handlers"
	"github.com/opencontainers/go-digest"
	orasartifacts "github.com/oras-project/artifacts-spec/specs-go/v1"
	"gopkg.in/yaml.v2"
)

const Name = "orasv2"

type orasNamespace struct {
	referrersEnabled bool
	distribution.Namespace
	linkBlobEnumerator storage.LinkBlobEnumerate
}

type OrasOptions struct {
	ArtifactsExtComponents []string `yaml:"artifacts,omitempty"`
}

func newOrasNamespace(ctx context.Context, options map[string]interface{}) (extension.ExtendedNamespace, error) {
	optionsYaml, err := yaml.Marshal(options)
	if err != nil {
		return nil, err
	}

	var orasOptions OrasOptions
	err = yaml.Unmarshal(optionsYaml, &orasOptions)
	if err != nil {
		return nil, err
	}

	referrersEnabled := false
	for _, component := range orasOptions.ArtifactsExtComponents {
		if component == "referrers" {
			referrersEnabled = true
			break
		}
	}

	return &orasNamespace{
		referrersEnabled: referrersEnabled,
	}, nil
}

func init() {
	extension.Register(Name, newOrasNamespace)
}

func (o *orasNamespace) GetManifestHandlers(repo distribution.Repository, blobStore distribution.BlobStore, linkFunc storage.LinkFunc) []storage.ManifestHandler {
	if o.referrersEnabled {
		return []storage.ManifestHandler{
			&artifactManifestHandler{
				repository: repo,
				blobStore:  blobStore,
				linkFunc:   linkFunc,
			}}
	}

	return []storage.ManifestHandler{}
}

func (o *orasNamespace) UseLinkBlobEnumerator(enumerator storage.LinkBlobEnumerate) {
	o.linkBlobEnumerator = enumerator
}

func (o *orasNamespace) GetRepositoryRoutes(base distribution.Namespace) []extension.ExtendedRoute {
	o.Namespace = base
	if o.referrersEnabled {
		return []extension.ExtendedRoute{
			{
				Namespace:  Name,
				Extension:  "artifacts",
				Component:  "referrers",
				Dispatcher: o.referrersDispatcher,
			},
		}
	}
	return []extension.ExtendedRoute{}
}

func (o *orasNamespace) GetRegistryRoutes(base distribution.Namespace) []extension.ExtendedRoute {
	return nil
}

// referrersResponse describes the response body of the referrers API.
type referrersResponse struct {
	Referrers []orasartifacts.Descriptor `json:"references"`
}

func (o *orasNamespace) referrersDispatcher(extCtx *extension.Context, r *http.Request) http.Handler {

	handler := &referrersHandler{
		Namespace:          o.Namespace,
		linkBlobEnumerator: o.linkBlobEnumerator,
		extContext:         extCtx,
	}
	q := r.URL.Query()
	if dgstStr := q.Get("digest"); dgstStr == "" {
		dcontext.GetLogger(extCtx).Errorf("digest not available")
	} else if d, err := digest.Parse(dgstStr); err != nil {
		dcontext.GetLogger(extCtx).Errorf("error parsing digest=%q: %v", dgstStr, err)
	} else {
		handler.Digest = d
	}

	mhandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(handler.Get),
	}

	return mhandler
}

func (h *referrersHandler) Get(w http.ResponseWriter, r *http.Request) {
	dcontext.GetLogger(h.extContext).Debug("Get")

	// This can be empty
	artifactType := r.FormValue("artifactType")

	if h.Digest == "" {
		h.extContext.Errors = append(h.extContext.Errors, v2.ErrorCodeManifestUnknown.WithDetail("digest not specified"))
		return
	}

	referrers, err := h.Referrers(h.extContext, h.Digest, artifactType)
	if err != nil {
		if _, ok := err.(distribution.ErrManifestUnknownRevision); ok {
			h.extContext.Errors = append(h.extContext.Errors, v2.ErrorCodeManifestUnknown.WithDetail(err))
		} else {
			h.extContext.Errors = append(h.extContext.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		}
		return
	}

	if referrers == nil {
		referrers = []orasartifacts.Descriptor{}
	}

	response := referrersResponse{
		Referrers: referrers,
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err = enc.Encode(response); err != nil {
		h.extContext.Errors = append(h.extContext.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}
