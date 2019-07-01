package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/buildpack/imgutil"
	"github.com/buildpack/imgutil/local"
	"github.com/buildpack/imgutil/remote"
	"github.com/pkg/errors"

	"github.com/buildpack/lifecycle"
	"github.com/buildpack/lifecycle/cache"
	"github.com/buildpack/lifecycle/cmd"
	"github.com/buildpack/lifecycle/docker"
	"github.com/buildpack/lifecycle/image"
	"github.com/buildpack/lifecycle/image/auth"
	"github.com/buildpack/lifecycle/metadata"
)

var (
	repoNames      []string
	runImageRef    string
	layersDir      string
	appDir         string
	groupPath      string
	analyzedPath   string
	stackPath      string
	launchCacheDir string
	useDaemon      bool
	useHelpers     bool
	uid            int
	gid            int
)

const launcherPath = "/lifecycle/launcher"

func init() {
	cmd.FlagRunImage(&runImageRef)
	cmd.FlagLayersDir(&layersDir)
	cmd.FlagAppDir(&appDir)
	cmd.FlagGroupPath(&groupPath)
	cmd.FlagAnalyzedPath(&analyzedPath)
	cmd.FlagStackPath(&stackPath)
	cmd.FlagLaunchCacheDir(&launchCacheDir)
	cmd.FlagUseDaemon(&useDaemon)
	cmd.FlagUseCredHelpers(&useHelpers)
	cmd.FlagUID(&uid)
	cmd.FlagGID(&gid)
}

func main() {
	// suppress output from libraries, lifecycle will not use standard logger
	log.SetOutput(ioutil.Discard)

	flag.Parse()

	for _, v := range flag.Args() {
		if v != "" {
			repoNames = append(repoNames, v)
		}
	}

	if len(repoNames) == 0 {
		cmd.Exit(cmd.FailErrCode(errors.New("at least one image argument is required"), cmd.CodeInvalidArgs, "parse arguments"))
	}

	if launchCacheDir != "" && !useDaemon {
		cmd.Exit(cmd.FailErrCode(errors.New("launch cache can only be used when exporting to a Docker daemon"), cmd.CodeInvalidArgs, "parse arguments"))
	}

	cmd.Exit(export())
}

func export() error {
	var err error

	var group lifecycle.BuildpackGroup
	if _, err := toml.DecodeFile(groupPath, &group); err != nil {
		return cmd.FailErr(err, "read group")
	}

	artifactsDir, err := ioutil.TempDir("", "lifecycle.exporter.layer")
	if err != nil {
		return cmd.FailErr(err, "create temp directory")
	}
	defer os.RemoveAll(artifactsDir)

	outLog := log.New(os.Stdout, "", 0)
	errLog := log.New(os.Stderr, "", 0)
	exporter := &lifecycle.Exporter{
		Buildpacks:   group.Buildpacks,
		Out:          outLog,
		Err:          errLog,
		UID:          uid,
		GID:          gid,
		ArtifactsDir: artifactsDir,
	}

	analyzedMD, err := parseOptionalAnalyzedMD(outLog, analyzedPath)
	if err != nil {
		return cmd.FailErrCode(err, cmd.CodeInvalidArgs, "parse analyzed TOML")
	}

	var registry string
	if registry, err = ensureSingleRegistry(repoNames...); err != nil {
		return cmd.FailErrCode(err, cmd.CodeInvalidArgs, "parse arguments")
	}

	var stackMD metadata.StackMetadata
	_, err = toml.DecodeFile(stackPath, &stackMD)
	if err != nil {
		outLog.Printf("no stack.toml found at path '%s', stack metadata will not be exported\n", stackPath)
	}

	if runImageRef == "" {
		if stackMD.RunImage.Image == "" {
			return cmd.FailErrCode(errors.New("-image is required when there is no stack metadata available"), cmd.CodeInvalidArgs, "parse arguments")
		}

		runImageRef, err = runImageFromStackToml(stackMD, registry)
		if err != nil {
			return err
		}
	}

	if useHelpers {
		if err := lifecycle.SetupCredHelpers(filepath.Join(os.Getenv("HOME"), ".docker"), repoNames[0], runImageRef); err != nil {
			return cmd.FailErr(err, "setup credential helpers")
		}
	}

	var appImage imgutil.Image
	if useDaemon {
		dockerClient, err := docker.DefaultClient()
		if err != nil {
			return err
		}

		var opts = []local.ImageOption{
			local.FromBaseImage(runImageRef),
		}

		if analyzedMD.Image != nil {
			opts = append(opts, local.WithPreviousImage(analyzedMD.Image.Reference))
		}

		appImage, err = local.NewImage(
			repoNames[0],
			dockerClient,
			opts...,
		)

		if err != nil {
			return cmd.FailErr(err, "access run image")
		}

		if launchCacheDir != "" {
			volumeCache, err := cache.NewVolumeCache(launchCacheDir)
			if err != nil {
				return cmd.FailErr(err, "create launch cache")
			}
			appImage = lifecycle.NewCachingImage(appImage, volumeCache)
		}
	} else {
		var opts = []remote.ImageOption{
			remote.FromBaseImage(runImageRef),
		}

		if analyzedMD.Image != nil {
			opts = append(opts, remote.WithPreviousImage(analyzedMD.Image.Reference))
			analyzedRegistry, err := image.ParseRegistry(analyzedMD.Image.Reference)
			if err != nil {
				return cmd.FailErr(err, "parse analyzed registry")
			}
			if analyzedRegistry != registry {
				return fmt.Errorf("analyzed image is on a different registry %s from the exported image %s", analyzedRegistry, registry)
			}
		}

		appImage, err = remote.NewImage(
			repoNames[0],
			auth.DefaultEnvKeychain(),
			opts...,
		)
		if err != nil {
			return cmd.FailErr(err, "access run image")
		}
	}

	if err := exporter.Export(layersDir, appDir, appImage, analyzedMD.Metadata, repoNames[1:], launcherPath, stackMD); err != nil {
		if _, isSaveError := err.(imgutil.SaveError); isSaveError {
			return cmd.FailErrCode(err, cmd.CodeFailedSave, "export")
		}

		return cmd.FailErr(err, "export")
	}

	return nil
}

func parseOptionalAnalyzedMD(logger *log.Logger, path string) (metadata.AnalyzedMetadata, error) {
	var analyzedMD metadata.AnalyzedMetadata

	_, err := toml.DecodeFile(path, &analyzedMD)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Printf("Warning: analyzed TOML file not found at '%s'", path)
			return metadata.AnalyzedMetadata{}, nil
		}

		return metadata.AnalyzedMetadata{}, err
	}

	return analyzedMD, nil
}

func runImageFromStackToml(stack metadata.StackMetadata, registry string) (string, error) {
	runImageMirrors := []string{stack.RunImage.Image}
	runImageMirrors = append(runImageMirrors, stack.RunImage.Mirrors...)
	runImageRef, err := image.ByRegistry(registry, runImageMirrors)
	if err != nil {
		return "", cmd.FailErrCode(err, cmd.CodeInvalidArgs, "parse mirrors")
	}
	return runImageRef, nil
}

func ensureSingleRegistry(repoNames ...string) (string, error) {
	set := make(map[string]interface{})

	var (
		err      error
		registry string
	)

	for _, repoName := range repoNames {
		registry, err = image.ParseRegistry(repoName)
		if err != nil {
			return "", errors.Wrapf(err, "parsing registry from repo '%s'", repoName)
		}
		set[registry] = nil
	}

	if len(set) != 1 {
		return "", errors.New("exporting to multiple registries is unsupported")
	}

	return registry, nil
}
