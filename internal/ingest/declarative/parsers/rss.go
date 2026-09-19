package parsers

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// RSSParser parses RSS 2.0 and Atom feeds into records.
//
// Behavior:
//   - Auto-detects feed format by inspecting the root element:
//   - Root "rss" → RSS 2.0 (records_path: "rss.channel.item")
//   - Root "feed" → Atom   (records_path: "feed.entry")
//   - Any other root element → error: "unrecognized feed format"
//   - Delegates XML parsing to XMLParser with the detected records_path.
//   - Post-processes records to normalize namespace prefixes:
//   - "dc:" prefix → stripped (e.g., "dc:creator" → "creator")
//   - "georss:" prefix → "georss_" (e.g., "georss:point" → "georss_point")
//   - "content:" prefix → "content_" (e.g., "content:encoded" → "content_encoded")
//   - "media:" prefix → "media_" (e.g., "media:thumbnail" → "media_thumbnail")
//   - Splits "georss_point" ("lat lon") into float64 fields georss_lat and georss_lon.
//   - Applies MaxRecords truncation (delegated to XMLParser).
type RSSParser struct{}

// Parse implements RecordParser.
func (p *RSSParser) Parse(data []byte, config ParserConfig) ([]map[string]interface{}, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("rss parser: empty input")
	}

	// Detect the root element name to determine feed format.
	rootName, err := detectRootElement(data)
	if err != nil {
		return nil, fmt.Errorf("rss parser: %w", err)
	}

	var recordsPath string
	switch rootName {
	case "rss":
		recordsPath = "rss.channel.item"
	case "feed":
		// Atom feeds: the root is "feed" (possibly namespace-qualified).
		// After XMLParser processes it, the root key may have a namespace prefix.
		// We detect the raw local name "feed" here and set the path accordingly.
		// The XMLParser will key the root under its qualified name. We need to
		// resolve this after parsing. We pass an empty config.RecordsPath and
		// find items manually.
		recordsPath = ""
	default:
		return nil, fmt.Errorf("rss parser: unrecognized feed format: root element %q", rootName)
	}

	xmlParser := &XMLParser{}

	if rootName == "feed" {
		return p.parseAtom(xmlParser, data, config)
	}

	xmlCfg := ParserConfig{
		RecordsPath: recordsPath,
		MaxRecords:  config.MaxRecords,
	}

	records, err := xmlParser.Parse(data, xmlCfg)
	if err != nil {
		return nil, fmt.Errorf("rss parser: %w", err)
	}

	// Post-process: normalize namespace prefixes and split georss_point.
	for i, rec := range records {
		records[i] = normalizeNamespaces(rec)
		records[i] = splitGeoRSSPoint(records[i])
	}

	return records, nil
}

// parseAtom handles Atom feeds. Because the Atom root element "feed" is in
// the http://www.w3.org/2005/Atom namespace, the XMLParser will key all
// elements with the "atom:" prefix (derived from the well-known namespace table).
// We parse the whole document without a records_path and locate entries by
// looking for the "atom:entry" key (or plain "entry") in the root map.
func (p *RSSParser) parseAtom(xmlParser *XMLParser, data []byte, config ParserConfig) ([]map[string]interface{}, error) {
	// Parse without records_path to get the root element's children.
	xmlCfg := ParserConfig{MaxRecords: 0} // We truncate after post-processing.
	rootRecords, err := xmlParser.Parse(data, xmlCfg)
	if err != nil {
		return nil, fmt.Errorf("rss parser (atom): %w", err)
	}

	if len(rootRecords) == 0 {
		return []map[string]interface{}{}, nil
	}

	// rootRecords[0] is the root element's children map (the feed element's contents).
	rootMap := rootRecords[0]

	// Find the "entry" key. It may be "atom:entry" or plain "entry".
	var entries []map[string]interface{}
	for key, val := range rootMap {
		localKey := stripNSPrefix(key)
		if localKey == "entry" {
			switch v := val.(type) {
			case map[string]interface{}:
				entries = append(entries, v)
			case []interface{}:
				for _, elem := range v {
					if m, ok := elem.(map[string]interface{}); ok {
						entries = append(entries, m)
					}
				}
			}
			break
		}
	}

	// Post-process each entry: normalize namespace prefixes and split georss_point.
	for i, rec := range entries {
		entries[i] = normalizeNamespaces(rec)
		entries[i] = splitGeoRSSPoint(entries[i])
	}

	return truncateRecords(entries, config.MaxRecords), nil
}

// detectRootElement reads just enough of the XML to find the root element name,
// returning its local name (without namespace prefix) for format detection.
func detectRootElement(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", fmt.Errorf("could not read root element: %w", err)
		}
		if t, ok := tok.(xml.StartElement); ok {
			return t.Name.Local, nil
		}
	}
}

// normalizeNamespaces renames keys in a record according to RSS namespace conventions:
//   - "dc:X"      → "X"          (Dublin Core: strip prefix entirely)
//   - "georss:X"  → "georss_X"   (GeoRSS: replace colon with underscore)
//   - "content:X" → "content_X"  (Content module)
//   - "media:X"   → "media_X"    (Media RSS)
//   - "atom:X"    → "X"          (Atom namespace used as default: strip prefix)
//   - "itunes:X"  → "itunes_X"   (iTunes podcast extension)
//
// Keys without a recognized namespace prefix are left unchanged.
func normalizeNamespaces(rec map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(rec))
	for k, v := range rec {
		out[normalizeKey(k)] = v
	}
	return out
}

// normalizeKey applies the namespace normalization rules to a single key.
func normalizeKey(k string) string {
	// Prefixes to strip entirely (map to their local name only).
	stripPrefixes := []string{"dc:", "atom:"}
	for _, p := range stripPrefixes {
		if strings.HasPrefix(k, p) {
			return k[len(p):]
		}
	}
	// Prefixes to convert to "prefix_local" style.
	for _, ns := range []struct{ prefix, replacement string }{
		{"georss:", "georss_"},
		{"content:", "content_"},
		{"media:", "media_"},
		{"itunes:", "itunes_"},
	} {
		if strings.HasPrefix(k, ns.prefix) {
			return ns.replacement + k[len(ns.prefix):]
		}
	}
	return k
}

// stripNSPrefix removes any "prefix:" portion from a key, returning just the local name.
func stripNSPrefix(k string) string {
	if i := strings.Index(k, ":"); i >= 0 {
		return k[i+1:]
	}
	return k
}

// splitGeoRSSPoint splits a "georss_point" field ("lat lon") into float64
// fields "georss_lat" and "georss_lon". The original "georss_point" key is
// removed on success. If the field is absent or malformed, the record is
// returned unchanged.
func splitGeoRSSPoint(rec map[string]interface{}) map[string]interface{} {
	raw, ok := rec["georss_point"]
	if !ok {
		return rec
	}

	s, ok := raw.(string)
	if !ok {
		return rec
	}

	parts := strings.Fields(s)
	if len(parts) != 2 {
		return rec
	}

	lat, err1 := strconv.ParseFloat(parts[0], 64)
	lon, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil {
		return rec
	}

	delete(rec, "georss_point")
	rec["georss_lat"] = lat
	rec["georss_lon"] = lon
	return rec
}
