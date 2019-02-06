package lifecycle

import (
	"github.com/buildpack/lifecycle/image"
	"log"
)

type Analyzer struct {
	Buildpacks []*Buildpack
	AppDir     string
	LayersDir  string
	In         []byte
	Out, Err   *log.Logger
}

func (a *Analyzer) Analyze(image image.Image) error {
	found, err := image.Found()
	if err != nil {
		return err
	}

	var metadata AppImageMetadata
	if !found {
		a.Out.Printf("WARNING: image '%s' not found or requires authentication to access\n", image.Name())
	} else {
		metadata, err = getMetadata(image, a.Out)
		if err != nil {
			return err
		}
	}
	return a.analyze(metadata)
}

func (a *Analyzer) analyze(metadata AppImageMetadata) error {
	groupBPs := a.buildpacks()
	for buildpackID := range groupBPs {
		cache, err := readBuildpackLayersDir(a.LayersDir, buildpackID)
		if err != nil {
			return err
		}

		metadataLayers := metadata.metadataForBuildpack(buildpackID).Layers
		for _, cachedLayer := range cache.layers {
			cacheType := cachedLayer.classifyCache(metadataLayers)
			switch cacheType {
			case cacheStaleNoMetadata:
				a.Out.Printf("removing stale cached launch layer '%s/%s', not in metadata \n", buildpackID, cachedLayer)
				if err := cachedLayer.remove(); err != nil {
					return err
				}
			case cacheStaleWrongSHA:
				a.Out.Printf("removing stale cached launch layer '%s/%s'", buildpackID, cachedLayer)
				if err := cachedLayer.remove(); err != nil {
					return err
				}
			case cacheMalformed:
				a.Out.Printf("removing malformed cached layer '%s/%s'", buildpackID, cachedLayer)
				if err := cachedLayer.remove(); err != nil {
					return err
				}
			case cacheNotForLaunch:
				a.Out.Printf("using cached layer '%s/%s'", buildpackID, cachedLayer)
			case cacheValid:
				a.Out.Printf("using cached launch layer '%s/%s'", buildpackID, cachedLayer)
				a.Out.Printf("rewriting metadata for layer '%s/%s'", buildpackID, cachedLayer)
				if err := cachedLayer.writeMetadata(metadataLayers); err != nil {
					return err
				}
				delete(metadataLayers, cachedLayer.name())
			}
		}

		for layer, data := range metadataLayers {
			if !data.Build {
				a.Out.Printf("writing metadata for uncached layer '%s/%s'", buildpackID, layer)
				if err := cache.newBPLayer(layer).writeMetadata(metadataLayers); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (a *Analyzer) buildpacks() map[string]struct{} {
	buildpacks := make(map[string]struct{}, len(a.Buildpacks))
	for _, b := range a.Buildpacks {
		buildpacks[b.EscapedID()] = struct{}{}
	}
	return buildpacks
}
