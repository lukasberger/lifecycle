package lifecycle_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	"github.com/buildpack/lifecycle"
	"github.com/buildpack/lifecycle/fs"
	"github.com/buildpack/lifecycle/image"
	h "github.com/buildpack/lifecycle/testhelpers"
)

func TestRestorer(t *testing.T) {
	spec.Run(t, "Restorer", testRestorer, spec.Report(report.Terminal{}))
}

func testRestorer(t *testing.T, when spec.G, it spec.S) {
	when("#Restore", func() {
		var (
			restorer  *lifecycle.Restorer
			layersDir string
		)

		it.Before(func() {
			var err error
			layersDir, err = ioutil.TempDir("", "lifecycle-layer-dir")
			h.AssertNil(t, err)


			restorer = &lifecycle.Restorer{
				LayersDir: layersDir,
				Buildpacks: []string{
					"buildpack.id",
				},
				Out: log.New(ioutil.Discard, "", 0),
			}
		})

		it.After(func() {
			h.AssertNil(t, os.RemoveAll(layersDir))
		})

		when("there is no cache image", func() {
			var cacheImage image.Image

			it.Before(func() {
				factory, err := image.DefaultFactory()
				h.AssertNil(t, err)
				cacheImage, err = factory.NewLocal("not-exist", false)
				h.AssertNil(t, err)
			})

			it("does nothing", func() {
				h.AssertNil(t, restorer.Restore(cacheImage))
			})
		})

		when("there is a cache image", func() {
			var (
				cacheImage *h.FakeImage
				tarTempDir string
				cacheOnlyLayerSHA string
			)

			it.Before(func() {
				h.RecursiveCopy(t, filepath.Join("testdata", "restorer"), layersDir)
				var err error

				cacheImage = h.NewFakeImage(t, "cache-image-name", "", "")
				tarTempDir, err = ioutil.TempDir("", "restorer-test")
				h.AssertNil(t, err)
				cacheOnlyLayerTarFile, err := os.Create(filepath.Join(tarTempDir, "temp-layer.tar"))
				h.AssertNil(t, err)
				err = (&fs.FS{}).
					WriteTarArchive(cacheOnlyLayerTarFile, filepath.Join(layersDir, "buildpack.id", "cache-only"), 0, 0)
				h.AssertNil(t, err)
				cacheOnlyLayerSHA = h.ComputeSHA256ForFile(t, filepath.Join(tarTempDir, "temp-layer.tar"))
				cacheImage.AddLayer(filepath.Join(tarTempDir, "temp-layer.tar"))
				h.AssertNil(t, os.RemoveAll(layersDir))
				h.AssertNil(t, os.Mkdir(layersDir, 0777))

				cacheImage.SetLabel("io.buildpacks.lifecycle.metadata", fmt.Sprintf(`{
  "buildpacks": [
    {
      "key": "buildpack.id",
      "layers": {
        "cache-only": {
          "data": {
            "cache-only-key": "cache-only-val"
          },
          "cache": true,
          "sha": "%s"
        }
      }
    }
  ]
}`, cacheOnlyLayerSHA))
			})

			it.After(func() {
				os.RemoveAll(tarTempDir)
			})

			it.Focus("restores the layers to the layers dir", func() {
				h.AssertNil(t, restorer.Restore(cacheImage))
				if txt, err := ioutil.ReadFile(filepath.Join(layersDir, "buildpack.id", "cache-only.toml")); err != nil {
					t.Fatalf("failed to read cache-only.toml: %s", err)
				} else if !strings.Contains(string(txt), `[metadata]
  cache-only-key = "cache-only-val"`) {
					t.Fatalf(`Error: expected "%s" to be equal %s`, txt, `cache-only-key = "cache-only-val"`)
				}

				if txt, err := ioutil.ReadFile(filepath.Join(layersDir, "buildpack.id", "cache-only", "file-from-cache-only-layer")); err != nil {
					t.Fatalf("failed to read file-from-cache-only-layer: %s", err)
				} else if !strings.Contains(string(txt), "echo text from cache-only layer") {
					t.Fatalf(`Error: expected "%s" to be equal %s`, txt, "echo text from cache-only layer")
				}
			})
		})
	})
}
