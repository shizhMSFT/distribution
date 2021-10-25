package orasv2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/storage"
	"github.com/opencontainers/go-digest"
)

// artifactManifestHandler is a ManifestHandler that covers ORAS Artifacts.
type artifactManifestHandler struct {
	repository distribution.Repository
	blobStore  distribution.BlobStore
	linkFunc   storage.LinkFunc
}

func (amh *artifactManifestHandler) CanUnmarshal(content []byte) bool {
	var v json.RawMessage
	return json.Unmarshal(content, &v) == nil
}

func (amh *artifactManifestHandler) Unmarshal(ctx context.Context, dgst digest.Digest, content []byte) (distribution.Manifest, error) {
	dcontext.GetLogger(ctx).Debug("(*artifactManifestHandler).Unmarshal")

	if !amh.CanUnmarshal(content) {
		return nil, distribution.ErrManifestFormatNotSupported
	}

	dm := &DeserializedManifest{}
	if err := dm.UnmarshalJSON(content); err != nil {
		return nil, err
	}

	return dm, nil
}

func (ah *artifactManifestHandler) CanPut(man distribution.Manifest) bool {
	_, ok := man.(*DeserializedManifest)
	return ok
}

func (ah *artifactManifestHandler) Put(ctx context.Context, man distribution.Manifest, skipDependencyVerification bool) (digest.Digest, error) {
	dcontext.GetLogger(ctx).Debug("(*artifactManifestHandler).Put")

	if !ah.CanPut(man) {
		return "", distribution.ErrManifestFormatNotSupported
	}

	da, ok := man.(*DeserializedManifest)
	if !ok {
		return "", fmt.Errorf("wrong type put to artifactManifestHandler: %T", man)
	}

	if err := ah.verifyManifest(ctx, *da, skipDependencyVerification); err != nil {
		return "", err
	}

	mt, payload, err := da.Payload()
	if err != nil {
		return "", err
	}

	revision, err := ah.blobStore.Put(ctx, mt, payload)
	if err != nil {
		dcontext.GetLogger(ctx).Errorf("error putting payload into blobstore: %v", err)
		return "", err
	}

	err = ah.indexReferrers(ctx, *da, revision.Digest)
	if err != nil {
		dcontext.GetLogger(ctx).Errorf("error indexing referrers: %v", err)
		return "", err
	}

	return revision.Digest, nil
}

// verifyManifest ensures that the manifest content is valid from the
// perspective of the registry. As a policy, the registry only tries to
// store valid content, leaving trust policies of that content up to
// consumers.
func (amh *artifactManifestHandler) verifyManifest(ctx context.Context, dm DeserializedManifest, skipDependencyVerification bool) error {
	var errs distribution.ErrManifestVerification

	if dm.ArtifactType() == "" {
		errs = append(errs, distribution.ErrManifestVerification{errors.New("artifactType invalid")})
	}

	if !skipDependencyVerification {
		bs := amh.repository.Blobs(ctx)

		// All references must exist.
		for _, blobDesc := range dm.References() {
			desc, err := bs.Stat(ctx, blobDesc.Digest)
			if err != nil && err != distribution.ErrBlobUnknown {
				errs = append(errs, err)
			}
			if err != nil || desc.Digest == "" {
				// On error here, we always append unknown blob errors.
				errs = append(errs, distribution.ErrManifestBlobUnknown{Digest: blobDesc.Digest})
			}
		}

		ms, err := amh.repository.Manifests(ctx)
		if err != nil {
			return err
		}

		// Validate subject manifest.
		subject := dm.Subject()
		exists, err := ms.Exists(ctx, subject.Digest)
		if !exists || err == distribution.ErrBlobUnknown {
			errs = append(errs, distribution.ErrManifestBlobUnknown{Digest: subject.Digest})
		} else if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}

// indexReferrers indexes the subject of the given revision in its referrers index store.
func (amh *artifactManifestHandler) indexReferrers(ctx context.Context, dm DeserializedManifest, revision digest.Digest) error {
	// We can use artifact type in the link path to support filtering by artifact type
	//  but need to consider the max path length in different os
	//artifactType := dm.ArtifactType()
	subjectRevision := dm.Subject().Digest

	rootPath := path.Join(referrersLinkPath(amh.repository.Named().Name()), subjectRevision.Algorithm().String(), subjectRevision.Hex())
	referenceLinkPath := path.Join(rootPath, revision.Algorithm().String(), revision.Hex(), "link")
	if err := amh.linkFunc(ctx, referenceLinkPath, revision); err != nil {
		return err
	}

	return nil
}

func referrersLinkPath(name string) string {
	return path.Join("/docker/registry/", "v2", "repositories", name, "_refs", "subjects")
}

func canonicalArtifactType(artifactType string) string {
	return strings.ReplaceAll(artifactType, "/", ".")
}
