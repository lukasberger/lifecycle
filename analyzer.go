package lifecycle

import (
	"fmt"
	"os"

	"github.com/buildpack/imgutil"
	"github.com/pkg/errors"

	"github.com/buildpack/lifecycle/metadata"
)

type Analyzer struct {
	AnalyzedPath string
	AppDir       string
	Buildpacks   []Buildpack
	GID, UID     int
	LayersDir    string
	Logger       Logger
	SkipLayers   bool
}

// Analyze restores metadata for launch and cache layers into the layers directory.
func (a *Analyzer) Analyze(image imgutil.Image, cache Cache) (*metadata.AnalyzedMetadata, error) {
	imageID, err := a.getImageIdentifier(image)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving image identifier")
	}

	appMeta, err := metadata.GetLayersMetadata(image)
	if err != nil {
		return nil, errors.Wrap(err, "getting app image metadata")
	}

	if a.SkipLayers {
		a.Logger.Infof("Skipping buildpack layer analysis")
		return &metadata.AnalyzedMetadata{
			Image:    imageID,
			Metadata: appMeta,
		}, nil
	}

	cacheMeta, err := cache.RetrieveMetadata()
	if err != nil {
		return nil, errors.Wrap(err, "retrieving cache metadata")
	}

	for _, buildpack := range a.Buildpacks {
		buildpackDir, err := readBuildpackLayersDir(a.LayersDir, buildpack)
		if err != nil {
			return nil, errors.Wrap(err, "reading buildpack layer directory")
		}

		// Restore metadata for launch=true layers.
		// The restorer step will restore the layer data for cache=true layers if possible or delete the layer.
		appLayers := appMeta.MetadataForBuildpack(buildpack.ID).Layers
		for name, layer := range appLayers {
			identifier := fmt.Sprintf("%s:%s", buildpack.ID, name)
			if !layer.Launch {
				a.Logger.Infof("Not restoring metadata for %q, marked as launch=false", identifier)
				continue
			}
			if layer.Build && !layer.Cache {
				a.Logger.Infof("Not restoring metadata for %q, marked as build=true, cache=false", identifier)
				continue
			}
			a.Logger.Infof("Restoring metadata for %q from app image", identifier)
			if err := a.writeLayerMetadata(buildpackDir, name, layer); err != nil {
				return nil, err
			}
		}

		// Restore metadata for cache=true layers.
		// The restorer step will restore the layer data if possible or delete the layer.
		cachedLayers := cacheMeta.MetadataForBuildpack(buildpack.ID).Layers
		for name, layer := range cachedLayers {
			identifier := fmt.Sprintf("%s:%s", buildpack.ID, name)
			if !layer.Cache {
				a.Logger.Debugf("Not restoring %q from cache, marked as cache=false", identifier)
				continue
			}
			// If launch=true, the metadata was restored from the app image or the layer is stale.
			if layer.Launch {
				a.Logger.Debugf("Not restoring %q from cache, marked as launch=true", identifier)
				continue
			}
			a.Logger.Infof("Restoring metadata for %q from cache", identifier)
			if err := a.writeLayerMetadata(buildpackDir, name, layer); err != nil {
				return nil, err
			}
		}
	}

	// if analyzer is running as root it needs to fix the ownership of the layers dir
	if current := os.Getuid(); current == 0 {
		if err := recursiveChown(a.LayersDir, a.UID, a.GID); err != nil {
			return nil, errors.Wrapf(err, "chowning layers dir to '%d/%d'", a.UID, a.GID)
		}
	}

	return &metadata.AnalyzedMetadata{
		Image:    imageID,
		Metadata: appMeta,
	}, nil
}

func (a *Analyzer) getImageIdentifier(image imgutil.Image) (*metadata.ImageIdentifier, error) {
	if !image.Found() {
		a.Logger.Warnf("Image %q not found", image.Name())
		return nil, nil
	}
	identifier, err := image.Identifier()
	if err != nil {
		return nil, err
	}
	a.Logger.Debugf("Analyzing image %q", identifier.String())
	return &metadata.ImageIdentifier{
		Reference: identifier.String(),
	}, nil
}

func (a *Analyzer) writeLayerMetadata(buildpackDir bpLayersDir, name string, metadata metadata.BuildpackLayerMetadata) error {
	layer := buildpackDir.newBPLayer(name)
	a.Logger.Debugf("Writing layer metadata for %q", layer.Identifier())
	if err := layer.writeMetadata(metadata); err != nil {
		return err
	}
	return layer.writeSha(metadata.SHA)
}
