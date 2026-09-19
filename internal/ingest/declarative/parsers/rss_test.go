package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rss20Feed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
	xmlns:georss="http://www.georss.org/georss"
	xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <title>Test News Feed</title>
    <link>https://example.com</link>
    <description>A test RSS 2.0 feed</description>
    <item>
      <title>Breaking News</title>
      <link>https://example.com/1</link>
      <description>First item description</description>
      <guid>guid-001</guid>
      <dc:creator>John Doe</dc:creator>
      <georss:point>45.0 -93.0</georss:point>
    </item>
    <item>
      <title>Second Story</title>
      <link>https://example.com/2</link>
      <description>Second item description</description>
      <guid>guid-002</guid>
      <dc:creator>Jane Smith</dc:creator>
    </item>
  </channel>
</rss>`

const atomFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Test Feed</title>
  <id>https://example.com/atom</id>
  <entry>
    <title>Atom Entry One</title>
    <id>entry-id-001</id>
    <summary>Summary of entry one</summary>
    <link href="https://example.com/atom/1"/>
  </entry>
</feed>`

func TestRSSParser_RSS20_AutoDetection(t *testing.T) {
	parser := &RSSParser{}
	cfg := ParserConfig{}

	records, err := parser.Parse([]byte(rss20Feed), cfg)
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "Breaking News", records[0]["title"])
	assert.Equal(t, "Second Story", records[1]["title"])
}

func TestRSSParser_RSS20_GeoRSSSplitting(t *testing.T) {
	parser := &RSSParser{}
	cfg := ParserConfig{}

	records, err := parser.Parse([]byte(rss20Feed), cfg)
	require.NoError(t, err)
	require.Len(t, records, 2)

	// First item has georss:point -> should be split.
	assert.Equal(t, 45.0, records[0]["georss_lat"])
	assert.Equal(t, -93.0, records[0]["georss_lon"])
}

func TestRSSParser_RSS20_NamespaceNormalization(t *testing.T) {
	parser := &RSSParser{}
	cfg := ParserConfig{}

	records, err := parser.Parse([]byte(rss20Feed), cfg)
	require.NoError(t, err)
	require.Len(t, records, 2)

	// dc:creator should be normalized to "creator".
	assert.Equal(t, "John Doe", records[0]["creator"])
	assert.Equal(t, "Jane Smith", records[1]["creator"])
}

func TestRSSParser_RSS20_StandardFields(t *testing.T) {
	parser := &RSSParser{}
	cfg := ParserConfig{}

	records, err := parser.Parse([]byte(rss20Feed), cfg)
	require.NoError(t, err)
	require.Len(t, records, 2)

	assert.Equal(t, "https://example.com/1", records[0]["link"])
	assert.Equal(t, "guid-001", records[0]["guid"])
	assert.Equal(t, "First item description", records[0]["description"])
}

func TestRSSParser_Atom_AutoDetection(t *testing.T) {
	parser := &RSSParser{}
	cfg := ParserConfig{}

	records, err := parser.Parse([]byte(atomFeed), cfg)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "Atom Entry One", records[0]["title"])
	assert.Equal(t, "entry-id-001", records[0]["id"])
}

func TestRSSParser_EmptyInput(t *testing.T) {
	parser := &RSSParser{}
	cfg := ParserConfig{}

	_, err := parser.Parse([]byte{}, cfg)
	require.Error(t, err)
}

func TestRSSParser_MaxRecords(t *testing.T) {
	parser := &RSSParser{}
	cfg := ParserConfig{MaxRecords: 1}

	records, err := parser.Parse([]byte(rss20Feed), cfg)
	require.NoError(t, err)
	assert.Len(t, records, 1)
}

func TestRSSParser_NoGeoRSS_NoSplitFields(t *testing.T) {
	parser := &RSSParser{}
	cfg := ParserConfig{}

	records, err := parser.Parse([]byte(rss20Feed), cfg)
	require.NoError(t, err)
	require.Len(t, records, 2)

	// Second item has no georss:point, so no split fields.
	_, hasLat := records[1]["georss_lat"]
	_, hasLon := records[1]["georss_lon"]
	assert.False(t, hasLat, "expected no georss_lat on item without georss:point")
	assert.False(t, hasLon, "expected no georss_lon on item without georss:point")
}

func TestRSSParser_UnknownFormat(t *testing.T) {
	parser := &RSSParser{}
	cfg := ParserConfig{}

	_, err := parser.Parse([]byte(`<html><body>Not RSS</body></html>`), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized feed format")
}
