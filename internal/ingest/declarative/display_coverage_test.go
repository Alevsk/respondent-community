package declarative

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
)

func TestRegisterDisplayConfigs_WithTransportOnDemandURL(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `schema_version: 1
name: ondemand_display_src
source_type: ondemand_display_src_type
layer_type: ondemand_display_layer
display_name: "OnDemand Display Test"
transport:
  on_demand_url: "https://example.com/on-demand/{lat}/{lon}"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	if err := os.WriteFile(filepath.Join(dir, "ondemand.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	if err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), nil); err != nil {
		t.Fatalf("RegisterDisplayConfigs: %v", err)
	}
}

func TestRegisterDisplayConfigs_ReadDirError(t *testing.T) {
	// Pass a path that is a file (not a directory) to trigger a ReadDir error
	// that is not IsNotExist.
	f, err := os.CreateTemp("", "not_a_dir")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_ = f.Close()

	// ReadDir on a file returns an error that is NOT IsNotExist.
	err = RegisterDisplayConfigs(f.Name(), domain.NewDynamicSourceRegistry(), logging.NewNopLogger())
	if err == nil {
		t.Error("expected error when dir is a file, got nil")
	}
}

func TestRegisterDisplayConfigs_ReadFileError(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(yamlPath, []byte("layer_type: test\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Make the file unreadable.
	if err := os.Chmod(yamlPath, 0000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer func() { _ = os.Chmod(yamlPath, 0644) }()

	// Should silently skip the unreadable file.
	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), logging.NewNopLogger())
	if err != nil {
		t.Errorf("expected nil (file error is warned, not fatal), got %v", err)
	}
}

func TestRegisterDisplayConfigs_NilLoggerReadError(t *testing.T) {
	dir := t.TempDir()
	// Create an unreadable file to trigger the os.ReadFile error path.
	badFile := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badFile, []byte("name: test"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.Chmod(badFile, 0000); err != nil {
		t.Skipf("cannot chmod file to 000 (probably root): %v", err)
	}
	defer func() { _ = os.Chmod(badFile, 0644) }()

	// With nil logger – warnings silently dropped (covers if log != nil check).
	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), nil)
	if err != nil {
		t.Errorf("RegisterDisplayConfigs with nil logger should not error: %v", err)
	}
}

func TestRegisterDisplayConfigs_NonNilLoggerReadError(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badFile, []byte("name: test"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.Chmod(badFile, 0000); err != nil {
		t.Skipf("cannot chmod file to 000: %v", err)
	}
	defer func() { _ = os.Chmod(badFile, 0644) }()

	// With non-nil logger – warning is logged but no error returned.
	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), logging.NewNopLogger())
	if err != nil {
		t.Errorf("RegisterDisplayConfigs with logger should not error on read failure: %v", err)
	}
}

func TestRegisterDisplayConfigs_NonYAMLExtension(t *testing.T) {
	dir := t.TempDir()
	// Create files with non-yaml extensions – they should be skipped.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("text"), 0644); err != nil {
		t.Fatalf("write txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), logging.NewNopLogger())
	if err != nil {
		t.Errorf("RegisterDisplayConfigs should not error for non-yaml files: %v", err)
	}
}

func TestRegisterDisplayConfigs_IsDirSkipped(t *testing.T) {
	dir := t.TempDir()
	// Create a subdirectory – should be skipped.
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), logging.NewNopLogger())
	if err != nil {
		t.Errorf("RegisterDisplayConfigs should not error with subdir: %v", err)
	}
}

func TestRegisterDisplayConfigs_EmptyLayerType(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `display_name: "No Layer Type"
display:
  icon:
    shape: dot
    scale: 1.0
  style:
    color: "#ffffff"
    point_size: 6
`
	if err := os.WriteFile(filepath.Join(dir, "no_layer.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), logging.NewNopLogger())
	if err != nil {
		t.Errorf("RegisterDisplayConfigs should not error for empty layer_type: %v", err)
	}
}

func TestRegisterDisplayConfigs_EmptyIconShape(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `layer_type: test_no_shape_layer
source_type: test_no_shape
display_name: "No Icon Shape"
display:
  icon:
    scale: 1.0
  style:
    color: "#ffffff"
    point_size: 6
`
	if err := os.WriteFile(filepath.Join(dir, "no_shape.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), logging.NewNopLogger())
	if err != nil {
		t.Errorf("RegisterDisplayConfigs should not error for empty icon shape: %v", err)
	}
}

func TestRegisterDisplayConfigs_YAMLParseError(t *testing.T) {
	dir := t.TempDir()
	invalidYAML := "layer_type: [unclosed bracket\n"
	if err := os.WriteFile(filepath.Join(dir, "invalid.yaml"), []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), logging.NewNopLogger())
	if err != nil {
		t.Errorf("RegisterDisplayConfigs should not error for invalid YAML: %v", err)
	}
}

func TestRegisterDisplayConfigs_RegisterError(t *testing.T) {
	dir := t.TempDir()
	// Valid display config but with an invalid indicator level_expr in a value.
	yamlContent := `layer_type: test_indicator_err_layer
source_type: test_indicator_err
display_name: "Indicator Error Test"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ff0000"
  style:
    color: "#ff0000"
    point_size: 6
indicator:
  values:
    - key: "bad_value"
      label: "Bad"
      source_field: "val"
      unit: ""
      level_expr: "!!! INVALID CEL !!!"
  summary_expr: '"ok"'
`
	if err := os.WriteFile(filepath.Join(dir, "indicator_err.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), logging.NewNopLogger())
	if err != nil {
		t.Errorf("RegisterDisplayConfigs should not error for indicator compile error: %v", err)
	}
}
