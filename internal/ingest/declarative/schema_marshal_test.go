package declarative

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestDuration_MarshalYAML verifies that Duration.MarshalYAML produces the string
// representation that round-trips correctly through UnmarshalYAML.
func TestDuration_MarshalYAML(t *testing.T) {
	tests := []struct {
		name  string
		input string // YAML to unmarshal
		want  string // expected marshaled string
	}{
		{name: "10 seconds", input: `"10s"`, want: "10s"},
		{name: "5 minutes", input: `"5m0s"`, want: "5m0s"},
		{name: "1 hour 30 minutes", input: `"1h30m0s"`, want: "1h30m0s"},
		{name: "500 milliseconds", input: `"500ms"`, want: "500ms"},
		{name: "zero", input: `"0s"`, want: "0s"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var d Duration
			if err := yaml.Unmarshal([]byte(tc.input), &d); err != nil {
				t.Fatalf("UnmarshalYAML: %v", err)
			}

			got, err := d.MarshalYAML()
			if err != nil {
				t.Fatalf("MarshalYAML: %v", err)
			}

			s, ok := got.(string)
			if !ok {
				t.Fatalf("MarshalYAML returned %T, want string", got)
			}
			if s != tc.want {
				t.Errorf("MarshalYAML = %q, want %q", s, tc.want)
			}
		})
	}
}

// TestOAuth2Value_UnmarshalYAML verifies both plain-string and struct forms.
func TestOAuth2Value_UnmarshalYAML(t *testing.T) {
	t.Run("plain string value", func(t *testing.T) {
		input := `"mysecret"`
		var v OAuth2Value
		if err := yaml.Unmarshal([]byte(input), &v); err != nil {
			t.Fatalf("UnmarshalYAML: %v", err)
		}
		if v.Value != "mysecret" {
			t.Errorf("Value = %q, want %q", v.Value, "mysecret")
		}
		if v.EnvVar != "" {
			t.Errorf("EnvVar = %q, want empty", v.EnvVar)
		}
	})

	t.Run("struct with env_var", func(t *testing.T) {
		input := `
env_var: MY_SECRET_ENV
`
		var v OAuth2Value
		if err := yaml.Unmarshal([]byte(input), &v); err != nil {
			t.Fatalf("UnmarshalYAML: %v", err)
		}
		if v.EnvVar != "MY_SECRET_ENV" {
			t.Errorf("EnvVar = %q, want %q", v.EnvVar, "MY_SECRET_ENV")
		}
		if v.Value != "" {
			t.Errorf("Value = %q, want empty", v.Value)
		}
	})

	t.Run("struct with literal value field", func(t *testing.T) {
		input := `
value: mytoken
`
		var v OAuth2Value
		if err := yaml.Unmarshal([]byte(input), &v); err != nil {
			t.Fatalf("UnmarshalYAML: %v", err)
		}
		if v.Value != "mytoken" {
			t.Errorf("Value = %q, want %q", v.Value, "mytoken")
		}
	})

	t.Run("empty string", func(t *testing.T) {
		input := `""`
		var v OAuth2Value
		if err := yaml.Unmarshal([]byte(input), &v); err != nil {
			t.Fatalf("UnmarshalYAML: %v", err)
		}
		if v.Value != "" {
			t.Errorf("Value = %q, want empty", v.Value)
		}
	})
}

// TestSourceDefinition_IsEnabled verifies the enabled/disabled logic.
func TestSourceDefinition_IsEnabled(t *testing.T) {
	t.Run("nil pointer means enabled", func(t *testing.T) {
		sd := &SourceDefinition{Enabled: nil}
		if !sd.IsEnabled() {
			t.Error("expected IsEnabled() == true when Enabled is nil")
		}
	})

	t.Run("explicit true", func(t *testing.T) {
		b := true
		sd := &SourceDefinition{Enabled: &b}
		if !sd.IsEnabled() {
			t.Error("expected IsEnabled() == true when Enabled = &true")
		}
	})

	t.Run("explicit false", func(t *testing.T) {
		b := false
		sd := &SourceDefinition{Enabled: &b}
		if sd.IsEnabled() {
			t.Error("expected IsEnabled() == false when Enabled = &false")
		}
	})
}

// TestNewValidator_Success verifies the validator is created without error.
func TestNewValidator_Success(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}
