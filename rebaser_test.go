package lifecycle_test

import (
	"bytes"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/buildpack/imgutil/fakes"
	"github.com/buildpack/imgutil/local"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpack/lifecycle"
	"github.com/buildpack/lifecycle/internal/mocks"
	"github.com/buildpack/lifecycle/metadata"
	h "github.com/buildpack/lifecycle/testhelpers"
)

func TestRebaser(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())
	spec.Run(t, "Rebaser", testRebaser, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testRebaser(t *testing.T, when spec.G, it spec.S) {
	var (
		rebaser          *lifecycle.Rebaser
		fakeWorkingImage *fakes.Image
		fakeNewBaseImage *fakes.Image
		outLog           bytes.Buffer
		additionalNames  []string
	)

	it.Before(func() {
		outLog = bytes.Buffer{}

		fakeWorkingImage = fakes.NewImage(
			"some-repo/app-image",
			"some-top-layer-sha",
			local.IDIdentifier{
				ImageID: "some-image-id",
			},
		)
		h.AssertNil(t, fakeWorkingImage.SetLabel(metadata.StackMetadataLabel, "io.buildpacks.stacks.bionic"))

		fakeNewBaseImage = fakes.NewImage(
			"some-repo/new-base-image",
			"new-top-layer-sha",
			local.IDIdentifier{
				ImageID: "new-run-id",
			},
		)
		h.AssertNil(t, fakeNewBaseImage.SetLabel(metadata.StackMetadataLabel, "io.buildpacks.stacks.bionic"))

		additionalNames = []string{"some-repo/app-image:foo", "some-repo/app-image:bar"}

		rebaser = &lifecycle.Rebaser{
			Logger: mocks.NewMockLogger(io.MultiWriter(&outLog, it.Out())),
		}
	})

	it.After(func() {
		h.AssertNil(t, fakeWorkingImage.Cleanup())
		h.AssertNil(t, fakeNewBaseImage.Cleanup())
	})

	when("#Rebase", func() {
		when("app image and run image exist", func() {
			it("updates the base image of the working image", func() {
				h.AssertNil(t, rebaser.Rebase(fakeWorkingImage, fakeNewBaseImage, additionalNames))
				h.AssertEq(t, fakeWorkingImage.Base(), "some-repo/new-base-image")
			})

			it("saves to all names", func() {
				h.AssertNil(t, rebaser.Rebase(fakeWorkingImage, fakeNewBaseImage, additionalNames))
				h.AssertContains(t, fakeWorkingImage.SavedNames(), "some-repo/app-image", "some-repo/app-image:foo", "some-repo/app-image:bar")
			})

			it("sets the top layer in the metadata", func() {
				h.AssertNil(t, rebaser.Rebase(fakeWorkingImage, fakeNewBaseImage, additionalNames))
				md, err := metadata.GetLayersMetadata(fakeWorkingImage)
				h.AssertNil(t, err)

				h.AssertEq(t, md.RunImage.TopLayer, "new-top-layer-sha")
			})

			it("sets the run image reference in the metadata", func() {
				h.AssertNil(t, rebaser.Rebase(fakeWorkingImage, fakeNewBaseImage, additionalNames))
				md, err := metadata.GetLayersMetadata(fakeWorkingImage)
				h.AssertNil(t, err)

				h.AssertEq(t, md.RunImage.Reference, "new-run-id")
			})

			it("preserves other existing metadata", func() {
				h.AssertNil(t, fakeWorkingImage.SetLabel(
					metadata.LayerMetadataLabel,
					`{"buildpacks":[{"key": "buildpack.id", "layers": {}}]}`,
				))
				h.AssertNil(t, rebaser.Rebase(fakeWorkingImage, fakeNewBaseImage, additionalNames))
				md, err := metadata.GetLayersMetadata(fakeWorkingImage)
				h.AssertNil(t, err)

				h.AssertEq(t, len(md.Buildpacks), 1)
				h.AssertEq(t, md.Buildpacks[0].ID, "buildpack.id")
			})
		})

		when("app image and run image are based on different stacks", func() {
			it("returns an error and prevents the rebase from taking place when the stacks are different", func() {
				h.AssertNil(t, fakeWorkingImage.SetLabel(metadata.StackMetadataLabel, "io.buildpacks.stacks.bionic"))
				h.AssertNil(t, fakeNewBaseImage.SetLabel(metadata.StackMetadataLabel, "io.buildpacks.stacks.cflinuxfs3"))

				err := rebaser.Rebase(fakeWorkingImage, fakeNewBaseImage, additionalNames)
				h.AssertError(t, err, "incompatible stack: 'io.buildpacks.stacks.cflinuxfs3' is not compatible with 'io.buildpacks.stacks.bionic'")
			})

			it("returns an error and prevents the rebase from taking place when the new base image has no stack defined", func() {
				h.AssertNil(t, fakeWorkingImage.SetLabel(metadata.StackMetadataLabel, "io.buildpacks.stacks.bionic"))
				h.AssertNil(t, fakeNewBaseImage.SetLabel(metadata.StackMetadataLabel, ""))

				err := rebaser.Rebase(fakeWorkingImage, fakeNewBaseImage, additionalNames)
				h.AssertError(t, err, "stack not defined on new base image")
			})

			it("returns an error and prevents the rebase from taking place when the working image has no stack defined", func() {
				h.AssertNil(t, fakeWorkingImage.SetLabel(metadata.StackMetadataLabel, ""))
				h.AssertNil(t, fakeNewBaseImage.SetLabel(metadata.StackMetadataLabel, "io.buildpacks.stacks.cflinuxfs3"))

				err := rebaser.Rebase(fakeWorkingImage, fakeNewBaseImage, additionalNames)
				h.AssertError(t, err, "stack not defined on working image")
			})
		})
	})
}
