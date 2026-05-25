package config_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestValidationError_Error_WithHint(t *testing.T) {
	e := config.ValidationError{
		Path:    "versioning.strategy",
		Message: `invalid value "semvr"`,
		Hint:    "must be one of: semver, calver",
	}
	got := e.Error()
	assert.Contains(t, got, "versioning.strategy")
	assert.Contains(t, got, "semvr")
	assert.Contains(t, got, "hint:")
	assert.Contains(t, got, "semver, calver")
}

func TestValidationError_Error_NoHint(t *testing.T) {
	e := config.ValidationError{
		Path:    "versioning.strategy",
		Message: "required field missing",
	}
	got := e.Error()
	assert.Contains(t, got, "versioning.strategy")
	assert.Contains(t, got, "required field missing")
	assert.NotContains(t, got, "hint:")
}

func TestValidationErrors_Error_Multiple(t *testing.T) {
	errs := config.ValidationErrors{
		{Path: "a", Message: "err1"},
		{Path: "b", Message: "err2"},
	}
	got := errs.Error()
	assert.Contains(t, got, "a: err1")
	assert.Contains(t, got, "b: err2")
}

func TestValidationErrors_Error_Empty(t *testing.T) {
	var errs config.ValidationErrors
	assert.Equal(t, "", errs.Error())
}
