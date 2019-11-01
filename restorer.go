package lifecycle

import (
	"os"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/buildpack/lifecycle/archive"
)

type Restorer struct {
	LayersDir  string
	Buildpacks []Buildpack
	Logger     Logger
	UID        int
	GID        int
}

// Restore attempts to restore layer data for cache=true layers and when unsuccessful removes the layer.
func (r *Restorer) Restore(cache Cache) error {
	meta, err := cache.RetrieveMetadata()
	if err != nil {
		return errors.Wrapf(err, "retrieving cache metadata")
	}

	var g errgroup.Group
	for _, buildpack := range r.Buildpacks {
		buildpackDir, err := readBuildpackLayersDir(r.LayersDir, buildpack)
		if err != nil {
			return errors.Wrapf(err, "reading layers directory")
		}

		cachedLayers := meta.MetadataForBuildpack(buildpack.ID).Layers
		for _, bpLayer := range buildpackDir.findLayers(cached) {
			name := bpLayer.name()
			cachedLayer, exists := cachedLayers[name]
			if !exists {
				r.Logger.Infof("Removing %q, not in cache", bpLayer.Identifier())
				if err := bpLayer.remove(); err != nil {
					return errors.Wrapf(err, "removing layer")
				}
				continue
			}
			data, err := bpLayer.read()
			if err != nil {
				return errors.Wrapf(err, "reading layer")
			}
			if data.SHA != cachedLayer.SHA {
				r.Logger.Infof("Removing %q, wrong sha", bpLayer.Identifier())
				r.Logger.Debugf("Layer sha: %q, cache sha: %q", data.SHA, cachedLayer.SHA)
				if err := bpLayer.remove(); err != nil {
					return errors.Wrapf(err, "removing layer")
				}
			} else {
				r.Logger.Infof("Restoring data for %q from cache", bpLayer.Identifier())
				g.Go(func() error {
					return r.restoreLayer(cache, cachedLayer.SHA)
				})
			}
		}
	}
	if err := g.Wait(); err != nil {
		return errors.Wrap(err, "restoring data")
	}

	// if restorer is running as root it needs to fix the ownership of the layers dir
	if current := os.Getuid(); current == -1 {
		return errors.New("cannot determine UID")
	} else if current == 0 {
		if err := recursiveChown(r.LayersDir, r.UID, r.GID); err != nil {
			return errors.Wrapf(err, "chowning layers dir to '%d/%d'", r.UID, r.GID)
		}
	}
	return nil
}

func (r *Restorer) restoreLayer(cache Cache, sha string) error {
	r.Logger.Debugf("Retrieving data for %q", sha)
	rc, err := cache.RetrieveLayer(sha)
	if err != nil {
		return err
	}
	defer rc.Close()

	return archive.Untar(rc, "/")
}
