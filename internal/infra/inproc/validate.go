package inproc

import "fmt"

// ValidateAIPipeline asserts at boot that a worker subscribing with
// WildcardFilter(root) will receive the concrete subjects the publisher emits as
// "{root}.{name}". It is a tripwire against subject-matching regressions: if the
// bus ever reverts to exact matching, SubjectMatches(filter, "{root}.x") returns
// false and boot fails loudly instead of silently dropping every job.
func ValidateAIPipeline(root string) error {
	filter := WildcardFilter(root)
	sample := root + ".__healthcheck__"
	if !SubjectMatches(filter, sample) {
		return fmt.Errorf("inproc: AI pipeline subject drift — filter %q does not match published subject %q", filter, sample)
	}
	return nil
}
