package parsers

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// XMLParser parses XML responses into records using token-based streaming.
//
// Behavior:
//   - Builds a nested map[string]interface{} tree from XML tokens.
//   - Stores element attributes with "@" prefix (e.g., @id, @type).
//   - Preserves namespace prefixes on element names (e.g., geo:point).
//   - Auto-converts repeated sibling elements into []interface{}.
//   - Unwraps CDATA sections to plain text strings.
//   - Uses records_path (dot-separated) to locate the array of records.
//   - If records_path is empty and root is a single element, wraps as a
//     single-record slice.
//   - Applies MaxRecords truncation before returning.
type XMLParser struct{}

// Parse implements RecordParser.
func (p *XMLParser) Parse(data []byte, config ParserConfig) ([]map[string]interface{}, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("xml parser: empty input")
	}

	// Parse the entire XML document into a nested map tree.
	root, rootName, err := parseXMLTree(data)
	if err != nil {
		return nil, fmt.Errorf("xml parser: %w", err)
	}

	// Wrap root into a map so path traversal works uniformly.
	doc := map[string]interface{}{rootName: root}

	if config.RecordsPath == "" {
		// No records_path: return the root element's children as a single record.
		rootMap, ok := root.(map[string]interface{})
		if !ok {
			// Root is a text-only element.
			rootMap = map[string]interface{}{"#text": root}
		}
		records := []map[string]interface{}{rootMap}
		return truncateRecords(records, config.MaxRecords), nil
	}

	// Navigate to the records_path.
	segments := splitPath(config.RecordsPath)
	var current interface{} = doc
	for _, seg := range segments {
		m, ok := current.(map[string]interface{})
		if !ok {
			return []map[string]interface{}{}, nil
		}
		val, exists := m[seg]
		if !exists {
			return []map[string]interface{}{}, nil
		}
		current = val
	}

	// current should be either a []interface{} (multiple items) or a
	// map[string]interface{} (single item).
	var records []map[string]interface{}
	switch v := current.(type) {
	case []interface{}:
		records = make([]map[string]interface{}, 0, len(v))
		for _, elem := range v {
			m, ok := elem.(map[string]interface{})
			if !ok {
				continue
			}
			records = append(records, m)
		}
	case map[string]interface{}:
		records = []map[string]interface{}{v}
	default:
		// Path resolves to a scalar; wrap it.
		records = []map[string]interface{}{{"#text": v}}
	}

	return truncateRecords(records, config.MaxRecords), nil
}

// stackFrame holds the in-progress state for a single XML element being parsed.
type stackFrame struct {
	name     string
	m        map[string]interface{}
	charData strings.Builder
}

// parseXMLTree decodes an XML document into a nested map[string]interface{}.
// Returns the root element value and its local name.
func parseXMLTree(data []byte) (interface{}, string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))

	// stack holds pointers to avoid copying strings.Builder values.
	var stack []*stackFrame
	var rootName string
	var rootValue interface{}

	for {
		tok, err := dec.Token()
		if err != nil {
			// io.EOF is the normal end-of-document signal.
			if err == io.EOF {
				break
			}
			return nil, "", fmt.Errorf("decode error: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			elemName := elementName(t.Name)
			if len(stack) == 0 {
				rootName = elemName
			}

			frame := &stackFrame{
				name: elemName,
				m:    make(map[string]interface{}),
			}
			// Store attributes with "@" prefix.
			for _, attr := range t.Attr {
				attrKey := "@" + elementName(attr.Name)
				frame.m[attrKey] = attr.Value
			}
			stack = append(stack, frame)

		case xml.EndElement:
			if len(stack) == 0 {
				break
			}

			// Pop the current frame.
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			// Determine the value for this element.
			charText := strings.TrimSpace(top.charData.String())
			var elemValue interface{}

			if len(top.m) == 0 {
				// Leaf element: text only (or empty).
				elemValue = charText
			} else {
				// Has child elements. Merge charData as "#text" if non-empty.
				if charText != "" {
					top.m["#text"] = charText
				}
				elemValue = top.m
			}

			if len(stack) == 0 {
				// This was the root element.
				rootValue = elemValue
			} else {
				// Merge into the parent.
				parent := stack[len(stack)-1]
				mergeChild(parent.m, top.name, elemValue)
			}

		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].charData.Write(t)
			}

		case xml.Comment:
			// Ignore comments.

		case xml.ProcInst:
			// Ignore processing instructions.

		case xml.Directive:
			// Ignore directives (DOCTYPE etc.).
		}
	}

	if rootName == "" {
		return nil, "", fmt.Errorf("no root element found")
	}

	return rootValue, rootName, nil
}

// wellKnownNS maps namespace URIs to their canonical short prefixes.
// This table covers the namespaces most commonly seen in RSS/Atom feeds.
var wellKnownNS = map[string]string{
	"http://www.georss.org/georss":               "georss",
	"http://purl.org/dc/elements/1.1/":           "dc",
	"http://purl.org/dc/elements/1.1":            "dc",
	"http://www.w3.org/2005/Atom":                "atom",
	"http://purl.org/rss/1.0/modules/content/":   "content",
	"http://purl.org/rss/1.0/modules/content":    "content",
	"http://search.yahoo.com/mrss/":              "media",
	"http://search.yahoo.com/mrss":               "media",
	"http://www.itunes.com/dtds/podcast-1.0.dtd": "itunes",
	"http://www.w3.org/1999/xhtml":               "xhtml",
	"http://www.w3.org/XML/1998/namespace":       "xml",
}

// elementName returns the qualified name for an XML name, preserving the
// namespace prefix if one was declared (e.g., "geo:point").
//
// Go's encoding/xml populates Name.Space with the namespace URI and Name.Local
// with the local part. The original prefix is not preserved. We reconstruct a
// canonical prefix by consulting a well-known namespace table first, then
// falling back to the last non-empty segment of the URI path.
func elementName(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	// Derive a short prefix from the namespace URI.
	prefix := nsPrefix(name.Space)
	if prefix == "" {
		return name.Local
	}
	return prefix + ":" + name.Local
}

// nsPrefix derives a short prefix from a namespace URI.
// Checks well-known namespace table first, then falls back to the URI's last path segment.
func nsPrefix(uri string) string {
	if p, ok := wellKnownNS[uri]; ok {
		return p
	}
	// Strip trailing slash and take the last segment after '/' or '#'.
	trimmed := strings.TrimRight(uri, "/")
	for i := len(trimmed) - 1; i >= 0; i-- {
		if trimmed[i] == '/' || trimmed[i] == '#' {
			seg := trimmed[i+1:]
			if seg != "" {
				return seg
			}
		}
	}
	return trimmed
}

// mergeChild adds a child value to a parent map.
// If the key already exists, it converts the value to a []interface{} slice
// (or appends to an existing slice) to handle repeated sibling elements.
func mergeChild(parent map[string]interface{}, key string, value interface{}) {
	existing, exists := parent[key]
	if !exists {
		parent[key] = value
		return
	}

	// Key already exists: promote to/append to a slice.
	switch ev := existing.(type) {
	case []interface{}:
		parent[key] = append(ev, value)
	default:
		parent[key] = []interface{}{ev, value}
	}
}
