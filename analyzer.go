package lifecycle

import (
	"os"

	"github.com/buildpack/imgutil"
	"github.com/pkg/errors"

	"github.com/buildpack/lifecycle/logging"
	"github.com/buildpack/lifecycle/metadata"
)

type Analyzer struct {
	AnalyzedPath string
	AppDir       string
	Buildpacks   []Buildpack
	GID, UID     int
	LayersDir    string
	Logger       logging.Logger
	SkipLayers   bool
}

func (a *Analyzer) Analyze(image imgutil.Image) (metadata.AnalyzedMetadata, error) {
	imageID, err := a.getImageIdentifier(image)
	if err != nil {
		return metadata.AnalyzedMetadata{}, errors.Wrap(err, "retrieve image identifier")
	}

	data, err := metadata.GetLayersMetadata(image)
	if err != nil {
		return metadata.AnalyzedMetadata{}, err
	}

	if !a.SkipLayers {
		for _, buildpack := range a.Buildpacks {
			bpLayersDir, err := readBuildpackLayersDir(a.LayersDir, buildpack)
			if err != nil {
				return metadata.AnalyzedMetadata{}, err
			}

			metadataLayers := data.MetadataForBuildpack(buildpack.ID).Layers
			for _, cachedLayer := range bpLayersDir.layers {
				cacheType := cachedLayer.classifyCache(metadataLayers)
				switch cacheType {
				case cacheStaleNoMetadata:
					a.Logger.Infof("Removing stale cached launch layer '%s', not in metadata \n", cachedLayer.Identifier())
					if err := cachedLayer.remove(); err != nil {
						return metadata.AnalyzedMetadata{}, err
					}
				case cacheStaleWrongSHA:
					a.Logger.Infof("Removing stale cached launch layer '%s'", cachedLayer.Identifier())
					if err := cachedLayer.remove(); err != nil {
						return metadata.AnalyzedMetadata{}, err
					}
				case cacheMalformed:
					a.Logger.Infof("Removing malformed cached layer '%s'", cachedLayer.Identifier())
					if err := cachedLayer.remove(); err != nil {
						return metadata.AnalyzedMetadata{}, err
					}
				case cacheNotForLaunch:
					a.Logger.Infof("Using cached layer '%s'", cachedLayer.Identifier())
				case cacheValid:
					a.Logger.Infof("Using cached launch layer '%s'", cachedLayer.Identifier())
					a.Logger.Infof("Rewriting metadata for layer '%s'", cachedLayer.Identifier())
					if err := cachedLayer.writeMetadata(metadataLayers); err != nil {
						return metadata.AnalyzedMetadata{}, err
					}
				}
			}

			for lmd, data := range metadataLayers {
				if !data.Build && !data.Cache {
					layer := bpLayersDir.newBPLayer(lmd)
					a.Logger.Infof("Writing metadata for uncached layer '%s'", layer.Identifier())
					if err := layer.writeMetadata(metadataLayers); err != nil {
						return metadata.AnalyzedMetadata{}, err
					}
				}
			}
		}
	} else {
		a.Logger.Infof("Skipping buildpack layer analysis")
	}

	// if analyzer is running as root it needs to fix the ownership of the layers dir
	if current := os.Getuid(); current == 0 {
		if err := recursiveChown(a.LayersDir, a.UID, a.GID); err != nil {
			return metadata.AnalyzedMetadata{}, errors.Wrapf(err, "chowning layers dir to '%d/%d'", a.UID, a.GID)
		}
	}

	return metadata.AnalyzedMetadata{
		Image:    imageID,
		Metadata: data,
	}, nil
}

func (a *Analyzer) getImageIdentifier(image imgutil.Image) (*metadata.ImageIdentifier, error) {
	if !image.Found() {
		a.Logger.Warnf("Image '%s' not found", image.Name())
		return nil, nil
	}
	identifier, err := image.Identifier()
	if err != nil {
		return nil, err
	}
	a.Logger.Debugf("Analyzing image '%s'", identifier.String())
	return &metadata.ImageIdentifier{
		Reference: identifier.String(),
	}, nil
}
