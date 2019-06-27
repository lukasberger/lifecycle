package lifecycle

import (
	"log"
	"os"

	"github.com/buildpack/imgutil"
	"github.com/pkg/errors"

	"github.com/buildpack/lifecycle/metadata"
)

type Analyzer struct {
	AnalyzedPath string
	AppDir       string
	Buildpacks   []*Buildpack
	GID, UID     int
	In           []byte
	LayersDir    string
	Out, Err     *log.Logger
	SkipLayers   bool
}

func (a *Analyzer) Analyze(image imgutil.Image) error {
	data, err := metadata.GetAppMetadata(image)
	if err != nil {
		return err
	}

	if !a.SkipLayers {
		for _, buildpack := range a.Buildpacks {
			bpLayersDir, err := readBuildpackLayersDir(a.LayersDir, *buildpack)
			if err != nil {
				return err
			}

			metadataLayers := data.MetadataForBuildpack(buildpack.ID).Layers
			for _, cachedLayer := range bpLayersDir.layers {
				cacheType := cachedLayer.classifyCache(metadataLayers)
				switch cacheType {
				case cacheStaleNoMetadata:
					a.Out.Printf("removing stale cached launch layer '%s', not in metadata \n", cachedLayer.Identifier())
					if err := cachedLayer.remove(); err != nil {
						return err
					}
				case cacheStaleWrongSHA:
					a.Out.Printf("removing stale cached launch layer '%s'", cachedLayer.Identifier())
					if err := cachedLayer.remove(); err != nil {
						return err
					}
				case cacheMalformed:
					a.Out.Printf("removing malformed cached layer '%s'", cachedLayer.Identifier())
					if err := cachedLayer.remove(); err != nil {
						return err
					}
				case cacheNotForLaunch:
					a.Out.Printf("using cached layer '%s'", cachedLayer.Identifier())
				case cacheValid:
					a.Out.Printf("using cached launch layer '%s'", cachedLayer.Identifier())
					a.Out.Printf("rewriting metadata for layer '%s'", cachedLayer.Identifier())
					if err := cachedLayer.writeMetadata(metadataLayers); err != nil {
						return err
					}
				}
			}

			for lmd, data := range metadataLayers {
				if !data.Build && !data.Cache {
					layer := bpLayersDir.newBPLayer(lmd)
					a.Out.Printf("writing metadata for uncached layer '%s'", layer.Identifier())
					if err := layer.writeMetadata(metadataLayers); err != nil {
						return err
					}
				}
			}
		}
	} else {
		a.Out.Printf("Skipping buildpack layer analysis")
	}

	// if analyzer is running as root it needs to fix the ownership of the layers dir
	if current := os.Getuid(); current == 0 {
		if err := recursiveChown(a.LayersDir, a.UID, a.GID); err != nil {
			return errors.Wrapf(err, "chowning layers dir to '%d/%d'", a.UID, a.GID)
		}
	}

	return a.writeAnalyzedMetadata(image, data)
}

func (a *Analyzer) writeAnalyzedMetadata(image imgutil.Image, appMd metadata.AppImageMetadata) error {
	var (
		err    error
		digest string
	)
	if image.Found() {
		digest, err = image.Digest()
		if err != nil {
			return errors.Wrap(err, "retrieve image digest")
		}
	}

	md := metadata.AnalyzedMetadata{
		Repository: image.Name(),
		Digest:     digest,
		Metadata:   appMd,
	}
	if err := WriteTOML(a.AnalyzedPath, md); err != nil {
		return errors.Wrap(err, "write analyzed.toml")
	}

	return nil
}
