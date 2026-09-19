package parsers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXMLParser_SimpleElements(t *testing.T) {
	input := []byte(`<root><item><title>First</title><value>100</value></item><item><title>Second</title><value>200</value></item></root>`)
	parser := &XMLParser{}
	cfg := ParserConfig{RecordsPath: "root.item"}

	records, err := parser.Parse(input, cfg)
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "First", records[0]["title"])
	assert.Equal(t, "100", records[0]["value"])
	assert.Equal(t, "Second", records[1]["title"])
	assert.Equal(t, "200", records[1]["value"])
}

func TestXMLParser_Attributes(t *testing.T) {
	input := []byte(`<root><item id="5" type="news"><title>Test</title></item></root>`)
	parser := &XMLParser{}
	cfg := ParserConfig{RecordsPath: "root.item"}

	records, err := parser.Parse(input, cfg)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "5", records[0]["@id"])
	assert.Equal(t, "news", records[0]["@type"])
	assert.Equal(t, "Test", records[0]["title"])
}

func TestXMLParser_NamespacePrefixes(t *testing.T) {
	input := []byte(`<root xmlns:geo="http://example.com/geo"><item><geo:point>45.0 -93.0</geo:point></item></root>`)
	parser := &XMLParser{}
	cfg := ParserConfig{RecordsPath: "root.item"}

	records, err := parser.Parse(input, cfg)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "45.0 -93.0", records[0]["geo:point"])
}

func TestXMLParser_CDATA(t *testing.T) {
	input := []byte(`<root><item><desc><![CDATA[<b>Bold</b> text]]></desc></item></root>`)
	parser := &XMLParser{}
	cfg := ParserConfig{RecordsPath: "root.item"}

	records, err := parser.Parse(input, cfg)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "<b>Bold</b> text", records[0]["desc"])
}

func TestXMLParser_RepeatedChildrenBecomeArray(t *testing.T) {
	input := []byte(`<root><item><tag>a</tag><tag>b</tag><tag>c</tag></item></root>`)
	parser := &XMLParser{}
	cfg := ParserConfig{RecordsPath: "root.item"}

	records, err := parser.Parse(input, cfg)
	require.NoError(t, err)
	require.Len(t, records, 1)

	tags, ok := records[0]["tag"].([]interface{})
	require.True(t, ok, "expected []interface{} for repeated tag elements, got %T", records[0]["tag"])
	assert.Len(t, tags, 3)
	assert.Equal(t, "a", tags[0])
	assert.Equal(t, "b", tags[1])
	assert.Equal(t, "c", tags[2])
}

func TestXMLParser_EmptyInput(t *testing.T) {
	parser := &XMLParser{}
	cfg := ParserConfig{RecordsPath: "root.item"}

	_, err := parser.Parse([]byte{}, cfg)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "empty input"), "expected 'empty input' in error, got: %v", err)
}

func TestXMLParser_MalformedXML(t *testing.T) {
	input := []byte(`<root><unclosed>`)
	parser := &XMLParser{}
	cfg := ParserConfig{RecordsPath: "root.item"}

	_, err := parser.Parse(input, cfg)
	require.Error(t, err)
}

func TestXMLParser_NoRecordsPath(t *testing.T) {
	input := []byte(`<root><title>Hello</title><value>42</value></root>`)
	parser := &XMLParser{}
	cfg := ParserConfig{RecordsPath: ""}

	records, err := parser.Parse(input, cfg)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "Hello", records[0]["title"])
	assert.Equal(t, "42", records[0]["value"])
}

func TestXMLParser_MaxRecords(t *testing.T) {
	input := []byte(`<root><item><title>A</title></item><item><title>B</title></item><item><title>C</title></item></root>`)
	parser := &XMLParser{}
	cfg := ParserConfig{RecordsPath: "root.item", MaxRecords: 2}

	records, err := parser.Parse(input, cfg)
	require.NoError(t, err)
	assert.Len(t, records, 2)
}

func TestXMLParser_MissingRecordsPath(t *testing.T) {
	input := []byte(`<root><other>data</other></root>`)
	parser := &XMLParser{}
	cfg := ParserConfig{RecordsPath: "root.item"}

	records, err := parser.Parse(input, cfg)
	require.NoError(t, err)
	assert.Empty(t, records)
}
