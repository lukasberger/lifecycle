package lifecycle_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpack/lifecycle"
	h "github.com/buildpack/lifecycle/testhelpers"
	"github.com/buildpack/lifecycle/testmock"
)

func TestCacher(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())
	spec.Run(t, "Cacher", testCacher, spec.Parallel(), spec.Report(report.Terminal{}))
}

func testCacher(t *testing.T, when spec.G, it spec.S) {
	when("#Cacher", func() {
		var cacher *lifecycle.Cacher

		it.Before(func() {
			tmpDir, err := ioutil.TempDir("", "lifecycle.exporter.layer")
			h.AssertNil(t, err)
			cacher = &lifecycle.Cacher{
				ArtifactsDir: tmpDir,
				Buildpacks: []*lifecycle.Buildpack{
					{ID: "buildpack.id"},
					{ID: "other.buildpack.id"},
				},
				Out: log.New(os.Stdout, "", 0),
			}
		})

		when("there is no previous cached image", func() {
			var mockNonExistingOriginalImage *testmock.MockImage

			it.Before(func() {
				mockNonExistingOriginalImage = testmock.NewMockImage(gomock.NewController(t))
				mockNonExistingOriginalImage.EXPECT().Found().Return(false, nil)
				mockNonExistingOriginalImage.EXPECT().Label("io.buildpacks.lifecycle.metadata").
					Return("", errors.New("not exist")).AnyTimes()
			})

			it("exports cached layers to an image", func() {
				cacheImage := h.NewFakeImage(t, "cache-image", "", "")
				layersDir := filepath.Join("testdata", "cacher", "layers")
				err := cacher.Cache(layersDir, mockNonExistingOriginalImage, cacheImage)
				h.AssertNil(t, err)

				layerPath := cacheImage.FindLayerWithPath(filepath.Join(layersDir, "buildpack.id/cache-true-layer"))

				assertTarFileContents(t,
					layerPath,
					filepath.Join(layersDir, "buildpack.id/cache-true-layer/file-from-cache-true-layer"),
					"file-from-cache-true-contents")

				h.AssertEq(t, cacheImage.IsSaved(), true)
			})

			it("doesn't export uncached layers", func() {
				cacheImage := h.NewFakeImage(t, "cache-image", "", "")
				layersDir := filepath.Join("testdata", "cacher", "layers")
				err := cacher.Cache(layersDir, mockNonExistingOriginalImage, cacheImage)
				h.AssertNil(t, err)

				h.AssertEq(t, cacheImage.NumberOfAddedLayers(), 2)
				h.AssertEq(t, cacheImage.IsSaved(), true)
			})
		})

		when("there is a previous cached image", func() {
			var (
				fakeOriginalImage        *h.FakeImage
				layersDir                string
				computedReusableLayerSHA string
				metadataTemplate         string
			)
			it.Before(func() {
				layersDir = filepath.Join("testdata", "cacher", "layers")
				fakeOriginalImage = h.NewFakeImage(t, "", "", "")
				computedReusableLayerSHA = "sha256:" + h.ComputeSHA256ForPath(t, filepath.Join(layersDir, "buildpack.id/cache-true-no-sha-layer"), 0, 0)
				metadataTemplate = `{
  "buildpacks": [
    {
      "key": "buildpack.id",
      "layers": {
        "cache-true-layer": {
          "cache": true,
          "sha": "%s"
        },
        "cache-true-no-sha-layer": {
          "cache": true,
          "sha": "%s"
        }
      }
    }
  ]
}`
			})
			when("the shas match", func() {
				it.Before(func() {
					h.AssertNil(t, fakeOriginalImage.SetLabel(
						"io.buildpacks.lifecycle.metadata",
						fmt.Sprintf(metadataTemplate, "same-sha", computedReusableLayerSHA),
					))
				})

				it("reuses layers when the existing sha matches previous metadata", func() {
					cacheImage := h.NewFakeImage(t, "cache-image", "", "")
					layersDir := filepath.Join("testdata", "cacher", "layers")
					err := cacher.Cache(layersDir, fakeOriginalImage, cacheImage)
					h.AssertNil(t, err)

					reusedLayers := cacheImage.ReusedLayers()
					h.AssertEq(t, len(reusedLayers), 2)
					h.AssertContains(t, reusedLayers, "same-sha")
					h.AssertEq(t, cacheImage.IsSaved(), true)
				})

				it("reuses layers when the calculated sha matches previous metadata", func() {
					cacheImage := h.NewFakeImage(t, "cache-image", "", "")
					layersDir := filepath.Join("testdata", "cacher", "layers")
					err := cacher.Cache(layersDir, fakeOriginalImage, cacheImage)
					h.AssertNil(t, err)

					reusedLayers := cacheImage.ReusedLayers()
					h.AssertEq(t, len(reusedLayers), 2)
					h.AssertContains(t, reusedLayers, computedReusableLayerSHA)
					h.AssertEq(t, cacheImage.IsSaved(), true)
				})
			})

			when("the shas don't match", func() {
				it.Before(func() {
					h.AssertNil(t, fakeOriginalImage.SetLabel(
						"io.buildpacks.lifecycle.metadata",
						fmt.Sprintf(metadataTemplate, "different-sha", "not-the-sha-you-want"),
					))
				})

				it("doesn't reuse layers", func() {
					cacheImage := h.NewFakeImage(t, "cache-image", "", "")
					layersDir := filepath.Join("testdata", "cacher", "layers")
					err := cacher.Cache(layersDir, fakeOriginalImage, cacheImage)
					h.AssertNil(t, err)

					h.AssertEq(t, len(cacheImage.ReusedLayers()), 0)
					h.AssertEq(t, cacheImage.NumberOfAddedLayers(), 2)
					h.AssertEq(t, cacheImage.IsSaved(), true)
				})
			})
		})
	})
}
