package inproc_test

import (
	"testing"

	"github.com/Alevsk/respondent/internal/infra/inproc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAIPipeline_OK(t *testing.T) {
	require.NoError(t, inproc.ValidateAIPipeline(inproc.EnrichJobRoot))
}

func TestWildcardFilter(t *testing.T) {
	assert.Equal(t, "respondent.ai.enrich.>", inproc.WildcardFilter(inproc.EnrichJobRoot))
	// The filter must match a representative published subject — the guarantee
	// ValidateAIPipeline pins at boot.
	assert.True(t, inproc.SubjectMatches(inproc.WildcardFilter(inproc.EnrichJobRoot), inproc.EnrichJobRoot+".adsb_military"))
}
