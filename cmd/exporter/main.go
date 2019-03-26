package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/google/go-containerregistry/pkg/name"
	"io/ioutil"
	"log"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/buildpack/lifecycle"
	"github.com/buildpack/lifecycle/cmd"
	"github.com/buildpack/lifecycle/image"
)

var (
	repoName    string
	runImageRef string
	layersDir   string
	appDir      string
	groupPath   string
	useDaemon   bool
	useHelpers  bool
	uid         int
	gid         int
)

const launcherPath = "/lifecycle/launcher"

func init() {
	cmd.FlagRunImage(&runImageRef)
	cmd.FlagLayersDir(&layersDir)
	cmd.FlagAppDir(&appDir)
	cmd.FlagGroupPath(&groupPath)
	cmd.FlagUseDaemon(&useDaemon)
	cmd.FlagUseCredHelpers(&useHelpers)
	cmd.FlagUID(&uid)
	cmd.FlagGID(&gid)
}

func main() {
	flag.Parse()
	if flag.NArg() > 1 || flag.Arg(0) == "" {
		args := map[string]interface{}{"narg": flag.NArg(), "layersDir": layersDir}
		cmd.Exit(cmd.FailCode(cmd.CodeInvalidArgs, "parse arguments", fmt.Sprintf("%+v", args)))
	}
	repoName = flag.Arg(0)
	cmd.Exit(export())
}

func export() error {
	var err error

	var group lifecycle.BuildpackGroup
	if _, err := toml.DecodeFile(groupPath, &group); err != nil {
		return cmd.FailErr(err, "read group")
	}

	if useHelpers {
		if err := lifecycle.SetupCredHelpers(repoName, runImageRef); err != nil {
			return cmd.FailErr(err, "setup credential helpers")
		}
	}

	artifactsDir, err := ioutil.TempDir("", "lifecycle.exporter.layer")
	if err != nil {
		return cmd.FailErr(err, "create temp directory")
	}
	defer os.RemoveAll(artifactsDir)

	exporter := &lifecycle.Exporter{
		Buildpacks:   group.Buildpacks,
		Out:          log.New(os.Stdout, "", log.LstdFlags),
		Err:          log.New(os.Stderr, "", log.LstdFlags),
		UID:          uid,
		GID:          gid,
		ArtifactsDir: artifactsDir,
	}

	factory, err := image.NewFactory(image.WithOutWriter(os.Stdout), image.WithEnvKeychain, image.WithLegacyEnvKeychain)
	if err != nil {
		return err
	}

	mirrorsPath := os.Getenv("CNB_MIRRORS_PATH")
	if mirrorsPath == "" {
		return errors.New("env var CNB_MIRRORS_PATH must be set")
	}

	mirrors := lifecycle.MirrorsType{}
	_, err = toml.DecodeFile(mirrorsPath, &mirrors)
	if err != nil {
		return err
	}

	if runImageRef == "" {
		runImageRef, err = selectRunImageMirror(repoName, mirrors)
		if err != nil {
			return err
		}
	}

	var runImage, origImage image.Image
	if useDaemon {
		runImage, err = factory.NewLocal(runImageRef)
		if err != nil {
			return err
		}
		origImage, err = factory.NewLocal(repoName)
		if err != nil {
			return err
		}
	} else {
		runImage, err = factory.NewRemote(runImageRef)
		if err != nil {
			return err
		}
		origImage, err = factory.NewRemote(repoName)
		if err != nil {
			return err
		}
	}

	if err := exporter.Export(layersDir, appDir, runImage, origImage, launcherPath, mirrors); err != nil {
		return cmd.FailErrCode(err, cmd.CodeFailedBuild)
	}

	return nil
}

func selectRunImageMirror(repoName string, mirrors lifecycle.MirrorsType) (string, error) {
	desiredRegistry, err := registry(repoName)
	if err != nil {
		return "", err
	}

	fmt.Fprintln(os.Stderr, ">>>>>>>>>>>>>>>>>>>>>>>>>>", desiredRegistry)

	runImageList := append([]string{mirrors.Image}, mirrors.Mirrors...)
	fmt.Fprintln(os.Stderr, "=========================", runImageList)
	for _, img := range runImageList {
		if reg, err := registry(img); err == nil && reg == desiredRegistry {
			return img, nil
		}
	}

	return "", fmt.Errorf("failed to select run image mirror for: %s", repoName)
}

func registry(imageName string) (string, error) {
	ref, err := name.ParseReference(imageName, name.WeakValidation)
	if err != nil {
		return "", err
	}
	return ref.Context().RegistryStr(), nil
}
