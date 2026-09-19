package providers

import "net/http"

// HTTPDoer abstracts HTTP request execution. Defined in the consumer
// package (providers) following DIP so providers do not depend on a
// concrete *http.Client.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}
