package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/buildpack/imgutil"

	"github.com/buildpack/lifecycle"
	"github.com/buildpack/lifecycle/cmd"
	"github.com/buildpack/lifecycle/docker"
	"github.com/buildpack/lifecycle/image/auth"
)

var (
	repoName     string
	appDir       string
	layersDir    string
	analyzedPath string
	groupPath    string
	useDaemon    bool
	useHelpers   bool
	uid          int
	gid          int
)

func init() {
	cmd.FlagAppDir(&appDir)
	cmd.FlagGID(&gid)
	cmd.FlagGroupPath(&groupPath)
	cmd.FlagLayersDir(&layersDir)
	cmd.FlagAnalyzedPath(&analyzedPath)
	cmd.FlagUID(&uid)
	cmd.FlagUseDaemon(&useDaemon)
	cmd.FlagUseCredHelpers(&useHelpers)
}

func main() {
	// suppress output from libraries, lifecycle will not use standard logger
	log.SetOutput(ioutil.Discard)

	flag.Parse()
	if flag.NArg() > 1 {
		cmd.Exit(cmd.FailErrCode(fmt.Errorf("received %d args expected 1", flag.NArg()), cmd.CodeInvalidArgs, "parse arguments"))
	}
	if flag.Arg(0) == "" {
		cmd.Exit(cmd.FailErrCode(errors.New("image argument is required"), cmd.CodeInvalidArgs, "parse arguments"))
	}
	repoName = flag.Arg(0)
	cmd.Exit(analyzer())
}

func analyzer() error {
	if useHelpers {
		if err := lifecycle.SetupCredHelpers(filepath.Join(os.Getenv("HOME"), ".docker"), repoName); err != nil {
			return cmd.FailErr(err, "setup credential helpers")
		}
	}

	var group lifecycle.BuildpackGroup
	if _, err := toml.DecodeFile(groupPath, &group); err != nil {
		return cmd.FailErr(err, "read group")
	}

	analyzer := &lifecycle.Analyzer{
		Buildpacks:   group.Buildpacks,
		AppDir:       appDir,
		LayersDir:    layersDir,
		AnalyzedPath: analyzedPath,
		Out:          log.New(os.Stdout, "", 0),
		Err:          log.New(os.Stderr, "", 0),
		UID:          uid,
		GID:          gid,
	}

	var err error
	var previousImage imgutil.Image

	if useDaemon {
		dockerClient, err := docker.DefaultClient()
		if err != nil {
			return cmd.FailErr(err, "create docker client")
		}
		previousImage, err = imgutil.NewLocalImage(
			repoName,
			dockerClient,
			imgutil.FromLocalImageBase(repoName),
		)
		if err != nil {
			return cmd.FailErr(err, "access previous image")
		}
	} else {
		previousImage, err = imgutil.NewRemoteImage(
			repoName,
			auth.DefaultEnvKeychain(),
			imgutil.FromRemoteImageBase(repoName),
		)
		if err != nil {
			return cmd.FailErr(err, "access previous image")
		}
	}

	if err := analyzer.Analyze(previousImage); err != nil {
		return cmd.FailErrCode(err, cmd.CodeFailed, "analyze")
	}

	return nil
}
