package lifecycle

import (
	"encoding/json"
	"log"

	"github.com/buildpack/lifecycle/fs"
	"github.com/buildpack/lifecycle/image"
)

type Restorer struct {
	LayersDir  string
	Buildpacks []*Buildpack
	Out, Err   *log.Logger
}

func (r *Restorer) Restore(cacheImage image.Image) error {
	if found, err := cacheImage.Found(); !found || err != nil {
		r.Out.Printf("cache image '%s' not found, nothing to restore", cacheImage.Name())
		return nil
	}
	metadata := &AppImageMetadata{}
	label, err := cacheImage.Label(MetadataLabel)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(label), metadata); err != nil {
		return err
	}
	archiver := &fs.FS{}
	for _, bp := range r.Buildpacks {
		layersDir, err := readBuildpackLayersDir(r.LayersDir, bp.EscapedID())
		if err != nil {
			return err
		}
		layersToRestore := r.layersToRestore(bp.ID, *metadata)
		r.Out.Printf("found layers '%+v' for buildpack '%s'", layersToRestore, bp.ID)
		for name, layer := range layersToRestore {
			if !layer.Cache {
				r.Out.Printf("skipping cache=false layer '%s:%s'", bp.ID, name)
				continue
			}

			r.Out.Printf("restoring cached layer '%s:%s'", bp.ID, name)
			bpLayer := layersDir.newBPLayer(name)
			if err := bpLayer.writeMetadata(layersToRestore); err != nil {
				return err
			}
			if layer.Launch {
				if err := bpLayer.writeSha(layer.SHA); err != nil {
					return err
				}
			}
			rc, err := cacheImage.GetLayer(layer.SHA)
			if err != nil {
				return err
			}
			defer rc.Close()
			if err := archiver.Untar(rc, "/"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Restorer) layersToRestore(buildpackID string, metadata AppImageMetadata) map[string]LayerMetadata {
	layers := make(map[string]LayerMetadata)
	for _, bp := range metadata.Buildpacks {
		if bp.ID == buildpackID {
			return bp.Layers
		}
	}
	return layers
}
