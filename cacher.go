package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/buildpack/lifecycle/fs"
	"github.com/buildpack/lifecycle/image"
)

type Cacher struct {
	ArtifactsDir string
	Buildpacks   []*Buildpack
	Out          *log.Logger
}

func (c *Cacher) Cache(layersDir string, oldCacheImage, newCacheImage image.Image) error {
	loggingCacheimage := &loggingImage{
		Out:   c.Out,
		image: newCacheImage,
	}

	origMetadata, err := c.getImageMetadata(oldCacheImage)
	if err != nil {
		return errors.Wrap(err, "metadata for previous image")
	}

	for _, bp := range c.Buildpacks {
		bpDir, err := readBuildpackLayersDir(layersDir, bp.EscapedID())
		if err != nil {
			return err
		}
		for _, l := range bpDir.findLayers(cached) {
			origLayerMetadata := origMetadata.metadataForBuildpack(bp.ID).Layers[l.name()]
			if _, err := c.addOrReuseLayer(loggingCacheimage, l, origLayerMetadata.SHA); err != nil {
				return err
			}
		}
	}
	_, err = loggingCacheimage.Save()
	return err
}

func (c *Cacher) addOrReuseLayer(image *loggingImage, layer bpLayer, previousSHA string) (string, error) {
	md, err := layer.read()
	if err != nil {
		return "", err
	}
	if md.SHA == "" || md.SHA != previousSHA {
		md.SHA, err = c.exportTar(layer.Path())
		if err != nil {
			return "", errors.Wrapf(err, "caching layer '%s'", layer.Identifier())
		}
	}

	if md.SHA == previousSHA {
		return previousSHA, image.ReuseLayer(layer.Identifier(), previousSHA)
	}
	return md.SHA, image.AddLayer(layer.Identifier(), md.SHA, c.tarPath(md.SHA))
}

func (c *Cacher) exportTar(sourceDir string) (string, error) {
	hasher := sha256.New()
	f, err := ioutil.TempFile(c.ArtifactsDir, "tarfile")
	if err != nil {
		return "", err
	}
	defer f.Close()
	w := io.MultiWriter(hasher, f)

	fs := &fs.FS{}
	err = fs.WriteTarArchive(w, sourceDir, 0, 0)
	if err != nil {
		return "", err
	}
	sha := hex.EncodeToString(hasher.Sum(make([]byte, 0, hasher.Size())))

	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(f.Name(), filepath.Join(c.ArtifactsDir, sha+".tar")); err != nil {
		return "", err
	}

	return "sha256:" + sha, nil
}

func (c *Cacher) tarPath(sha string) string {
	return filepath.Join(c.ArtifactsDir, strings.TrimPrefix(sha, "sha256:")+".tar")
}

func (c *Cacher) getImageMetadata(image image.Image) (AppImageMetadata, error) {
	var metadata AppImageMetadata
	found, err := image.Found()
	if err != nil {
		return metadata, errors.Wrap(err, "looking for image")
	}
	if found {
		label, err := image.Label(MetadataLabel)
		if err != nil {
			return metadata, errors.Wrap(err, "getting metadata")
		}
		if err := json.Unmarshal([]byte(label), &metadata); err != nil {
			return metadata, err
		}
	}
	return metadata, nil
}
