package oras

import (
	"context"

	"github.com/distribution/distribution/v3"
	"github.com/opencontainers/go-digest"
)

type artifactManifestHandler struct {
	repository distribution.Repository
	blobStore  distribution.BlobStore
}

func (amh *artifactManifestHandler) Unmarshal(ctx context.Context, dgst digest.Digest, content []byte) (distribution.Manifest, error) {
	// return deserialized manifest
	return nil, nil
}

func (ah *artifactManifestHandler) Put(ctx context.Context, man distribution.Manifest, skipDependencyVerification bool) (digest.Digest, error) {
	// process the manifest
	return digest.FromString("manifest from blobstore"), nil
}
