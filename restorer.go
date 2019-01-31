package lifecycle

import (
	"encoding/json"
	"log"

	"github.com/buildpack/lifecycle/fs"
	"github.com/buildpack/lifecycle/image"
)

type Restorer struct {
	LayersDir  string
	Buildpacks []string
	Out, Err   *log.Logger
}

func (r *Restorer) Restore(cacheImage image.Image) error {
	if found, err := cacheImage.Found(); !found || err != nil {
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
	for _, bp := range metadata.Buildpacks {
		layersDir, err := readBuildpackLayersDir(r.LayersDir, bp.ID)
		if err != nil {
			return err
		}
		for name, layer := range bp.Layers {
			r.Out.Printf("writing metadata for cached layer '%s:%s'", bp.ID, name)
			if err := layersDir.newBPLayer(name).writeMetadata(bp.Layers); err != nil {
				return err
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


