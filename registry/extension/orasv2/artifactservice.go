package orasv2

import (
	"context"
	"path"

	"github.com/distribution/distribution/v3"
	dcontext "github.com/distribution/distribution/v3/context"
	"github.com/distribution/distribution/v3/registry/extension"
	"github.com/distribution/distribution/v3/registry/storage"
	"github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/opencontainers/go-digest"
	artifactv1 "github.com/oras-project/artifacts-spec/specs-go/v1"
)

type ArtifactService interface {
	Referrers(ctx context.Context, revision digest.Digest, referrerType string) ([]artifactv1.Descriptor, error)
}

// referrersHandler handles http operations on manifest referrers.
type referrersHandler struct {
	distribution.Namespace
	extContext         *extension.Context
	linkBlobEnumerator storage.LinkBlobEnumerate

	// Digest is the target manifest's digest.
	Digest digest.Digest
}

func (h *referrersHandler) Referrers(ctx context.Context, revision digest.Digest, referrerType string) ([]artifactv1.Descriptor, error) {
	dcontext.GetLogger(ctx).Debug("(*manifestStore).Referrers")

	var referrers []artifactv1.Descriptor

	repo := h.extContext.Repository
	manifests, err := repo.Manifests(ctx)
	if err != nil {
		return nil, err
	}

	blobStatter := h.Namespace.BlobStatter()

	rootPath := path.Join(referrersLinkPath(repo.Named().Name()), revision.Algorithm().String(), revision.Hex())
	err = h.linkBlobEnumerator(ctx, rootPath, func(referrerRevision digest.Digest) error {
		man, err := manifests.Get(ctx, referrerRevision)
		if err != nil {
			return err
		}

		ArtifactMan, ok := man.(*DeserializedManifest)
		if !ok {
			// The PUT handler would guard against this situation. Skip this manifest.
			return nil
		}

		desc, err := blobStatter.Stat(ctx, referrerRevision)
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
