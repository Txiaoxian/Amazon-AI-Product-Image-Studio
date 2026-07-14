package task

import (
	"errors"
	"testing"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/database"
)

func TestValidateModelCapabilityRequiresEditForReferenceGeneration(t *testing.T) {
	model := database.AIModel{SupportsGenerate: true, SupportsEdit: false, SupportsMultiReference: true}

	if err := validateModelCapability(TypeImageGeneration, model, nil); err != nil {
		t.Fatalf("plain generation capability error = %v", err)
	}
	if err := validateModelCapability(TypeImageGeneration, model, []string{"reference-1"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("reference generation error = %v, want ErrValidation", err)
	}

	model.SupportsEdit = true
	if err := validateModelCapability(TypeImageGeneration, model, []string{"reference-1"}); err != nil {
		t.Fatalf("reference generation with edit capability error = %v", err)
	}
}
