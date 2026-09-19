package enrichment

import "fmt"

// SchemaKey is the schema-registry key for an enrichment operation's output
// schema: "<source_name>.<operation_name>". It is the single source of truth for
// this format, used by BOTH the worker (schema lookup) and the startup schema
// registration, so the two can never drift (a mismatch silently fails every
// enrichment with "schema not found").
func SchemaKey(sourceName, operationName string) string {
	return fmt.Sprintf("%s.%s", sourceName, operationName)
}
