package audio

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type tarFile struct {
	name     string
	body     []byte
	typeflag byte
	link     string
}

func TestImportRejectsSpecialArchiveEntries(t *testing.T) {
	mapBody := v2Map(nil, nil, nil)
	cases := []struct {
		name string
		file tarFile
		want string
	}{
		{"absolute", tarFile{name: "/abs.mp3", body: []byte("nope")}, "absolute"},
		{"symlink", tarFile{name: "link.mp3", typeflag: tar.TypeSymlink, link: "other"}, "symlink"},
		{"hardlink", tarFile{name: "hard.mp3", typeflag: tar.TypeLink, link: "other.mp3"}, "hardlink"},
		{"fifo", tarFile{name: "pipe.mp3", typeflag: tar.TypeFifo}, "non-regular"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := importRawTar(t, []tarFile{
				{name: "clip-text-map.json", body: mapBody},
				tc.file,
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestImportRejectsRepeatedArchivePath(t *testing.T) {
	body := []byte("once")
	word := wordObj("קָם", "voice-a", "eleven_v4", body, "")
	raw := v2Map(nil, []any{word}, nil)
	rel := word["path"].(string)
	_, err := importRawTar(t, []tarFile{
		{name: "clip-text-map.json", body: raw},
		{name: rel, body: body},
		{name: rel, body: body},
	})
	if err == nil || !strings.Contains(err.Error(), "repeated path") {
		t.Fatalf("error = %v", err)
	}
}

func TestImportRejectsOversizeEntry(t *testing.T) {
	prev := maxClipBytes
	maxClipBytes = 4
	t.Cleanup(func() { maxClipBytes = prev })

	body := []byte("0123456789")
	word := wordObj("קָם", "voice-a", "eleven_v4", body, "")
	export := t.TempDir()
	writeFiles(t, export, map[string][]byte{word["path"].(string): body})
	writeMap(t, export, nil, []any{word}, nil)
	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC))
	_, err := Import(context.Background(), store, data, export, Options{})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v", err)
	}
	if mp3Count(t, filepath.Join(data, "audio")) != 0 {
		t.Fatal("oversize entry was stored")
	}
}

func TestImportRejectsSymlinkAndFifoOnDisk(t *testing.T) {
	export := t.TempDir()
	writeMap(t, export, nil, nil, nil)
	if err := os.Symlink("clip-text-map.json", filepath.Join(export, "link.mp3")); err != nil {
		t.Fatal(err)
	}
	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC))
	_, err := Import(context.Background(), store, data, export, Options{})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink error = %v", err)
	}

	export = t.TempDir()
	writeMap(t, export, nil, nil, nil)
	fifo := filepath.Join(export, "pipe.mp3")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	data = t.TempDir()
	store = openStoreAt(t, data, time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC))
	_, err = Import(context.Background(), store, data, export, Options{})
	if err == nil || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("fifo error = %v", err)
	}
}

func TestWriteAtomicRemovesTempOnFailure(t *testing.T) {
	dir := t.TempDir()
	failAtomicWrite = true
	t.Cleanup(func() { failAtomicWrite = false })
	err := writeAtomic(filepath.Join(dir, "clip.mp3"), []byte("abc"))
	if err == nil {
		t.Fatal("expected injected failure")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temp file left behind: %v", entries)
	}
}

func TestSpoolRemovedOnErrorAndOversize(t *testing.T) {
	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC))
	_, err := Import(context.Background(), store, data, "", Options{Archive: boomReader{}})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v", err)
	}
	assertNoSpool(t, data)

	prev := maxSpoolBytes
	maxSpoolBytes = 8
	t.Cleanup(func() { maxSpoolBytes = prev })
	_, err = Import(context.Background(), store, data, "", Options{Archive: bytes.NewReader(bytes.Repeat([]byte("x"), 64))})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversize error = %v", err)
	}
	assertNoSpool(t, data)
}

func TestSpoolRemovedOnSignal(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, ".import-stdin-*.tgz")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	stop := unlinkOnSignal(name)
	defer stop()
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(name); errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("spool file still present after SIGTERM")
}

type boomReader struct{}

func (boomReader) Read([]byte) (int, error) {
	return 0, errors.New("boom")
}

func assertNoSpool(t *testing.T, data string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(data, "audio", ".import-stdin-*.tgz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("spool left behind: %v", matches)
	}
}

func importRawTar(t *testing.T, files []tarFile) (Report, error) {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, file := range files {
		flag := file.typeflag
		if flag == 0 {
			flag = tar.TypeReg
		}
		size := int64(len(file.body))
		if flag == tar.TypeSymlink || flag == tar.TypeLink || flag == tar.TypeFifo {
			size = 0
			file.body = nil
		}
		hdr := &tar.Header{
			Name:     file.name,
			Mode:     0o644,
			Size:     size,
			Typeflag: flag,
			Linkname: file.link,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if size > 0 {
			if _, err := tw.Write(file.body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 8, 0, 0, 0, time.UTC))
	return Import(context.Background(), store, data, "", Options{Archive: &buf})
}
