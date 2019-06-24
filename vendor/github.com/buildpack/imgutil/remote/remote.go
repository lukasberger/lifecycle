package remote

import (
	"fmt"
	"github.com/buildpack/imgutil"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"
)

type Image struct {
	keychain   authn.Keychain
	repoName   string
	image      v1.Image
	prevLayers []v1.Layer
}

type ImageOption func(*Image) (*Image, error)

func WithPreviousImage(imageName string) ImageOption {
	return func(r *Image) (*Image, error) {
		var err error

		prevImage, err := newV1Image(r.keychain, imageName)
		if err != nil {
			return nil, err
		}

		prevLayers, err := prevImage.Layers()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get layers for previous image with repo name '%s'", imageName)
		}

		r.prevLayers = prevLayers
		return r, nil
	}
}

func FromBaseImage(imageName string) ImageOption {
	return func(r *Image) (*Image, error) {
		var err error
		r.image, err = newV1Image(r.keychain, imageName)
		if err != nil {
			return nil, err
		}
		return r, nil
	}
}

func NewImage(repoName string, keychain authn.Keychain, ops ...ImageOption) (imgutil.Image, error) {
	image, err := emptyImage()
	if err != nil {
		return nil, err
	}

	ri := &Image{
		keychain: keychain,
		repoName: repoName,
		image:    image,
	}

	for _, op := range ops {
		ri, err = op(ri)
		if err != nil {
			return nil, err
		}
	}

	return ri, nil
}

func newV1Image(keychain authn.Keychain, repoName string) (v1.Image, error) {
	ref, auth, err := referenceForRepoName(keychain, repoName)
	if err != nil {
		return nil, err
	}
	image, err := remote.Image(ref, remote.WithAuth(auth), remote.WithTransport(http.DefaultTransport))
	if err != nil {
		if transportErr, ok := err.(*transport.Error); ok && len(transportErr.Errors) > 0 {
			switch transportErr.Errors[0].Code {
			case transport.UnauthorizedErrorCode, transport.ManifestUnknownErrorCode:
				return emptyImage()
			}
		}
		return nil, fmt.Errorf("connect to repo store '%s': %s", repoName, err.Error())
	}
	return image, nil
}

func emptyImage() (v1.Image, error) {
	return random.Image(0, 0)
}

func referenceForRepoName(keychain authn.Keychain, ref string) (name.Reference, authn.Authenticator, error) {
	var auth authn.Authenticator
	r, err := name.ParseReference(ref, name.WeakValidation)
	if err != nil {
		return nil, nil, err
	}

	auth, err = keychain.Resolve(r.Context().Registry)
	if err != nil {
		return nil, nil, err
	}
	return r, auth, nil
}

func (r *Image) Label(key string) (string, error) {
	cfg, err := r.image.ConfigFile()
	if err != nil || cfg == nil {
		return "", fmt.Errorf("failed to get config file for image '%s'", r.repoName)
	}
	labels := cfg.Config.Labels
	return labels[key], nil

}

func (r *Image) Env(key string) (string, error) {
	cfg, err := r.image.ConfigFile()
	if err != nil || cfg == nil {
		return "", fmt.Errorf("failed to get config file for image '%s'", r.repoName)
	}
	for _, envVar := range cfg.Config.Env {
		parts := strings.Split(envVar, "=")
		if parts[0] == key {
			return parts[1], nil
		}
	}
	return "", nil
}

func (r *Image) Rename(name string) {
	r.repoName = name
}

func (r *Image) Name() string {
	return r.repoName
}

func (r *Image) Found() bool {
	ref, auth, err := referenceForRepoName(r.keychain, r.repoName)
	if err != nil {
		return false
	}
	_, err = remote.Image(ref, remote.WithAuth(auth), remote.WithTransport(http.DefaultTransport))
	if err != nil {
		return false
	}
	return true
}

func (r *Image) Digest() (string, error) {
	hash, err := r.image.Digest()
	if err != nil {
		return "", fmt.Errorf("failed to get digest for image '%s': %s", r.repoName, err)
	}
	return hash.String(), nil
}

func (r *Image) CreatedAt() (time.Time, error) {
	configFile, err := r.image.ConfigFile()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get createdAt time for image '%s': %s", r.repoName, err)
	}
	return configFile.Created.UTC(), nil
}

func (r *Image) Rebase(baseTopLayer string, newBase imgutil.Image) error {
	newBaseRemote, ok := newBase.(*Image)
	if !ok {
		return errors.New("expected new base to be a remote image")
	}

	newImage, err := mutate.Rebase(r.image, &subImage{img: r.image, topSHA: baseTopLayer}, newBaseRemote.image)
	if err != nil {
		return errors.Wrap(err, "rebase")
	}
	r.image = newImage
	return nil
}

func (r *Image) SetLabel(key, val string) error {
	configFile, err := r.image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels[key] = val
	r.image, err = mutate.Config(r.image, config)
	return err
}

func (r *Image) SetEnv(key, val string) error {
	configFile, err := r.image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	for i, e := range config.Env {
		parts := strings.Split(e, "=")
		if parts[0] == key {
			config.Env[i] = fmt.Sprintf("%s=%s", key, val)
			r.image, err = mutate.Config(r.image, config)
			if err != nil {
				return err
			}
			return nil
		}
	}
	config.Env = append(config.Env, fmt.Sprintf("%s=%s", key, val))
	r.image, err = mutate.Config(r.image, config)
	return err
}

func (r *Image) SetWorkingDir(dir string) error {
	configFile, err := r.image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	config.WorkingDir = dir
	r.image, err = mutate.Config(r.image, config)
	return err
}

func (r *Image) SetEntrypoint(ep ...string) error {
	configFile, err := r.image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	config.Entrypoint = ep
	r.image, err = mutate.Config(r.image, config)
	return err
}

func (r *Image) SetCmd(cmd ...string) error {
	configFile, err := r.image.ConfigFile()
	if err != nil {
		return err
	}
	config := *configFile.Config.DeepCopy()
	config.Cmd = cmd
	r.image, err = mutate.Config(r.image, config)
	return err
}

func (r *Image) TopLayer() (string, error) {
	all, err := r.image.Layers()
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return "", fmt.Errorf("image %s has no layers", r.Name())
	}
	topLayer := all[len(all)-1]
	hex, err := topLayer.DiffID()
	if err != nil {
		return "", err
	}
	return hex.String(), nil
}

func (r *Image) GetLayer(sha string) (io.ReadCloser, error) {
	layers, err := r.image.Layers()
	if err != nil {
		return nil, err
	}

	layer, err := findLayerWithSha(layers, sha)
	if err != nil {
		return nil, err
	}

	return layer.Compressed()
}

func (r *Image) AddLayer(path string) error {
	layer, err := tarball.LayerFromFile(path)
	if err != nil {
		return err
	}
	r.image, err = mutate.AppendLayers(r.image, layer)
	if err != nil {
		return errors.Wrap(err, "add layer")
	}
	return nil
}

func (r *Image) ReuseLayer(sha string) error {
	layer, err := findLayerWithSha(r.prevLayers, sha)
	if err != nil {
		return err
	}
	r.image, err = mutate.AppendLayers(r.image, layer)
	return err
}

func findLayerWithSha(layers []v1.Layer, sha string) (v1.Layer, error) {
	for _, layer := range layers {
		diffID, err := layer.DiffID()
		if err != nil {
			return nil, errors.Wrap(err, "get diff ID for previous image layer")
		}
		if sha == diffID.String() {
			return layer, nil
		}
	}
	return nil, fmt.Errorf(`previous image did not have layer with sha '%s'`, sha)
}

func (r *Image) Save(additionalNames ...string) imgutil.SaveResult {
	var err error

	allNames := append([]string{r.repoName}, additionalNames...)

	r.image, err = mutate.CreatedAt(r.image, v1.Time{Time: time.Now()})
	if err != nil {
		return imgutil.NewFailedResult(allNames, err)
	}

	hex, err := r.image.Digest()
	if err != nil {
		return imgutil.NewFailedResult(allNames, err)
	}

	var errs = map[string]error{}
	for _, n := range append([]string{r.repoName}, additionalNames...) {
		errs[n] = r.doSave(n)
	}

	return imgutil.SaveResult{
		Digest:   hex.String(),
		Outcomes: errs,
	}
}

func (r *Image) doSave(imageName string) error {
	ref, auth, err := referenceForRepoName(r.keychain, imageName)
	if err != nil {
		return err
	}

	if err := remote.Write(ref, r.image, remote.WithAuth(auth)); err != nil {
		return err
	}

	return nil
}

func (r *Image) Delete() error {
	return errors.New("remote image does not implement Delete")
}

type subImage struct {
	img    v1.Image
	topSHA string
}

func (si *subImage) Layers() ([]v1.Layer, error) {
	all, err := si.img.Layers()
	if err != nil {
		return nil, err
	}
	for i, l := range all {
		d, err := l.DiffID()
		if err != nil {
			return nil, err
		}
		if d.String() == si.topSHA {
			return all[:i+1], nil
		}
	}
	return nil, errors.New("could not find base layer in image")
}
func (si *subImage) BlobSet() (map[v1.Hash]struct{}, error)  { panic("Not Implemented") }
func (si *subImage) MediaType() (types.MediaType, error)     { panic("Not Implemented") }
func (si *subImage) ConfigName() (v1.Hash, error)            { panic("Not Implemented") }
func (si *subImage) ConfigFile() (*v1.ConfigFile, error)     { panic("Not Implemented") }
func (si *subImage) RawConfigFile() ([]byte, error)          { panic("Not Implemented") }
func (si *subImage) Digest() (v1.Hash, error)                { panic("Not Implemented") }
func (si *subImage) Manifest() (*v1.Manifest, error)         { panic("Not Implemented") }
func (si *subImage) RawManifest() ([]byte, error)            { panic("Not Implemented") }
func (si *subImage) LayerByDigest(v1.Hash) (v1.Layer, error) { panic("Not Implemented") }
func (si *subImage) LayerByDiffID(v1.Hash) (v1.Layer, error) { panic("Not Implemented") }
