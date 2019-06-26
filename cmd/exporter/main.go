package main

import (
	"flag"
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

	var analyzedMD metadata.AnalyzedMetadata
	_, err = toml.DecodeFile(analyzedPath, &analyzedMD)
	if err != nil {
		return cmd.FailErrCode(errors.Wrapf(err, "no analyzed.toml found at path '%s'", analyzedPath), cmd.CodeInvalidArgs, "parse arguments")
	}

	if err := validateSingleRegistry(append(repoNames, analyzedMD.Repository)...); err != nil {
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

		runImageRef, err = runImageFromStackToml(stackMD, analyzedMD.Repository)
		if err != nil {
			return err
		}
	}

	if useHelpers {
		if err := lifecycle.SetupCredHelpers(filepath.Join(os.Getenv("HOME"), ".docker"), analyzedMD.Repository, runImageRef); err != nil {
			return cmd.FailErr(err, "setup credential helpers")
		}
	}

	var appImage imgutil.Image
	if useDaemon {
		dockerClient, err := docker.DefaultClient()
		if err != nil {
			return err
		}

		appImage, err = local.NewImage(
			repoNames[0],
			dockerClient,
			local.FromBaseImage(runImageRef),
			local.WithPreviousImage(analyzedMD.FullName()),
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
		appImage, err = remote.NewImage(
			repoNames[0],
			auth.DefaultEnvKeychain(),
			remote.FromBaseImage(runImageRef),
			remote.WithPreviousImage(analyzedMD.FullName()),
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

func runImageFromStackToml(stack metadata.StackMetadata, repoName string) (string, error) {
	registry, err := image.ParseRegistry(repoName)
	if err != nil {
		return "", cmd.FailErrCode(err, cmd.CodeInvalidArgs, "parse image name")
	}

	runImageMirrors := []string{stack.RunImage.Image}
	runImageMirrors = append(runImageMirrors, stack.RunImage.Mirrors...)
	runImageRef, err := image.ByRegistry(registry, runImageMirrors)
	if err != nil {
		return "", cmd.FailErrCode(err, cmd.CodeInvalidArgs, "parse mirrors")
	}
	return runImageRef, nil
}

func validateSingleRegistry(repoNames ...string) error {
	set := make(map[string]interface{})
	for _, repoName := range repoNames {
		registry, err := image.ParseRegistry(repoName)
		if err != nil {
			return errors.Wrapf(err, "parsing registry from repo '%s'", repoName)
		}
		set[registry] = nil
	}
	if len(set) != 1 {
		return errors.New("exporting to multiple registries is unsupported")
	}
	return nil
}
