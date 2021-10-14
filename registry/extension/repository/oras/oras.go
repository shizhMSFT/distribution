package oras

import (
	"encoding/json"
	"net/http"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/api/errcode"
	v2 "github.com/distribution/distribution/v3/registry/api/v2"
	extension "github.com/distribution/distribution/v3/registry/extension"
	"github.com/distribution/distribution/v3/registry/storage/extension/repository/oras"
	"github.com/gorilla/handlers"
	"github.com/opencontainers/go-digest"
	orasartifacts "github.com/oras-project/artifacts-spec/specs-go/v1"
)

// referrersDispatcher takes the request context and builds the
// appropriate handler for handling manifest referrer requests.
func referrersDispatcher(ctx *extension.Context, r *http.Request) http.Handler {
	handler := &referrersHandler{
		Context: ctx,
	}

	q := r.URL.Query()
	if dgstStr := q.Get("digest"); dgstStr == "" {
		dcontext.GetLogger(ctx).Errorf("digest not available")
	} else if d, err := digest.Parse(dgstStr); err != nil {
		dcontext.GetLogger(ctx).Errorf("error parsing digest=%q: %v", dgstStr, err)
	} else {
		handler.Digest = d
	}

	mhandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(handler.Get),
	}

	return mhandler
}

// referrersResponse describes the response body of the referrers API.
type referrersResponse struct {
	Referrers []orasartifacts.Descriptor `json:"references"`
}

// referrersHandler handles http operations on manifest referrers.
type referrersHandler struct {
	*extension.Context

	// Digest is the target manifest's digest.
	Digest digest.Digest
}

// Get gets the list of artifacts that reference the given manifest filtered by the artifact type
// specified in the request.
func (h *referrersHandler) Get(w http.ResponseWriter, r *http.Request) {
	dcontext.GetLogger(h).Debug("Get")

	// This can be empty
	artifactType := r.FormValue("artifactType")

	if h.Digest == "" {
		h.Errors = append(h.Errors, v2.ErrorCodeManifestUnknown.WithDetail("digest not specified"))
		return
	}

	es, err := h.Repository.Extensions(h)
	if err != nil {
		if err == distribution.ErrUnsupported {
			h.Errors = append(h.Errors, errcode.ErrorCodeUnsupported.WithDetail("backend does not support extension"))
		} else {
			h.Errors = append(h.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		}
		return
	}

	asi, err := es.Get(h, "oras/artifacts")
	if err != nil {
		h.Errors = append(h.Errors, errcode.ErrorCodeUnsupported.WithDetail(err))
		return
	}
	as, ok := asi.(oras.ArtifactService)
	if !ok {
		h.Errors = append(h.Errors, errcode.ErrorCodeUnknown.WithDetail("invalid ORAS artifact service"))
		return
	}

	referrers, err := as.Referrers(h.Context, h.Digest, artifactType)
	if err != nil {
		if _, ok := err.(distribution.ErrManifestUnknownRevision); ok {
			h.Errors = append(h.Errors, v2.ErrorCodeManifestUnknown.WithDetail(err))
		} else {
			h.Errors = append(h.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
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
		h.Errors = append(h.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}
