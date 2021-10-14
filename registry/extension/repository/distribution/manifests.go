package distribution

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/registry/api/errcode"
	v2 "github.com/distribution/distribution/v3/registry/api/v2"
	"github.com/distribution/distribution/v3/registry/extension"
	"github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/gorilla/handlers"
	"github.com/opencontainers/go-digest"
)

func manifestDispatcher(ctx *extension.Context, r *http.Request) http.Handler {
	manifestHandler := &manifestHandler{
		Context: ctx,
	}

	return handlers.MethodHandler{
		"GET": http.HandlerFunc(manifestHandler.GetManifestDigests),
	}
}

// manifestHandler handles requests for manifests under a manifest name.
type manifestHandler struct {
	*extension.Context
}

type manifestAPIResponse struct {
	Name    string          `json:"name"`
	Tag     string          `json:"tag"`
	Digests []digest.Digest `json:"digests"`
}

func (mh *manifestHandler) GetManifestDigests(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	q := r.URL.Query()
	tag := q.Get("tag")
	if tag == "" {
		mh.Errors = append(mh.Errors, v2.ErrorCodeTagInvalid.WithDetail(tag))
		return
	}

	tags, ok := toTagManifestsProvider(mh.Repository.Tags(mh.Context))
	if !ok {
		mh.Errors = append(mh.Errors, errcode.ErrorCodeUnsupported.WithDetail(nil))
		return
	}

	digests, err := tags.ManifestDigests(mh.Context, tag)
	if err != nil {
		switch err := err.(type) {
		case driver.PathNotFoundError:
			mh.Errors = append(mh.Errors, v2.ErrorCodeManifestUnknown.WithDetail(map[string]string{"tag": tag}))
		default:
			mh.Errors = append(mh.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(w)
	if err := enc.Encode(manifestAPIResponse{
		Name:    mh.Repository.Named().Name(),
		Tag:     tag,
		Digests: digests,
	}); err != nil {
		mh.Errors = append(mh.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}

func decomposeStruct(in interface{}) (out interface{}, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			out = nil
			ok = false
		}
	}()
	out = reflect.ValueOf(in).Elem().Field(0).Interface()
	ok = true
	return
}

func toTagManifestsProvider(i interface{}) (distribution.TagManifestsProvider, bool) {
	tags, ok := i.(distribution.TagManifestsProvider)
	if ok {
		return tags, true
	}
	i, ok = decomposeStruct(i)
	if !ok {
		return nil, false
	}
	tags, ok = i.(distribution.TagManifestsProvider)
	return tags, ok
}
