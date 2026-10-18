package config

import (
	"net"
	"path/filepath"
	"strings"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func TestLoadDefaults(t *testing.T) {
	root := t.TempDir()
	cfg, err := load(env(map[string]string{"HEBREW_ROOT": root}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != DefaultPort {
		t.Fatalf("port: %d", cfg.Port)
	}
	if DefaultPort == 8080 || DefaultPort == 8787 {
		t.Fatalf("default port %d clashes with Node or MCP", DefaultPort)
	}
	if cfg.CookieSecure {
		t.Fatal("COOKIE_SECURE default must be false")
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Fatalf("proxies: %v", cfg.TrustedProxies)
	}
	if cfg.DataDir != filepath.Join(root, "data") {
		t.Fatalf("data: %s", cfg.DataDir)
	}
	if cfg.LessonsDir != filepath.Join(root, "worksheets") {
		t.Fatalf("lessons: %s", cfg.LessonsDir)
	}
	if cfg.DBPath != filepath.Join(root, "data", "hebrew.sqlite") {
		t.Fatalf("db: %s", cfg.DBPath)
	}
	if cfg.MediaDir != filepath.Join(root, "data", "media") {
		t.Fatalf("media: %s", cfg.MediaDir)
	}
}

func TestLoadOverrides(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "var", "app")
	lessons := filepath.Join(root, "var", "lessons")
	cfg, err := load(env(map[string]string{
		"HEBREW_ROOT":     root,
		"DATA_DIR":        data,
		"LESSONS_DIR":     lessons,
		"PORT":            " 8091 ",
		"COOKIE_SECURE":   "1",
		"TRUSTED_PROXIES": "10.0.0.0/8, 192.168.1.10, fd00::/8",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8091 {
		t.Fatalf("port: %d", cfg.Port)
	}
	if !cfg.CookieSecure {
		t.Fatal("COOKIE_SECURE=1 should be true")
	}
	if cfg.DataDir != data || cfg.LessonsDir != lessons {
		t.Fatalf("dirs: %+v", cfg)
	}
	if cfg.DBPath != filepath.Join(data, "hebrew.sqlite") {
		t.Fatalf("db: %s", cfg.DBPath)
	}
	got := make([]string, len(cfg.TrustedProxies))
	for i, n := range cfg.TrustedProxies {
		got[i] = n.String()
	}
	want := []string{"10.0.0.0/8", "192.168.1.10/32", "fd00::/8"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("proxies: got %v want %v", got, want)
	}
}

func TestCookieSecureOnlyExactOne(t *testing.T) {
	root := t.TempDir()
	for _, value := range []string{"", "0", "true", "yes", "1 "} {
		cfg, err := load(env(map[string]string{
			"HEBREW_ROOT":   root,
			"COOKIE_SECURE": value,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.CookieSecure {
			t.Fatalf("COOKIE_SECURE %q was true", value)
		}
	}
}

func TestPortZeroIsEphemeral(t *testing.T) {
	root := t.TempDir()
	cfg, err := load(env(map[string]string{
		"HEBREW_ROOT": root,
		"PORT":        "0",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 0 {
		t.Fatalf("port: %d", cfg.Port)
	}
}

func TestRejectBadPort(t *testing.T) {
	root := t.TempDir()
	for _, value := range []string{"nope", "-1", "65536", "8080x"} {
		_, err := load(env(map[string]string{
			"HEBREW_ROOT": root,
			"PORT":        value,
		}))
		if err == nil {
			t.Fatalf("PORT %q was accepted", value)
		}
	}
}

func TestRejectBadProxy(t *testing.T) {
	root := t.TempDir()
	_, err := load(env(map[string]string{
		"HEBREW_ROOT":     root,
		"TRUSTED_PROXIES": "10.0.0.0/8, not-a-cidr",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not-a-cidr") {
		t.Fatalf("error: %v", err)
	}
}

func TestRejectDataDirInsideLessons(t *testing.T) {
	root := t.TempDir()
	_, err := load(env(map[string]string{
		"HEBREW_ROOT": root,
		"DATA_DIR":    filepath.Join(root, "worksheets", "nested"),
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "served root") {
		t.Fatalf("error: %v", err)
	}
}

func TestLoadReadsProcessEnv(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HEBREW_ROOT", root)
	t.Setenv("DATA_DIR", filepath.Join(root, "var"))
	t.Setenv("LESSONS_DIR", filepath.Join(root, "lessons"))
	t.Setenv("PORT", "8092")
	t.Setenv("COOKIE_SECURE", "1")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8092 || !cfg.CookieSecure {
		t.Fatalf("cfg: %+v", cfg)
	}
	if len(cfg.TrustedProxies) != 1 || cfg.TrustedProxies[0].String() != "127.0.0.1/32" {
		t.Fatalf("proxies: %v", cfg.TrustedProxies)
	}
	if !cfg.TrustedProxies[0].Contains(net.ParseIP("127.0.0.1")) {
		t.Fatal("127.0.0.1/32 should contain itself")
	}
}
