package metadata

import (
	"github.com/buildpack/imgutil"
)

const StackMetadataLabel = "io.buildpacks.stack.id"

type Stack struct {
	ID string `json:"Id,inline"`
}

func GetStackMetadata(image imgutil.Image) (Stack, error) {
	contents, err := GetRawMetadata(image, StackMetadataLabel)
	if err != nil {
		return Stack{}, err
	}

	return Stack{ID: contents}, nil
}
