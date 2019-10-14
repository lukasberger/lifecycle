package metadata

import (
	"encoding/json"
	"path"

	"github.com/buildpack/imgutil"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
)

const LayerMetadataLabel = "io.buildpacks.lifecycle.metadata"

type LayersMetadata struct {
	App        LayerMetadata             `json:"app" toml:"app"`
	Config     LayerMetadata             `json:"config" toml:"config"`
	Launcher   LayerMetadata             `json:"launcher" toml:"launcher"`
	Buildpacks []BuildpackLayersMetadata `json:"buildpacks" toml:"buildpacks"`
	RunImage   RunImageMetadata          `json:"runImage" toml:"run-image"`
	Stack      StackMetadata             `json:"stack" toml:"stack"`
}

type AnalyzedMetadata struct {
	Image    *ImageIdentifier `toml:"image"`
	Metadata LayersMetadata   `toml:"metadata"`
}

// FIXME: fix key names to be accurate in the daemon case
type ImageIdentifier struct {
	Reference string `toml:"reference"`
}

type LayerMetadata struct {
	SHA string `json:"sha" toml:"sha"`
}

type BuildpackLayersMetadata struct {
	ID      string                            `json:"key" toml:"key"`
	Version string                            `json:"version" toml:"version"`
	Layers  map[string]BuildpackLayerMetadata `json:"layers" toml:"layers"`
}

type BuildpackLayerMetadata struct {
	LayerMetadata
	BuildpackLayerMetadataFile
}

type BuildpackLayerMetadataFile struct {
	Data   interface{} `json:"data" toml:"metadata"`
	Build  bool        `json:"build" toml:"build"`
	Launch bool        `json:"launch" toml:"launch"`
	Cache  bool        `json:"cache" toml:"cache"`
}

type RunImageMetadata struct {
	TopLayer  string `json:"topLayer" toml:"top-layer"`
	Reference string `json:"reference" toml:"reference"`
}

type StackMetadata struct {
	RunImage StackRunImageMetadata `json:"runImage" toml:"run-image"`
}

type StackRunImageMetadata struct {
	Image   string   `toml:"image" json:"image"`
	Mirrors []string `toml:"mirrors" json:"mirrors,omitempty"`
}

func (sm *StackMetadata) BestRunImageMirror(registry string) (string, error) {
	if sm.RunImage.Image == "" {
		return "", errors.New("missing run-image metadata")
	}
	runImageMirrors := []string{sm.RunImage.Image}
	runImageMirrors = append(runImageMirrors, sm.RunImage.Mirrors...)
	runImageRef, err := byRegistry(registry, runImageMirrors)
	if err != nil {
		return "", errors.Wrap(err, "failed to find run-image")
	}
	return runImageRef, nil
}

func (m *LayersMetadata) MetadataForBuildpack(id string) BuildpackLayersMetadata {
	for _, bpMd := range m.Buildpacks {
		if bpMd.ID == id {
			return bpMd
		}
	}
	return BuildpackLayersMetadata{}
}

func GetLayersMetadata(image imgutil.Image) (LayersMetadata, error) {
	contents, err := GetRawMetadata(image, LayerMetadataLabel)
	if err != nil {
		return LayersMetadata{}, err
	}

	meta := LayersMetadata{}
	if err := json.Unmarshal([]byte(contents), &meta); err != nil {
		return LayersMetadata{}, nil
	}
	return meta, nil
}

func GetRawMetadata(image imgutil.Image, metadataLabel string) (string, error) {
	if !image.Found() {
		return "", nil
	}
	contents, err := image.Label(metadataLabel)
	if err != nil {
		return "", errors.Wrapf(err, "retrieving label '%s' for image '%s'", metadataLabel, image.Name())
	}
	return contents, nil
}

func FilePath(layersDir string) string {
	return path.Join(layersDir, "config", "metadata.toml")
}

func byRegistry(reg string, imgs []string) (string, error) {
	if len(imgs) < 1 {
		return "", errors.New("no images provided to search")
	}

	for _, img := range imgs {
		ref, err := name.ParseReference(img, name.WeakValidation)
		if err != nil {
			continue
		}
		if reg == ref.Context().RegistryStr() {
			return img, nil
		}
	}
	return imgs[0], nil
}
