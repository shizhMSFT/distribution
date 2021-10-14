package oras

import (
	"context"
	"path"

	"github.com/distribution/distribution/v3"
	repositoryextension "github.com/distribution/distribution/v3/registry/storage/extension/repository"
	"github.com/opencontainers/go-digest"
)

type orasExtension struct{}

func newOrasExtension(ctx context.Context, options map[string]interface{}) (repositoryextension.RepositoryExtension, error) {
	return &orasExtension{}, nil
}

func (oe *orasExtension) Name() string {
	return "oras/artifacts"
}

func (oe *orasExtension) Components() []string {
	return []string{"referrers"}
}

func (oe *orasExtension) ManifestHandler(ctx context.Context, repo distribution.Repository, store repositoryextension.RepositoryStore) (repositoryextension.ManifestHandler, error) {
	blobStore, err := manifestStore(ctx, repo, store)
	if err != nil {
		return nil, err
	}

	return &artifactManifestHandler{
		repository:     repo,
		blobStore:      blobStore,
		referrersStore: referrersStoreFactory(repo, store),
	}, nil

}

func (oe *orasExtension) RepositoryExtension(ctx context.Context, repo distribution.Repository, store repositoryextension.RepositoryStore) (interface{}, error) {
	blobStore, err := manifestStore(ctx, repo, store)
	if err != nil {
		return nil, err
	}

	return &artifactService{
		repository:     repo,
		blobStore:      blobStore,
		referrersStore: referrersStoreFactory(repo, store),
	}, nil
}

func manifestStore(ctx context.Context, repo distribution.Repository, store repositoryextension.RepositoryStore) (distribution.BlobStore, error) {
	return store.LinkedBlobStore(ctx, repositoryextension.LinkedBlobStoreOptions{
		RootPath: manifestLinkPath(repo.Named().Name()),
		ResolvePath: func(name string, dgst digest.Digest) (string, error) {
			return path.Join(manifestLinkPath(name), dgst.Algorithm().String(), dgst.Hex(), "link"), nil
		},
		UseMiddleware: true,
	})
}

func referrersStoreFactory(repo distribution.Repository, store repositoryextension.RepositoryStore) referrersStoreFunc {
	return func(ctx context.Context, revision digest.Digest, artifactType string) (repositoryextension.LinkedBlobStore, error) {
		rootPathFn := func(name string) string {
			return path.Join(manifestLinkPath(name), revision.Algorithm().String(), revision.Hex(), "ref")
		}
		return store.LinkedBlobStore(ctx, repositoryextension.LinkedBlobStoreOptions{
			RootPath: rootPathFn(repo.Named().Name()),
			ResolvePath: func(name string, dgst digest.Digest) (string, error) {
				return path.Join(rootPathFn(name), dgst.Algorithm().String(), dgst.Hex(), "link"), nil
			},
		})
	}
}

func manifestLinkPath(name string) string {
	return path.Join("/docker/registry/", "v2", "repositories", name, "_manifests", "revisions")
}

func init() {
	repositoryextension.Register("oras", newOrasExtension)
}
