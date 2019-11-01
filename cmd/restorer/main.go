package main

import (
	"errors"
	"flag"
	"io/ioutil"
	"log"

	"github.com/buildpack/imgutil/remote"

	"github.com/buildpack/lifecycle"
	"github.com/buildpack/lifecycle/auth"
	"github.com/buildpack/lifecycle/cache"
	"github.com/buildpack/lifecycle/cmd"
)

var (
	cacheDir      string
	cacheImageTag string
	groupPath     string
	layersDir     string
	uid           int
	gid           int
	printVersion  bool
	logLevel      string
)

func init() {
	cmd.FlagCacheDir(&cacheDir)
	cmd.FlagCacheImage(&cacheImageTag)
	cmd.FlagGroupPath(&groupPath)
	cmd.FlagLayersDir(&layersDir)
	cmd.FlagUID(&uid)
	cmd.FlagGID(&gid)
	cmd.FlagVersion(&printVersion)
	cmd.FlagLogLevel(&logLevel)
}

func main() {
	// Suppress output from libraries, lifecycle will not use standard logger.
	log.SetOutput(ioutil.Discard)

	flag.Parse()

	if printVersion {
		cmd.ExitWithVersion()
	}

	if err := cmd.SetLogLevel(logLevel); err != nil {
		cmd.Exit(err)
	}

	if flag.NArg() > 0 {
		cmd.Exit(cmd.FailErrCode(errors.New("received unexpected args"), cmd.CodeInvalidArgs, "parse arguments"))
	}
	if cacheImageTag == "" && cacheDir == "" {
		cmd.Exit(cmd.FailErrCode(errors.New("must supply either -image or -path"), cmd.CodeInvalidArgs, "parse arguments"))
	}
	cmd.Exit(restore())
}

func restore() error {
	group, err := lifecycle.ReadGroup(groupPath)
	if err != nil {
		return cmd.FailErr(err, "read buildpack group")
	}

	restorer := &lifecycle.Restorer{
		LayersDir:  layersDir,
		Buildpacks: group.Group,
		Logger:     cmd.Logger,
		UID:        uid,
		GID:        gid,
	}

	var cacheStore lifecycle.Cache
	if cacheImageTag != "" {
		origCacheImage, err := remote.NewImage(
			cacheImageTag,
			auth.EnvKeychain(cmd.EnvRegistryAuth),
			remote.FromBaseImage(cacheImageTag),
		)
		if err != nil {
			return cmd.FailErr(err, "accessing cache image")
		}

		emptyImage, err := remote.NewImage(
			cacheImageTag,
			auth.EnvKeychain(cmd.EnvRegistryAuth),
			remote.WithPreviousImage(cacheImageTag),
		)
		if err != nil {
			return cmd.FailErr(err, "creating empty image")
		}

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
