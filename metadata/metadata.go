package metadata

import (
	"encoding/json"

	"github.com/buildpack/imgutil"
	"github.com/pkg/errors"
)

const AppMetadataLabel = "io.buildpacks.lifecycle.metadata"

type AppImageMetadata struct {
	App        AppMetadata         `json:"app" toml:"app"`
	Config     ConfigMetadata      `json:"config" toml:"config"`
	Launcher   LauncherMetadata    `json:"launcher" toml:"launcher"`
	Buildpacks []BuildpackMetadata `json:"buildpacks" toml:"buildpacks"`
	RunImage   RunImageMetadata    `json:"runImage" toml:"run-image"`
	Stack      StackMetadata       `json:"stack" toml:"stack"`
}

type AppMetadata struct {
	SHA string `json:"sha" toml:"sha"`
}
// Registry A+
// registry.com/some/repo:tag -> analyze -> md registry.com/some/repo@sha256:ab1345
// -> export (previous = some/repo@sha:ab1345) -> some/repo:tag

// Daemon
// [registry.com/]some/repo:tag -> analyze -> md registry.com/some/repo:tag or bec1c1
// registry.com/repo/name:tag@sha256:digest
type AnalyzedMetadata struct {
	Repository string           `toml:"repository"`
	Digest     string           `toml:"digest"`
	Metadata   AppImageMetadata `toml:"metadata"`
}

func (a AnalyzedMetadata) FullName() string {
	if a.Digest == "" {
		return a.Repository
	}
	return a.Repository + "@" + a.Digest
}

type ConfigMetadata struct {
	SHA string `json:"sha" toml:"sha"`
}

type LauncherMetadata struct {
	SHA string `json:"sha" toml:"sha"`
}

type BuildpackMetadata struct {
	ID      string                   `json:"key" toml:"key"`
	Version string                   `json:"version" toml:"version"`
	Layers  map[string]LayerMetadata `json:"layers" toml:"layers"`
}

type LayerMetadata struct {
	SHA    string      `json:"sha" toml:"sha"`
	Data   interface{} `json:"data" toml:"metadata"`
	Build  bool        `json:"build" toml:"build"`
	Launch bool        `json:"launch" toml:"launch"`
	Cache  bool        `json:"cache" toml:"cache"`
}

type RunImageMetadata struct {
	TopLayer string `json:"topLayer" toml:"topLayer"`
	SHA      string `json:"sha" toml:"sha"`
}

type StackMetadata struct {
	RunImage StackRunImageMetadata `json:"runImage" toml:"run-image"`
}

type StackRunImageMetadata struct {
	Image   string   `toml:"image" json:"image"`
	Mirrors []string `toml:"mirrors" json:"mirrors,omitempty"`
}

func (m *AppImageMetadata) MetadataForBuildpack(id string) BuildpackMetadata {
	for _, bpMd := range m.Buildpacks {
		if bpMd.ID == id {
			return bpMd
		}
	}
	return BuildpackMetadata{}
}

func GetAppMetadata(image imgutil.Image) (AppImageMetadata, error) {
	contents, err := GetRawMetadata(image, AppMetadataLabel)
	if err != nil {
		return AppImageMetadata{}, err
	}

	meta := AppImageMetadata{}
	_ = json.Unmarshal([]byte(contents), &meta)
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
