package oras

import (
	"context"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/distribution/distribution/v3/registry/storage/extension/repository/oras/artifact"
	"github.com/opencontainers/go-digest"
	artifactv1 "github.com/oras-project/artifacts-spec/specs-go/v1"
)

type ArtifactService interface {
	Referrers(ctx context.Context, revision digest.Digest, referrerType string) ([]artifactv1.Descriptor, error)
}

type artifactService struct {
	repository     distribution.Repository
	blobStore      distribution.BlobStore
	referrersStore referrersStoreFunc
}

func (as *artifactService) Referrers(ctx context.Context, revision digest.Digest, referrerType string) ([]artifactv1.Descriptor, error) {
	dcontext.GetLogger(ctx).Debug("(*manifestStore).Referrers")

	var referrers []artifactv1.Descriptor

	manifests, err := as.repository.Manifests(ctx)
	if err != nil {
		return nil, err
	}

	store, err := as.referrersStore(ctx, revision, referrerType)
	if err != nil {
		return nil, err
	}
	err = store.Enumerate(ctx, func(referrerRevision digest.Digest) error {
		man, err := manifests.Get(ctx, referrerRevision)
		if err != nil {
			return err
		}

		ArtifactMan, ok := man.(*artifact.DeserializedManifest)
		if !ok {
			// The PUT handler would guard against this situation. Skip this manifest.
			return nil
		}

		desc, err := as.blobStore.Stat(ctx, referrerRevision)
		if err != nil {
			return err
		}
		desc.MediaType, _, _ = man.Payload()
		referrers = append(referrers, artifactv1.Descriptor{
			MediaType:    desc.MediaType,
			Size:         desc.Size,
			Digest:       desc.Digest,
			ArtifactType: ArtifactMan.ArtifactType(),
		})
		return nil
	})

	if err != nil {
		switch err.(type) {
		case driver.PathNotFoundError:
			return nil, nil
		}
		return nil, err
	}

	return referrers, nil
}
