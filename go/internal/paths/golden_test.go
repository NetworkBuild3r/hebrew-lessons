package paths

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGoldenPathsMatchNode(t *testing.T) {
	app := t.TempDir()
	work := t.TempDir()
	if err := os.Mkdir(filepath.Join(work, "var"), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []map[string]any{
		{
			"name": "image defaults under HEBREW_ROOT",
			"cwd":  work,
			"env":  map[string]string{"HEBREW_ROOT": app},
		},
		{
			"name": "explicit data and lessons",
			"cwd":  work,
			"env": map[string]string{
				"HEBREW_ROOT": app,
				"DATA_DIR":    filepath.Join(app, "var", "app"),
				"LESSONS_DIR": filepath.Join(app, "var", "lessons"),
			},
		},
		{
			"name": "relative HEBREW_ROOT",
			"cwd":  work,
			"env":  map[string]string{"HEBREW_ROOT": "app"},
		},
		{
			"name": "relative DATA_DIR against cwd",
			"cwd":  work,
			"env": map[string]string{
				"HEBREW_ROOT": app,
				"DATA_DIR":    filepath.Join("var", "data"),
				"LESSONS_DIR": filepath.Join(app, "lessons"),
			},
		},
		{
			"name": "DATA_DIR inside lessons",
			"cwd":  app,
			"env": map[string]string{
				"HEBREW_ROOT": app,
				"DATA_DIR":    filepath.Join("worksheets", "nested"),
			},
		},
		{
			"name": "DATA_DIR is shared",
			"cwd":  work,
			"env": map[string]string{
				"HEBREW_ROOT": app,
				"DATA_DIR":    filepath.Join(app, "shared"),
			},
		},
		{
			"name": "DATA_DIR inside review",
			"cwd":  work,
			"env": map[string]string{
				"HEBREW_ROOT": app,
				"DATA_DIR":    filepath.Join(app, "review", "deck"),
			},
		},
		{
			"name": "shared-backup is not shared",
			"cwd":  work,
			"env": map[string]string{
				"HEBREW_ROOT": app,
				"DATA_DIR":    filepath.Join(app, "shared-backup"),
			},
		},
	}

	payload, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", filepath.Join("testdata", "golden-paths.js"))
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node golden: %v\n%s", err, out)
	}

	for _, line := range bytes.Split(bytes.TrimSpace(out), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var oracle struct {
			Name    string `json:"name"`
			OK      bool   `json:"ok"`
			Root    string `json:"root"`
			Lessons string `json:"lessons"`
			Data    string `json:"data"`
			DB      string `json:"db"`
			Media   string `json:"media"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(line, &oracle); err != nil {
			t.Fatalf("oracle line %s: %v", line, err)
		}
		var item map[string]any
		for _, c := range cases {
			if c["name"] == oracle.Name {
				item = c
				break
			}
		}
		if item == nil {
			t.Fatalf("unexpected oracle case %s", oracle.Name)
		}
		env := item["env"].(map[string]string)
		t.Chdir(item["cwd"].(string))
		layout, gotErr := Resolve(env["HEBREW_ROOT"], env["DATA_DIR"], env["LESSONS_DIR"])
		if oracle.OK {
			if gotErr != nil {
				t.Fatalf("%s: go error %v; node accepted", oracle.Name, gotErr)
			}
			if layout.Root != oracle.Root || layout.LessonsDir != oracle.Lessons ||
				layout.DataDir != oracle.Data || layout.DBPath != oracle.DB ||
				layout.MediaDir != oracle.Media {
				t.Fatalf("%s:\n go   root=%s data=%s lessons=%s db=%s media=%s\n node root=%s data=%s lessons=%s db=%s media=%s",
					oracle.Name, layout.Root, layout.DataDir, layout.LessonsDir, layout.DBPath, layout.MediaDir,
					oracle.Root, oracle.Data, oracle.Lessons, oracle.DB, oracle.Media)
			}
			continue
		}
		if gotErr == nil {
			t.Fatalf("%s: go accepted; node error %s", oracle.Name, oracle.Error)
		}
		if gotErr.Error() != oracle.Error {
			t.Fatalf("%s:\n go   %s\n node %s", oracle.Name, gotErr.Error(), oracle.Error)
		}
	}
}
