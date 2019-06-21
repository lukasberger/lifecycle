package lifecycle

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"

	"github.com/buildpack/imgutil"
	"github.com/pkg/errors"

	"github.com/buildpack/lifecycle/archive"
	"github.com/buildpack/lifecycle/cmd"
	"github.com/buildpack/lifecycle/metadata"
)

type Exporter struct {
	Buildpacks   []*Buildpack
	ArtifactsDir string
	In           []byte
	Out, Err     *log.Logger
	UID, GID     int
}

var FailedToSaveError = errors.New("one or more image names failed to save")

func (e *Exporter) Export(
	layersDir,
	appDir string,
	appImage imgutil.Image,
	origMetadata metadata.AppImageMetadata,
	additionalNames []string,
	launcher string,
	stack metadata.StackMetadata,
) error {
	var err error

	meta := metadata.AppImageMetadata{}

	meta.RunImage.TopLayer, err = appImage.TopLayer()
	if err != nil {
		return errors.Wrap(err, "get run image top layer SHA")
	}

	meta.RunImage.SHA, err = appImage.Digest()
	if err != nil {
		return errors.Wrap(err, "get run image digest")
	}

	meta.Stack = stack

	meta.App.SHA, err = e.addOrReuseLayer(appImage, &layer{path: appDir, identifier: "app"}, origMetadata.App.SHA)
	if err != nil {
		return errors.Wrap(err, "exporting app layer")
	}

	meta.Config.SHA, err = e.addOrReuseLayer(appImage, &layer{path: filepath.Join(layersDir, "config"), identifier: "config"}, origMetadata.Config.SHA)
	if err != nil {
		return errors.Wrap(err, "exporting config layer")
	}

	meta.Launcher.SHA, err = e.addOrReuseLayer(appImage, &layer{path: launcher, identifier: "launcher"}, origMetadata.Launcher.SHA)
	if err != nil {
		return errors.Wrap(err, "exporting launcher layer")
	}

	for _, bp := range e.Buildpacks {
		bpDir, err := readBuildpackLayersDir(layersDir, *bp)
		if err != nil {
			return errors.Wrapf(err, "reading layers for buildpack '%s'", bp.ID)
		}
		bpMD := metadata.BuildpackMetadata{ID: bp.ID, Version: bp.Version, Layers: map[string]metadata.LayerMetadata{}}

		for _, layer := range bpDir.findLayers(launch) {
			lmd, err := layer.read()
			if err != nil {
				return errors.Wrapf(err, "reading '%s' metadata", layer.Identifier())
			}

			if layer.hasLocalContents() {
				origLayerMetadata := origMetadata.MetadataForBuildpack(bp.ID).Layers[layer.name()]
				lmd.SHA, err = e.addOrReuseLayer(appImage, &layer, origLayerMetadata.SHA)
				if err != nil {
					return err
				}
			} else {
				if lmd.Cache {
					return fmt.Errorf("layer '%s' is cache=true but has no contents", layer.Identifier())
				}
				origLayerMetadata, ok := origMetadata.MetadataForBuildpack(bp.ID).Layers[layer.name()]
				if !ok {
					return fmt.Errorf("cannot reuse '%s', previous image has no metadata for layer '%s'", layer.Identifier(), layer.Identifier())
				}

				e.Out.Printf("Reusing layer '%s' with SHA %s\n", layer.Identifier(), origLayerMetadata.SHA)
				if err := appImage.ReuseLayer(origLayerMetadata.SHA); err != nil {
					return errors.Wrapf(err, "reusing layer: '%s'", layer.Identifier())
				}
				lmd.SHA = origLayerMetadata.SHA
			}
			bpMD.Layers[layer.name()] = lmd
		}

		if malformedLayers := bpDir.findLayers(malformed); len(malformedLayers) > 0 {
			ids := make([]string, 0, len(malformedLayers))
			for _, ml := range malformedLayers {
				ids = append(ids, ml.Identifier())
			}
			return fmt.Errorf("failed to parse metadata for layers '%s'", ids)
		}

		meta.Buildpacks = append(meta.Buildpacks, bpMD)
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return errors.Wrap(err, "marshall metadata")
	}
	if err := appImage.SetLabel(metadata.AppMetadataLabel, string(data)); err != nil {
		return errors.Wrap(err, "set app image metadata label")
	}

	if err := appImage.SetEnv(cmd.EnvLayersDir, layersDir); err != nil {
		return errors.Wrapf(err, "set app image env %s", cmd.EnvLayersDir)
	}

	if err := appImage.SetEnv(cmd.EnvAppDir, appDir); err != nil {
		return errors.Wrapf(err, "set app image env %s", cmd.EnvAppDir)
	}

	if err := appImage.SetEntrypoint(launcher); err != nil {
		return errors.Wrap(err, "setting entrypoint")
	}

	if err := appImage.SetCmd(); err != nil { // Note: Command intentionally empty
		return errors.Wrap(err, "setting cmd")
	}

	result := appImage.Save(additionalNames...)

	if result.Digest != "" {
		e.Out.Printf("\n*** Digest: %s\n", result.Digest)
	}

	hasError := false
	e.Out.Println("*** Images:")
	for _, n := range append([]string{appImage.Name()}, additionalNames...) {
		err := result.Outcomes[n]
		if err == nil {
			e.Out.Printf("      %s - succeeded\n", n)
		} else {
			hasError = true
			e.Out.Printf("      %s - %s\n", n, err.Error())
		}
	}

	if hasError {
		return FailedToSaveError
	}

	return nil
}

func (e *Exporter) addOrReuseLayer(image imgutil.Image, layer identifiableLayer, previousSha string) (string, error) {
	tarPath := filepath.Join(e.ArtifactsDir, escapeIdentifier(layer.Identifier())+".tar")
	sha, err := archive.WriteTarFile(layer.Path(), tarPath, e.UID, e.GID)
	if err != nil {
		return "", errors.Wrapf(err, "exporting layer '%s'", layer.Identifier())
	}
	if sha == previousSha {
		e.Out.Printf("Reusing layer '%s' with SHA %s\n", layer.Identifier(), sha)
		return sha, image.ReuseLayer(previousSha)
	}
	e.Out.Printf("Exporting layer '%s' with SHA %s\n", layer.Identifier(), sha)
	return sha, image.AddLayer(tarPath)
}
