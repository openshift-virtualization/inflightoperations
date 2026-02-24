package rules

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestLoadRules(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		errString string
	}{
		{
			name: "valid rules file",
			yaml: `gvk:
  group: kubevirt.io
  version: v1
  kind: VirtualMachine
rules:
- operation: Migrating
  expression: object.status.printableStatus == "Migrating"
- operation: Starting
  expression: object.status.printableStatus == "Starting"
`,
			wantErr: false,
		},
		{
			name: "core resource (empty group)",
			yaml: `gvk:
  group: ""
  version: v1
  kind: Pod
rules:
- operation: Pending
  expression: object.status.phase == "Pending"
`,
			wantErr: false,
		},
		{
			name: "missing kind",
			yaml: `gvk:
  group: kubevirt.io
  version: v1
rules:
- operation: Migrating
  expression: object.status.printableStatus == "Migrating"
`,
			wantErr:   true,
			errString: "GVK.kind is required",
		},
		{
			name: "missing version",
			yaml: `gvk:
  group: kubevirt.io
  kind: VirtualMachine
rules:
- operation: Migrating
  expression: object.status.printableStatus == "Migrating"
`,
			wantErr:   true,
			errString: "GVK.version is required",
		},
		{
			name: "no rules",
			yaml: `gvk:
  group: kubevirt.io
  version: v1
  kind: VirtualMachine
rules: []
`,
			wantErr:   true,
			errString: "at least one rule is required",
		},
		{
			name: "missing operation",
			yaml: `gvk:
  group: kubevirt.io
  version: v1
  kind: VirtualMachine
rules:
- expression: object.status.printableStatus == "Migrating"
`,
			wantErr:   true,
			errString: "rule[0].operation is required",
		},
		{
			name: "missing expression",
			yaml: `gvk:
  group: kubevirt.io
  version: v1
  kind: VirtualMachine
rules:
- operation: Migrating
`,
			wantErr:   true,
			errString: "rule[0].expression is required",
		},
		{
			name:      "invalid YAML",
			yaml:      "invalid: yaml: syntax: error",
			wantErr:   true,
			errString: "failed to parse rules file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "rules.yaml")
			if err := os.WriteFile(tmpFile, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			// Load rules
			rf, err := LoadRules(tmpFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadRules() expected error but got none")
					return
				}
				if tt.errString != "" && err.Error() != tt.errString {
					// Check if error message contains the expected string
					contains := false
					for i := 0; i <= len(err.Error())-len(tt.errString); i++ {
						if err.Error()[i:i+len(tt.errString)] == tt.errString {
							contains = true
							break
						}
					}
					if !contains {
						t.Errorf("LoadRules() error = %q, want error containing %q", err.Error(), tt.errString)
					}
				}
			} else {
				if err != nil {
					t.Errorf("LoadRules() unexpected error: %v", err)
					return
				}
				if rf == nil {
					t.Errorf("LoadRules() returned nil RuleSet")
				}
			}
		})
	}
}

func TestLoadRulesFileNotFound(t *testing.T) {
	_, err := LoadRules("/nonexistent/file.yaml")
	if err == nil {
		t.Errorf("LoadRules() expected error for nonexistent file but got none")
	}
}

func TestRulesFileValidate(t *testing.T) {
	tests := []struct {
		name    string
		rf      RuleSet
		wantErr bool
	}{
		{
			name: "valid",
			rf: RuleSet{
				GVK: GroupVersionKind{
					Group:   "kubevirt.io",
					Version: "v1",
					Kind:    "VirtualMachine",
				},
				Rules: []Rule{
					{Operation: "Migrating", Expression: "object.status.printableStatus == \"Migrating\""},
				},
			},
			wantErr: false,
		},
		{
			name: "empty kind",
			rf: RuleSet{
				GVK: GroupVersionKind{
					Version: "v1",
				},
				Rules: []Rule{
					{Operation: "Migrating", Expression: "object.status.printableStatus == \"Migrating\""},
				},
			},
			wantErr: true,
		},
		{
			name: "empty version",
			rf: RuleSet{
				GVK: GroupVersionKind{
					Kind: "VirtualMachine",
				},
				Rules: []Rule{
					{Operation: "Migrating", Expression: "object.status.printableStatus == \"Migrating\""},
				},
			},
			wantErr: true,
		},
		{
			name: "no rules",
			rf: RuleSet{
				GVK: GroupVersionKind{
					Version: "v1",
					Kind:    "VirtualMachine",
				},
				Rules: []Rule{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rf.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RuleSet.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadRulesFromBytes(t *testing.T) {
	yaml := []byte(`gvk:
  group: kubevirt.io
  version: v1
  kind: VirtualMachine
rules:
- operation: Migrating
  expression: object.status.printableStatus == "Migrating"
`)

	rf, err := LoadRulesFromBytes(yaml)
	if err != nil {
		t.Fatalf("LoadRulesFromBytes() unexpected error: %v", err)
	}

	if rf == nil {
		t.Fatal("LoadRulesFromBytes() returned nil RuleSet")
	}

	if rf.GVK.Kind != "VirtualMachine" {
		t.Errorf("LoadRulesFromBytes() GVK.Kind = %q, want %q", rf.GVK.Kind, "VirtualMachine")
	}

	if len(rf.Rules) != 1 {
		t.Errorf("LoadRulesFromBytes() len(Rules) = %d, want %d", len(rf.Rules), 1)
	}
}

func TestLoadRulesForGVK_File(t *testing.T) {
	gvk := schema.GroupVersionKind{
		Group:   "kubevirt.io",
		Version: "v1",
		Kind:    "VirtualMachine",
	}

	rf, err := LoadRulesForGVK("testdata/single-rule.yaml", gvk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rf.GVK.ToSchemaGVK() != gvk {
		t.Errorf("GVK = %v, want %v", rf.GVK.ToSchemaGVK(), gvk)
	}
}

func TestLoadRulesForGVK_File_WrongGVK(t *testing.T) {
	// File contains kubevirt.io/v1/VirtualMachine
	wrongGVK := schema.GroupVersionKind{
		Group:   "cdi.kubevirt.io",
		Version: "v1beta1",
		Kind:    "DataVolume",
	}

	_, err := LoadRulesForGVK("testdata/single-rule.yaml", wrongGVK)
	if err == nil {
		t.Fatal("expected error for mismatched GVK")
	}

	if !contains(err.Error(), "does not match") {
		t.Errorf("error should mention GVK mismatch, got: %v", err)
	}
}

func TestLoadRulesForGVK_Directory(t *testing.T) {
	vmGVK := schema.GroupVersionKind{
		Group:   "kubevirt.io",
		Version: "v1",
		Kind:    "VirtualMachine",
	}

	rf, err := LoadRulesForGVK("testdata/rules-dir", vmGVK)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rf.GVK.ToSchemaGVK() != vmGVK {
		t.Errorf("GVK = %v, want %v", rf.GVK.ToSchemaGVK(), vmGVK)
	}
}

func TestLoadRulesForGVK_Directory_DataVolume(t *testing.T) {
	dvGVK := schema.GroupVersionKind{
		Group:   "cdi.kubevirt.io",
		Version: "v1beta1",
		Kind:    "DataVolume",
	}

	rf, err := LoadRulesForGVK("testdata/rules-dir", dvGVK)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rf.GVK.ToSchemaGVK() != dvGVK {
		t.Errorf("GVK = %v, want %v", rf.GVK.ToSchemaGVK(), dvGVK)
	}
}

func TestLoadRulesForGVK_NotFound(t *testing.T) {
	podGVK := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	_, err := LoadRulesForGVK("testdata/rules-dir", podGVK)
	if err == nil {
		t.Fatal("expected error for non-existent GVK")
	}

	if !contains(err.Error(), "no rules found") {
		t.Errorf("error should mention no rules found: %v", err)
	}
}

func TestLoadRulesForGVK_FileNotFound(t *testing.T) {
	gvk := schema.GroupVersionKind{
		Group:   "kubevirt.io",
		Version: "v1",
		Kind:    "VirtualMachine",
	}

	_, err := LoadRulesForGVK("/nonexistent/file.yaml", gvk)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadRulesForGVK_DirectoryNotFound(t *testing.T) {
	gvk := schema.GroupVersionKind{
		Group:   "kubevirt.io",
		Version: "v1",
		Kind:    "VirtualMachine",
	}

	_, err := LoadRulesForGVK("/nonexistent/directory", gvk)
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
