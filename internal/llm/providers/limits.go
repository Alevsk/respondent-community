package providers

// maxLLMResponseBytes caps the amount of data read from any LLM provider
// HTTP response body. 10 MB is generous for text completions while preventing
// unbounded memory consumption from a misbehaving upstream.
const maxLLMResponseBytes = 10 * 1024 * 1024 // 10 MB
