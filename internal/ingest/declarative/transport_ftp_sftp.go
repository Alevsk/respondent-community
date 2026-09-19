package declarative

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/Alevsk/respondent/internal/logging"
)

// defaultFTPPort is the default port for FTP connections.
const defaultFTPPort = "21"

// defaultSFTPPort is the default port for SFTP connections.
const defaultSFTPPort = "22"

// defaultFTPTimeout is the connection timeout for FTP/SFTP.
const defaultFTPTimeout = 30 * time.Second

// maxFTPFileSize limits individual file reads to 100 MB.
const maxFTPFileSize = 100 * 1024 * 1024

// Compile-time assertion that FTPSFTPTransport implements Transport.
var _ Transport = (*FTPSFTPTransport)(nil)

func init() {
	RegisterTransport("ftp_sftp", newFTPSFTPTransport)
}

// FTPSFTPTransport implements Transport (pull-based) for FTP/SFTP file retrieval.
// On each Fetch call it connects to the server, lists files matching the configured
// pattern, downloads new (unseen) files, and returns their concatenated content.
type FTPSFTPTransport struct {
	spec       *FTPSFTPSpec
	sourceName string
	logger     *logging.Logger

	// Resolved credentials.
	username   string
	password   string
	privateKey string

	// Compiled file pattern (nil if no pattern configured).
	filePattern *regexp.Regexp

	// trackSeen controls whether we skip previously fetched filenames.
	trackSeen bool
	seenFiles sync.Map
}

// newFTPSFTPTransport creates an FTPSFTPTransport from a source definition.
func newFTPSFTPTransport(def *SourceDefinition, _ map[string]string, _ TokenProvider, envResolve EnvResolver, logger *logging.Logger) (Transport, error) {
	if def.Transport.FTPSFTP == nil {
		return nil, fmt.Errorf("ftp_sftp transport requires transport.ftp_sftp configuration")
	}

	spec := def.Transport.FTPSFTP

	if spec.Host == "" {
		return nil, fmt.Errorf("ftp_sftp transport requires host")
	}
	if spec.Path == "" {
		return nil, fmt.Errorf("ftp_sftp transport requires path")
	}
	if spec.Protocol != "ftp" && spec.Protocol != "sftp" {
		return nil, fmt.Errorf("ftp_sftp transport requires protocol to be 'ftp' or 'sftp', got %q", spec.Protocol)
	}

	// Resolve credentials via envResolve.
	var username, password, privateKey string
	if spec.Username != "" {
		username = envResolve(spec.Username)
	}
	if spec.Password != "" {
		password = envResolve(spec.Password)
	}
	if spec.PrivateKey != "" {
		privateKey = envResolve(spec.PrivateKey)
	}

	// TrackSeen defaults to true when nil.
	trackSeen := true
	if spec.TrackSeen != nil {
		trackSeen = *spec.TrackSeen
	}

	// Pre-compile file pattern once.
	var filePattern *regexp.Regexp
	if spec.FilePattern != "" {
		p, err := regexp.Compile(spec.FilePattern)
		if err != nil {
			return nil, fmt.Errorf("ftp_sftp: invalid file_pattern %q: %w", spec.FilePattern, err)
		}
		filePattern = p
	}

	return &FTPSFTPTransport{
		spec:        spec,
		sourceName:  def.Name,
		logger:      logger.WithSource(def.Name),
		username:    username,
		password:    password,
		privateKey:  privateKey,
		filePattern: filePattern,
		trackSeen:   trackSeen,
	}, nil
}

// Fetch connects to the FTP/SFTP server, downloads matching files, and returns
// concatenated content. The method and url parameters are ignored; the transport
// uses its configured Host and Path instead.
func (t *FTPSFTPTransport) Fetch(ctx context.Context, _, _ string) ([]byte, int, error) {
	switch t.spec.Protocol {
	case "ftp":
		return t.fetchFTP(ctx)
	case "sftp":
		return t.fetchSFTP(ctx)
	default:
		return nil, 0, fmt.Errorf("unsupported protocol: %s", t.spec.Protocol)
	}
}

// fetchFTP handles the FTP protocol path.
func (t *FTPSFTPTransport) fetchFTP(ctx context.Context) ([]byte, int, error) {
	host := ensurePort(t.spec.Host, defaultFTPPort)

	conn, err := ftp.Dial(host, ftp.DialWithTimeout(defaultFTPTimeout), ftp.DialWithContext(ctx))
	if err != nil {
		return nil, 0, fmt.Errorf("ftp dial %s: %w", host, err)
	}
	defer func() {
		_ = conn.Quit()
	}()

	// Login (anonymous if no credentials).
	user := t.username
	if user == "" {
		user = "anonymous"
	}
	if err := conn.Login(user, t.password); err != nil {
		return nil, 0, fmt.Errorf("ftp login: %w", err)
	}

	files, err := t.listFTPFiles(conn)
	if err != nil {
		return nil, 0, fmt.Errorf("ftp list: %w", err)
	}

	var buf bytes.Buffer
	for _, name := range files {
		if t.trackSeen {
			if _, loaded := t.seenFiles.LoadOrStore(name, struct{}{}); loaded {
				continue
			}
		}

		remotePath := filepath.ToSlash(filepath.Join(t.spec.Path, name))
		resp, err := conn.Retr(remotePath)
		if err != nil {
			t.logger.Warn("ftp retrieve failed",
				logging.String("source_name", t.sourceName),
				logging.String("file", remotePath),
				logging.Err("error", err),
			)
			// Remove from seen so we retry next cycle.
			t.seenFiles.Delete(name)
			continue
		}

		data, readErr := io.ReadAll(io.LimitReader(resp, maxFTPFileSize))
		_ = resp.Close()
		if readErr != nil {
			t.seenFiles.Delete(name)
			continue
		}

		buf.Write(data)

		if t.spec.DeleteAfterFetch {
			if delErr := conn.Delete(remotePath); delErr != nil {
				t.logger.Warn("ftp delete after fetch failed",
					logging.String("source_name", t.sourceName),
					logging.String("file", remotePath),
					logging.Err("error", delErr),
				)
			}
		}
	}

	if buf.Len() == 0 {
		return nil, 0, nil
	}
	return buf.Bytes(), 0, nil
}

// listFTPFiles lists files in the configured path, filtered by the pre-compiled pattern.
func (t *FTPSFTPTransport) listFTPFiles(conn *ftp.ServerConn) ([]string, error) {
	entries, err := conn.List(t.spec.Path)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.Type != ftp.EntryTypeFile {
			continue
		}
		if t.filePattern != nil && !t.filePattern.MatchString(e.Name) {
			continue
		}
		files = append(files, e.Name)
	}
	return files, nil
}

// fetchSFTP handles the SFTP protocol path.
func (t *FTPSFTPTransport) fetchSFTP(ctx context.Context) ([]byte, int, error) {
	host := ensurePort(t.spec.Host, defaultSFTPPort)

	sshConfig := &ssh.ClientConfig{
		User:            t.username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // configurable host key verification is out of scope for MVP
		Timeout:         defaultFTPTimeout,
	}

	// Auth methods.
	var authMethods []ssh.AuthMethod
	if t.privateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(t.privateKey))
		if err != nil {
			return nil, 0, fmt.Errorf("parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if t.password != "" {
		authMethods = append(authMethods, ssh.Password(t.password))
	}
	sshConfig.Auth = authMethods

	// Dial with context for cancellation support.
	var d net.Dialer
	netConn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, 0, fmt.Errorf("sftp dial %s: %w", host, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, host, sshConfig)
	if err != nil {
		_ = netConn.Close()
		return nil, 0, fmt.Errorf("ssh handshake %s: %w", host, err)
	}
	sshClient := ssh.NewClient(sshConn, chans, reqs)
	defer func() { _ = sshClient.Close() }()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, 0, fmt.Errorf("sftp client: %w", err)
	}
	defer func() { _ = sftpClient.Close() }()

	files, err := t.listSFTPFiles(sftpClient)
	if err != nil {
		return nil, 0, fmt.Errorf("sftp list: %w", err)
	}

	var buf bytes.Buffer
	for _, name := range files {
		if t.trackSeen {
			if _, loaded := t.seenFiles.LoadOrStore(name, struct{}{}); loaded {
				continue
			}
		}

		remotePath := filepath.ToSlash(filepath.Join(t.spec.Path, name))
		f, err := sftpClient.Open(remotePath)
		if err != nil {
			t.logger.Warn("sftp open failed",
				logging.String("source_name", t.sourceName),
				logging.String("file", remotePath),
				logging.Err("error", err),
			)
			t.seenFiles.Delete(name)
			continue
		}

		data, readErr := io.ReadAll(io.LimitReader(f, maxFTPFileSize))
		_ = f.Close()
		if readErr != nil {
			t.seenFiles.Delete(name)
			continue
		}

		buf.Write(data)

		if t.spec.DeleteAfterFetch {
			if delErr := sftpClient.Remove(remotePath); delErr != nil {
				t.logger.Warn("sftp delete after fetch failed",
					logging.String("source_name", t.sourceName),
					logging.String("file", remotePath),
					logging.Err("error", delErr),
				)
			}
		}
	}

	if buf.Len() == 0 {
		return nil, 0, nil
	}
	return buf.Bytes(), 0, nil
}

// listSFTPFiles lists files in the configured path, filtered by the pre-compiled pattern.
func (t *FTPSFTPTransport) listSFTPFiles(client *sftp.Client) ([]string, error) {
	entries, err := client.ReadDir(t.spec.Path)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if t.filePattern != nil && !t.filePattern.MatchString(e.Name()) {
			continue
		}
		files = append(files, e.Name())
	}
	return files, nil
}

// ensurePort adds a default port to the host if one is not already present.
func ensurePort(host, defaultPort string) string {
	_, _, err := net.SplitHostPort(host)
	if err != nil {
		return net.JoinHostPort(host, defaultPort)
	}
	return host
}
