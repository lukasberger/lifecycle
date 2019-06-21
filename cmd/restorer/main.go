package main

import (
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/buildpack/imgutil"

	"github.com/buildpack/lifecycle"
	"github.com/buildpack/lifecycle/cache"
	"github.com/buildpack/lifecycle/cmd"
	"github.com/buildpack/lifecycle/docker"
)

var (
	cacheImageTag string
	cacheDir      string
	layersDir     string
	groupPath     string
	uid           int
	gid           int
)

func init() {
	cmd.FlagLayersDir(&layersDir)
	cmd.FlagCacheImage(&cacheImageTag)
	cmd.FlagCacheDir(&cacheDir)
	cmd.FlagGroupPath(&groupPath)
	cmd.FlagUID(&uid)
	cmd.FlagGID(&gid)
}

func main() {
	// suppress output from libraries, lifecycle will not use standard logger
	log.SetOutput(ioutil.Discard)

	flag.Parse()
	if flag.NArg() > 0 {
		cmd.Exit(cmd.FailErrCode(errors.New("received unexpected args"), cmd.CodeInvalidArgs, "parse arguments"))
	}
	if cacheImageTag == "" && cacheDir == "" {
		cmd.Exit(cmd.FailErrCode(errors.New("must supply either -image or -path"), cmd.CodeInvalidArgs, "parse arguments"))
	}
	cmd.Exit(restore())
}

func restore() error {
	var group lifecycle.BuildpackGroup
	if _, err := toml.DecodeFile(groupPath, &group); err != nil {
		return cmd.FailErr(err, "read group")
	}

	restorer := &lifecycle.Restorer{
		LayersDir:  layersDir,
		Buildpacks: group.Buildpacks,
		Out:        log.New(os.Stdout, "", 0),
		Err:        log.New(os.Stderr, "", 0),
		UID:        uid,
		GID:        gid,
	}

	var cacheStore lifecycle.Cache
	if cacheImageTag != "" {
		dockerClient, err := docker.DefaultClient()
		if err != nil {
			return cmd.FailErr(err, "create docker client")
		}

		origCacheImage, err := imgutil.NewLocalImage(cacheImageTag, dockerClient, imgutil.FromLocalImageBase(cacheImageTag))
		if err != nil {
			return cmd.FailErr(err, "access cache image")
		}

		emptyImage, err := imgutil.NewLocalImage(cacheImageTag, dockerClient, imgutil.WithPreviousLocalImage(cacheImageTag))
		cacheStore = cache.NewImageCache(
			origCacheImage,
			emptyImage,
		)
	} else {
		var err error
		cacheStore, err = cache.NewVolumeCache(cacheDir)
		if err != nil {
			return cmd.FailErr(err, "create volume cache")
		}
	}

	if err := restorer.Restore(cacheStore); err != nil {
		return cmd.FailErrCode(err, cmd.CodeFailed, "restore")
	}
	return nil
}
