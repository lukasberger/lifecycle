package image_test

import (
	"testing"

	"github.com/sclevine/spec"

	"github.com/buildpack/lifecycle/image"
	h "github.com/buildpack/lifecycle/testhelpers"
)

func TestImage(t *testing.T) {
	spec.Run(t, "Test Image", testImage)
}

func testImage(t *testing.T, when spec.G, it spec.S) {
	when("#EnsureSingleRegistry", func() {
		when("multiple registries are provided", func() {
			it("errors as unsupported", func() {
				_, err := image.EnsureSingleRegistry("some/repo", "gcr.io/other-repo:latest", "example.com/final-repo")
				h.AssertError(t, err, "exporting to multiple registries is unsupported")
			})
		})

		when("a single registry is provided", func() {
			it("does not return an error and", func() {
				registry, err := image.EnsureSingleRegistry("gcr.io/some/repo", "gcr.io/other-repo:latest", "gcr.io/final-repo")
				h.AssertNil(t, err)

				h.AssertEq(t, registry, "gcr.io")
			})
		})
	})
}
