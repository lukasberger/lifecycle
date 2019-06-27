package main

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	h "github.com/buildpack/lifecycle/testhelpers"
)

func TestAnalyzer(t *testing.T) {
	spec.Run(t, "exporter", testExporter, spec.Report(report.Terminal{}))
}

func testExporter(t *testing.T, when spec.G, it spec.S) {
	when("#ensureSingleRegistry", func() {
		when("multiple registries are provided", func() {
			it("errors as unsupported", func() {
				_, err := ensureSingleRegistry("some/repo", "gcr.io/other-repo:latest", "example.com/final-repo")
				h.AssertError(t, err, "exporting to multiple registries is unsupported")
			})
		})

		when("a single registry is provided", func() {
			it("does not return an error and", func() {
				registry, err := ensureSingleRegistry("gcr.io/some/repo", "gcr.io/other-repo:latest", "gcr.io/final-repo")
				h.AssertNil(t, err)

				h.AssertEq(t, registry, "gcr.io")
			})
		})
	})
}
