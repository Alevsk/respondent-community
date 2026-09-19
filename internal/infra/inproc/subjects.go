package inproc

// EnrichJobRoot is the subject prefix for AI enrichment jobs. The publisher
// emits "{EnrichJobRoot}.{sourceName}"; the worker subscribes WildcardFilter(EnrichJobRoot).
const EnrichJobRoot = "respondent.ai.enrich"

// WildcardFilter returns the NATS-style subscription filter matching every
// subject published under root.
func WildcardFilter(root string) string { return root + ".>" }
