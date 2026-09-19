package declarative

import (
	"strings"
	"testing"
	"time"
)

func TestFTPSFTPTransport_RegisteredInFactory(t *testing.T) {
	transportMu.RLock()
	_, ok := transportConstructors["ftp_sftp"]
	transportMu.RUnlock()
	if !ok {
		t.Fatal("ftp_sftp transport not registered in factory")
	}
}

func TestFTPSFTPTransport_NewFTPSFTPTransport_ValidFTP(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ftp",
		Transport: TransportSpec{
			Type:     "ftp_sftp",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			FTPSFTP: &FTPSFTPSpec{
				Protocol:    "ftp",
				Host:        "ftp.example.com",
				Path:        "/data/feeds",
				FilePattern: `\.csv$`,
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ft := st.(*FTPSFTPTransport)
	if ft.spec.Protocol != "ftp" {
		t.Errorf("expected protocol ftp, got %s", ft.spec.Protocol)
	}
	if ft.spec.Host != "ftp.example.com" {
		t.Errorf("expected host ftp.example.com, got %s", ft.spec.Host)
	}
	if ft.spec.Path != "/data/feeds" {
		t.Errorf("expected path /data/feeds, got %s", ft.spec.Path)
	}
	if ft.sourceName != "test_ftp" {
		t.Errorf("expected source name test_ftp, got %s", ft.sourceName)
	}
	if ft.spec.FilePattern != `\.csv$` {
		t.Errorf("expected file_pattern '\\.csv$', got %s", ft.spec.FilePattern)
	}
}

func TestFTPSFTPTransport_NewFTPSFTPTransport_ValidSFTP(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_sftp",
		Transport: TransportSpec{
			Type:     "ftp_sftp",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			FTPSFTP: &FTPSFTPSpec{
				Protocol:         "sftp",
				Host:             "sftp.example.com:2222",
				Path:             "/uploads",
				Username:         "SFTP_USER",
				Password:         "SFTP_PASS",
				DeleteAfterFetch: true,
			},
		},
	}

	envResolve := func(name string) string {
		switch name {
		case "SFTP_USER":
			return "resolved-user"
		case "SFTP_PASS":
			return "resolved-pass"
		default:
			return ""
		}
	}

	st, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ft := st.(*FTPSFTPTransport)
	if ft.spec.Protocol != "sftp" {
		t.Errorf("expected protocol sftp, got %s", ft.spec.Protocol)
	}
	if ft.username != "resolved-user" {
		t.Errorf("expected username resolved-user, got %s", ft.username)
	}
	if ft.password != "resolved-pass" {
		t.Errorf("expected password resolved-pass, got %s", ft.password)
	}
	if !ft.spec.DeleteAfterFetch {
		t.Error("expected DeleteAfterFetch=true")
	}
}

func TestFTPSFTPTransport_NewFTPSFTPTransport_NilSpec(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ftp",
		Transport: TransportSpec{
			Type: "ftp_sftp",
			// FTPSFTP is nil
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for nil FTP/SFTP spec, got nil")
	}
	if !strings.Contains(err.Error(), "transport.ftp_sftp configuration") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFTPSFTPTransport_NewFTPSFTPTransport_MissingHost(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ftp",
		Transport: TransportSpec{
			Type: "ftp_sftp",
			FTPSFTP: &FTPSFTPSpec{
				Protocol: "ftp",
				Host:     "",
				Path:     "/data",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for missing host")
	}
	if !strings.Contains(err.Error(), "host") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFTPSFTPTransport_NewFTPSFTPTransport_MissingPath(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ftp",
		Transport: TransportSpec{
			Type: "ftp_sftp",
			FTPSFTP: &FTPSFTPSpec{
				Protocol: "ftp",
				Host:     "ftp.example.com",
				Path:     "",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	if !strings.Contains(err.Error(), "path") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFTPSFTPTransport_NewFTPSFTPTransport_InvalidProtocol(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ftp",
		Transport: TransportSpec{
			Type: "ftp_sftp",
			FTPSFTP: &FTPSFTPSpec{
				Protocol: "ftps",
				Host:     "ftp.example.com",
				Path:     "/data",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for invalid protocol")
	}
	if !strings.Contains(err.Error(), "protocol") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFTPSFTPTransport_TrackSeenDefaultsTrue(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ftp",
		Transport: TransportSpec{
			Type: "ftp_sftp",
			FTPSFTP: &FTPSFTPSpec{
				Protocol: "ftp",
				Host:     "ftp.example.com",
				Path:     "/data",
				// TrackSeen is nil -- should default to true
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ft := st.(*FTPSFTPTransport)
	if !ft.trackSeen {
		t.Error("expected trackSeen=true when TrackSeen is nil")
	}
}

func TestFTPSFTPTransport_TrackSeenExplicitFalse(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ftp",
		Transport: TransportSpec{
			Type: "ftp_sftp",
			FTPSFTP: &FTPSFTPSpec{
				Protocol:  "ftp",
				Host:      "ftp.example.com",
				Path:      "/data",
				TrackSeen: ptrBool(false),
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ft := st.(*FTPSFTPTransport)
	if ft.trackSeen {
		t.Error("expected trackSeen=false when TrackSeen is explicitly false")
	}
}

func TestFTPSFTPTransport_TrackSeenExplicitTrue(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ftp",
		Transport: TransportSpec{
			Type: "ftp_sftp",
			FTPSFTP: &FTPSFTPSpec{
				Protocol:  "ftp",
				Host:      "ftp.example.com",
				Path:      "/data",
				TrackSeen: ptrBool(true),
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ft := st.(*FTPSFTPTransport)
	if !ft.trackSeen {
		t.Error("expected trackSeen=true when TrackSeen is explicitly true")
	}
}

func TestFTPSFTPTransport_CredentialResolution(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_sftp",
		Transport: TransportSpec{
			Type: "ftp_sftp",
			FTPSFTP: &FTPSFTPSpec{
				Protocol:   "sftp",
				Host:       "sftp.example.com",
				Path:       "/data",
				Username:   "FTP_USER",
				Password:   "FTP_PASS",
				PrivateKey: "FTP_KEY",
			},
		},
	}

	envResolve := func(name string) string {
		switch name {
		case "FTP_USER":
			return "resolved-user"
		case "FTP_PASS":
			return "resolved-pass"
		case "FTP_KEY":
			return "resolved-key-data"
		default:
			return ""
		}
	}

	st, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ft := st.(*FTPSFTPTransport)
	if ft.username != "resolved-user" {
		t.Errorf("expected username resolved-user, got %s", ft.username)
	}
	if ft.password != "resolved-pass" {
		t.Errorf("expected password resolved-pass, got %s", ft.password)
	}
	if ft.privateKey != "resolved-key-data" {
		t.Errorf("expected privateKey resolved-key-data, got %s", ft.privateKey)
	}
}

func TestFTPSFTPTransport_CredentialResolution_NoCredentials(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ftp",
		Transport: TransportSpec{
			Type: "ftp_sftp",
			FTPSFTP: &FTPSFTPSpec{
				Protocol: "ftp",
				Host:     "ftp.example.com",
				Path:     "/data",
				// No username, password, or private key
			},
		},
	}

	envResolve := func(name string) string { return "should-not-be-called" }

	st, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ft := st.(*FTPSFTPTransport)
	if ft.username != "" {
		t.Errorf("expected empty username, got %s", ft.username)
	}
	if ft.password != "" {
		t.Errorf("expected empty password, got %s", ft.password)
	}
	if ft.privateKey != "" {
		t.Errorf("expected empty privateKey, got %s", ft.privateKey)
	}
}

func TestFTPSFTPTransport_EnsurePort(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		defaultPort string
		want        string
	}{
		{
			name:        "no_port_ftp",
			host:        "ftp.example.com",
			defaultPort: "21",
			want:        "ftp.example.com:21",
		},
		{
			name:        "no_port_sftp",
			host:        "sftp.example.com",
			defaultPort: "22",
			want:        "sftp.example.com:22",
		},
		{
			name:        "with_port",
			host:        "ftp.example.com:2121",
			defaultPort: "21",
			want:        "ftp.example.com:2121",
		},
		{
			name:        "ipv4_no_port",
			host:        "192.168.1.1",
			defaultPort: "21",
			want:        "192.168.1.1:21",
		},
		{
			name:        "ipv4_with_port",
			host:        "192.168.1.1:990",
			defaultPort: "21",
			want:        "192.168.1.1:990",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ensurePort(tt.host, tt.defaultPort)
			if got != tt.want {
				t.Errorf("ensurePort(%q, %q) = %q, want %q", tt.host, tt.defaultPort, got, tt.want)
			}
		})
	}
}

func TestFTPSFTPTransport_ConstructorValidation(t *testing.T) {
	tests := []struct {
		name    string
		def     *SourceDefinition
		wantSub string
	}{
		{
			name: "nil_spec",
			def: &SourceDefinition{
				Name:      "test_ftp",
				Transport: TransportSpec{Type: "ftp_sftp"},
			},
			wantSub: "transport.ftp_sftp configuration",
		},
		{
			name: "empty_host",
			def: &SourceDefinition{
				Name: "test_ftp",
				Transport: TransportSpec{
					Type:    "ftp_sftp",
					FTPSFTP: &FTPSFTPSpec{Protocol: "ftp", Path: "/data"},
				},
			},
			wantSub: "host",
		},
		{
			name: "empty_path",
			def: &SourceDefinition{
				Name: "test_ftp",
				Transport: TransportSpec{
					Type:    "ftp_sftp",
					FTPSFTP: &FTPSFTPSpec{Protocol: "ftp", Host: "ftp.example.com"},
				},
			},
			wantSub: "path",
		},
		{
			name: "invalid_protocol",
			def: &SourceDefinition{
				Name: "test_ftp",
				Transport: TransportSpec{
					Type:    "ftp_sftp",
					FTPSFTP: &FTPSFTPSpec{Protocol: "http", Host: "ftp.example.com", Path: "/data"},
				},
			},
			wantSub: "protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envResolve := func(name string) string { return "" }
			_, err := newFTPSFTPTransport(tt.def, nil, nil, envResolve, testLogger())
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestFTPSFTPTransport_DeleteAfterFetchStored(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ftp",
		Transport: TransportSpec{
			Type: "ftp_sftp",
			FTPSFTP: &FTPSFTPSpec{
				Protocol:         "ftp",
				Host:             "ftp.example.com",
				Path:             "/data",
				DeleteAfterFetch: true,
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ft := st.(*FTPSFTPTransport)
	if !ft.spec.DeleteAfterFetch {
		t.Error("expected DeleteAfterFetch=true")
	}
}

func TestFTPSFTPTransport_FilePatternStored(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ftp",
		Transport: TransportSpec{
			Type: "ftp_sftp",
			FTPSFTP: &FTPSFTPSpec{
				Protocol:    "ftp",
				Host:        "ftp.example.com",
				Path:        "/data",
				FilePattern: `^report_\d{8}\.json$`,
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newFTPSFTPTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ft := st.(*FTPSFTPTransport)
	if ft.spec.FilePattern != `^report_\d{8}\.json$` {
		t.Errorf("expected file_pattern '^report_\\d{8}\\.json$', got %s", ft.spec.FilePattern)
	}
}
