package oras

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	repositoryextension "github.com/distribution/distribution/v3/registry/storage/extension/repository"
	"github.com/distribution/distribution/v3/registry/storage/extension/repository/oras/artifact"
	"github.com/opencontainers/go-digest"
)

// referrersStoreFunc describes a function that returns the referrers store
// for the given manifest of the given artifactType.
// A referrers store provides links to referrer manifests.
type referrersStoreFunc func(ctx context.Context, revision digest.Digest, artifactType string) (repositoryextension.LinkedBlobStore, error)

// artifactManifestHandler is a ManifestHandler that covers ORAS Artifacts.
type artifactManifestHandler struct {
	repository     distribution.Repository
	blobStore      distribution.BlobStore
	referrersStore referrersStoreFunc
}

func (amh *artifactManifestHandler) CanUnmarshal(content []byte) bool {
	var v json.RawMessage
	return json.Unmarshal(content, &v) == nil
}

func (amh *artifactManifestHandler) Unmarshal(ctx context.Context, dgst digest.Digest, content []byte) (distribution.Manifest, error) {
	dcontext.GetLogger(ctx).Debug("(*artifactManifestHandler).Unmarshal")

	dm := &artifact.DeserializedManifest{}
	if err := dm.UnmarshalJSON(content); err != nil {
		return nil, err
	}

	return dm, nil
}

func (ah *artifactManifestHandler) CanPut(man distribution.Manifest) bool {
	_, ok := man.(*artifact.DeserializedManifest)
	return ok
}

func (ah *artifactManifestHandler) Put(ctx context.Context, man distribution.Manifest, skipDependencyVerification bool) (digest.Digest, error) {
	dcontext.GetLogger(ctx).Debug("(*artifactManifestHandler).Put")

	da, ok := man.(*artifact.DeserializedManifest)
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
func (amh *artifactManifestHandler) verifyManifest(ctx context.Context, dm artifact.DeserializedManifest, skipDependencyVerification bool) error {
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
func (amh *artifactManifestHandler) indexReferrers(ctx context.Context, dm artifact.DeserializedManifest, revision digest.Digest) error {
	artifactType := dm.ArtifactType()
	subject := dm.Subject()

	store, err := amh.referrersStore(ctx, subject.Digest, artifactType)
	if err != nil {
		return err
	}
	if err := store.LinkBlob(ctx, distribution.Descriptor{Digest: revision}); err != nil {
		return err
	}

	return nil
}
