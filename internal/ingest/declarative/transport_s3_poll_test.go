package declarative

import (
	"strings"
	"testing"
)

func TestS3PollTransport_RegisteredInFactory(t *testing.T) {
	transportMu.RLock()
	_, ok := transportConstructors["s3_poll"]
	transportMu.RUnlock()
	if !ok {
		t.Fatal("s3_poll transport not registered in factory")
	}
}

func TestS3PollTransport_NilSpec(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_s3",
		Transport: TransportSpec{
			Type: "s3_poll",
			// S3Poll is nil
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newS3PollTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for nil S3Poll spec, got nil")
	}
	if !strings.Contains(err.Error(), "transport.s3_poll configuration") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestS3PollTransport_MissingEndpoint(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_s3",
		Transport: TransportSpec{
			Type: "s3_poll",
			S3Poll: &S3PollSpec{
				Endpoint: "",
				Bucket:   "my-bucket",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newS3PollTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for empty endpoint")
	}
	if !strings.Contains(err.Error(), "endpoint") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestS3PollTransport_MissingBucket(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_s3",
		Transport: TransportSpec{
			Type: "s3_poll",
			S3Poll: &S3PollSpec{
				Endpoint: "s3.example.com",
				Bucket:   "",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newS3PollTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for empty bucket")
	}
	if !strings.Contains(err.Error(), "bucket") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestS3PollTransport_DefaultRegion(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_s3",
		Transport: TransportSpec{
			Type: "s3_poll",
			S3Poll: &S3PollSpec{
				Endpoint: "s3.example.com",
				Bucket:   "my-bucket",
				// Region not set -- should default to us-east-1
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newS3PollTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s3t := st.(*S3PollTransport)
	if s3t.region != defaultS3Region {
		t.Errorf("expected region %q, got %q", defaultS3Region, s3t.region)
	}
}

func TestS3PollTransport_CustomRegion(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_s3",
		Transport: TransportSpec{
			Type: "s3_poll",
			S3Poll: &S3PollSpec{
				Endpoint: "s3.example.com",
				Bucket:   "my-bucket",
				Region:   "eu-west-1",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newS3PollTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s3t := st.(*S3PollTransport)
	if s3t.region != "eu-west-1" {
		t.Errorf("expected region eu-west-1, got %q", s3t.region)
	}
}

func TestS3PollTransport_CredentialResolution(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_s3",
		Transport: TransportSpec{
			Type: "s3_poll",
			S3Poll: &S3PollSpec{
				Endpoint:  "s3.example.com",
				Bucket:    "my-bucket",
				AccessKey: "S3_ACCESS_KEY",
				SecretKey: "S3_SECRET_KEY",
			},
		},
	}

	envResolve := func(name string) string {
		switch name {
		case "S3_ACCESS_KEY":
			return "resolved-access-key"
		case "S3_SECRET_KEY":
			return "resolved-secret-key"
		default:
			return ""
		}
	}

	st, err := newS3PollTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s3t := st.(*S3PollTransport)
	if s3t.accessKey != "resolved-access-key" {
		t.Errorf("expected resolved access key, got %q", s3t.accessKey)
	}
	if s3t.secretKey != "resolved-secret-key" {
		t.Errorf("expected resolved secret key, got %q", s3t.secretKey)
	}
}

func TestS3PollTransport_EmptyCredentials(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_s3",
		Transport: TransportSpec{
			Type: "s3_poll",
			S3Poll: &S3PollSpec{
				Endpoint: "s3.example.com",
				Bucket:   "my-bucket",
				// No credentials -- anonymous access
			},
		},
	}

	envResolve := func(name string) string { return "should-not-be-called" }

	st, err := newS3PollTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s3t := st.(*S3PollTransport)
	if s3t.accessKey != "" {
		t.Errorf("expected empty access key for anonymous access, got %q", s3t.accessKey)
	}
	if s3t.secretKey != "" {
		t.Errorf("expected empty secret key for anonymous access, got %q", s3t.secretKey)
	}
}

func TestS3PollTransport_ValidConfig(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_s3_source",
		Transport: TransportSpec{
			Type: "s3_poll",
			S3Poll: &S3PollSpec{
				Endpoint:         "minio.example.com:9000",
				Bucket:           "data-bucket",
				Prefix:           "feeds/",
				Region:           "us-west-2",
				AccessKey:        "AK",
				SecretKey:        "SK",
				DeleteAfterFetch: true,
				FilePattern:      "*.json",
				PathStyle:        true,
			},
		},
	}

	envResolve := func(name string) string {
		switch name {
		case "AK":
			return "my-access-key"
		case "SK":
			return "my-secret-key"
		default:
			return ""
		}
	}

	st, err := newS3PollTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s3t := st.(*S3PollTransport)
	if s3t.sourceName != "test_s3_source" {
		t.Errorf("expected source name test_s3_source, got %s", s3t.sourceName)
	}
	if s3t.spec.Bucket != "data-bucket" {
		t.Errorf("expected bucket data-bucket, got %s", s3t.spec.Bucket)
	}
	if s3t.spec.Prefix != "feeds/" {
		t.Errorf("expected prefix feeds/, got %s", s3t.spec.Prefix)
	}
	if s3t.region != "us-west-2" {
		t.Errorf("expected region us-west-2, got %s", s3t.region)
	}
	if !s3t.spec.DeleteAfterFetch {
		t.Error("expected DeleteAfterFetch to be true")
	}
	if s3t.spec.FilePattern != "*.json" {
		t.Errorf("expected file pattern *.json, got %s", s3t.spec.FilePattern)
	}
	if !s3t.spec.PathStyle {
		t.Error("expected PathStyle to be true")
	}
	if s3t.client == nil {
		t.Error("expected non-nil S3 client")
	}
}

func TestS3PollTransport_ConstructorValidation(t *testing.T) {
	tests := []struct {
		name    string
		def     *SourceDefinition
		wantSub string
	}{
		{
			name: "nil_s3_poll_spec",
			def: &SourceDefinition{
				Name:      "test_s3",
				Transport: TransportSpec{Type: "s3_poll"},
			},
			wantSub: "transport.s3_poll configuration",
		},
		{
			name: "empty_endpoint",
			def: &SourceDefinition{
				Name: "test_s3",
				Transport: TransportSpec{
					Type:   "s3_poll",
					S3Poll: &S3PollSpec{Bucket: "b"},
				},
			},
			wantSub: "endpoint",
		},
		{
			name: "empty_bucket",
			def: &SourceDefinition{
				Name: "test_s3",
				Transport: TransportSpec{
					Type:   "s3_poll",
					S3Poll: &S3PollSpec{Endpoint: "s3.example.com"},
				},
			},
			wantSub: "bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envResolve := func(name string) string { return "" }
			_, err := newS3PollTransport(tt.def, nil, nil, envResolve, testLogger())
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestS3PollTransport_BucketLookup(t *testing.T) {
	tests := []struct {
		name      string
		pathStyle bool
		want      string
	}{
		{
			name:      "path_style",
			pathStyle: true,
			want:      "path",
		},
		{
			name:      "virtual_hosted",
			pathStyle: false,
			want:      "auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &SourceDefinition{
				Name: "test_s3",
				Transport: TransportSpec{
					Type: "s3_poll",
					S3Poll: &S3PollSpec{
						Endpoint:  "s3.example.com",
						Bucket:    "my-bucket",
						PathStyle: tt.pathStyle,
					},
				},
			}

			envResolve := func(name string) string { return "" }

			st, err := newS3PollTransport(def, nil, nil, envResolve, testLogger())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			s3t := st.(*S3PollTransport)
			if s3t.spec.PathStyle != tt.pathStyle {
				t.Errorf("expected PathStyle %v, got %v", tt.pathStyle, s3t.spec.PathStyle)
			}
		})
	}
}

func TestS3PollTransport_ResolveCredential(t *testing.T) {
	tests := []struct {
		name  string
		value string
		env   map[string]string
		want  string
	}{
		{
			name:  "empty_value",
			value: "",
			env:   map[string]string{"ANY": "val"},
			want:  "",
		},
		{
			name:  "resolved_value",
			value: "MY_KEY",
			env:   map[string]string{"MY_KEY": "secret123"},
			want:  "secret123",
		},
		{
			name:  "unresolved_value",
			value: "MISSING_KEY",
			env:   map[string]string{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := func(name string) string { return tt.env[name] }
			got := resolveCredential(tt.value, resolver)
			if got != tt.want {
				t.Errorf("resolveCredential(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
