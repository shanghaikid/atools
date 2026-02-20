package installer

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestInstallFile(t *testing.T) {
	tests := []struct {
		name      string
		embedPath string
		files     map[string]string
		existing  string // pre-existing dest content, empty = no file
		wantErr   bool
		wantBody  string
	}{
		{
			name:      "basic install",
			embedPath: "templates/foo.md",
			files:     map[string]string{"templates/foo.md": "hello"},
			wantBody:  "hello",
		},
		{
			name:      "skip identical content",
			embedPath: "templates/foo.md",
			files:     map[string]string{"templates/foo.md": "hello"},
			existing:  "hello",
			wantBody:  "hello",
		},
		{
			name:      "overwrite changed content",
			embedPath: "templates/foo.md",
			files:     map[string]string{"templates/foo.md": "new"},
			existing:  "old",
			wantBody:  "new",
		},
		{
			name:      "creates parent dirs",
			embedPath: "templates/sub/bar.md",
			files:     map[string]string{"templates/sub/bar.md": "nested"},
			wantBody:  "nested",
		},
		{
			name:      "missing embed path",
			embedPath: "templates/nope.md",
			files:     map[string]string{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := make(fstest.MapFS)
			for k, v := range tt.files {
				fsys[k] = &fstest.MapFile{Data: []byte(v)}
			}

			tmpDir := t.TempDir()
			destPath := filepath.Join(tmpDir, "out", "file.md")

			if tt.existing != "" {
				if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(destPath, []byte(tt.existing), 0644); err != nil {
					t.Fatal(err)
				}
			}

			inst := &Installer{FS: fsys}
			err := inst.InstallFile(tt.embedPath, destPath)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, err := os.ReadFile(destPath)
			if err != nil {
				t.Fatalf("read dest: %v", err)
			}
			if string(got) != tt.wantBody {
				t.Errorf("got %q, want %q", string(got), tt.wantBody)
			}
		})
	}
}

func TestInstallFile_DryRun(t *testing.T) {
	fsys := fstest.MapFS{
		"templates/foo.md": &fstest.MapFile{Data: []byte("content")},
	}

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "out", "foo.md")

	inst := &Installer{FS: fsys, DryRun: true}
	if err := inst.InstallFile("templates/foo.md", destPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Fatal("file should not exist in dry-run mode")
	}
}

func TestInstallDir(t *testing.T) {
	fsys := fstest.MapFS{
		"agents":          &fstest.MapFile{Mode: os.ModeDir},
		"agents/coder.md": &fstest.MapFile{Data: []byte("coder")},
		"agents/test.md":  &fstest.MapFile{Data: []byte("tester")},
		"agents/.hidden":  &fstest.MapFile{Data: []byte("skip")},
	}

	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "out")

	inst := &Installer{FS: fsys}
	if err := inst.InstallDir("agents", destDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check installed files
	for _, name := range []string{"coder.md", "test.md"} {
		if _, err := os.Stat(filepath.Join(destDir, name)); err != nil {
			t.Errorf("expected %s to exist", name)
		}
	}

	// Hidden file should be skipped
	if _, err := os.Stat(filepath.Join(destDir, ".hidden")); !os.IsNotExist(err) {
		t.Error("hidden file should be skipped")
	}
}

func TestInstallDir_MissingDir(t *testing.T) {
	fsys := fstest.MapFS{}

	inst := &Installer{FS: fsys}
	err := inst.InstallDir("nonexistent", t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}
