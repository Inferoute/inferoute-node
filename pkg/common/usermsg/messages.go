package usermsg

import (
	"fmt"

	"github.com/sentnl/inferoute-node/pkg/common"
)

// NoMatchingProviderError returns a user-friendly error when no provider matches the user's cost constraints
func NoMatchingProviderError(modelName string, maxInputCost, maxOutputCost float64) error {
	return common.ErrNotFound(fmt.Errorf(
		"no available provider for model '%s' matching your cost constraints (max input: %.6f, max output: %.6f tokens). "+
			"Please try increasing your cost limits or try a different model",
		modelName, maxInputCost, maxOutputCost))
}

// ModelNotAvailableError returns a user-friendly error when no provider offers the requested model
func ModelNotAvailableError(modelName string) error {
	return common.ErrNotFound(fmt.Errorf(
		"model '%s' is not currently available from any provider. "+
			"Please check the model name or try a different model",
		modelName))
}

// DuplicateModelError returns a user-friendly error when a provider tries to add a model that already exists
func DuplicateModelError(modelName string) error {
	return common.ErrInvalidInput(fmt.Errorf(
		"model '%s' already exists for this provider. To update the model's configuration, use the PUT /api/provider/models/{model_id} endpoint",
		modelName))
}
