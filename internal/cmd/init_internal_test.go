package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintDroppedFieldsWarning_Empty(t *testing.T) {
	var buf bytes.Buffer
	printDroppedFieldsWarning(&buf, nil)
	assert.Empty(t, buf.String())
}

func TestPrintDroppedFieldsWarning_ListsEachField(t *testing.T) {
	var buf bytes.Buffer
	printDroppedFieldsWarning(&buf, []string{"tickets", "environments.prod.release"})
	out := buf.String()
	assert.Contains(t, out, "tickets")
	assert.Contains(t, out, "environments.prod.release")
}
