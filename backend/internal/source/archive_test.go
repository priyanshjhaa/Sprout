package source

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"
)

func archive(t *testing.T, headers ...tar.Header) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := tar.NewWriter(&data)
	for _, header := range headers {
		if header.Format == tar.FormatUnknown {
			header.Format = tar.FormatUSTAR
		}
		if err := writer.WriteHeader(&header); err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := io.CopyN(writer, zeroReader{}, header.Size); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

type zeroReader struct{}

func TestArchiveAcceptsBSDNumericPadding(t *testing.T) {
	parent := temporaryParent(t)
	input := archive(t, tar.Header{Name: "index.html", Typeflag: tar.TypeReg, Size: 2})
	input[107] = ' ' // Mode field ends in a space rather than NUL.
	for i := 148; i < 156; i++ {
		input[i] = ' '
	}
	var sum int
	for _, b := range input[:512] {
		sum += int(b)
	}
	copy(input[148:156], fmt.Sprintf("%06o\x00 ", sum))
	_, err := WithArchive(context.Background(), bytes.NewReader(input), func(context.Context, fs.FS) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	assertClean(t, parent)
}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }
func temporaryParent(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	t.Setenv("TMPDIR", parent)
	return parent
}
func assertClean(t *testing.T, parent string) {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary source leaked: %d entries", len(entries))
	}
}
func TestArchivePreparationAndCleanup(t *testing.T) {
	parent := temporaryParent(t)
	input := archive(t, tar.Header{Name: "src/", Typeflag: tar.TypeDir, Mode: 0777}, tar.Header{Name: "src/main.go", Typeflag: tar.TypeReg, Size: 5, Mode: 07777})
	summary, err := WithArchive(context.Background(), bytes.NewReader(input), func(ctx context.Context, files fs.FS) error {
		data, err := fs.ReadFile(files, "src/main.go")
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != 5 {
			t.Fatal("wrong contents")
		}
		info, err := fs.Stat(files, "src/main.go")
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 || info.Mode()&os.ModeSetuid != 0 {
			t.Fatal("archive permissions retained")
		}
		dir, err := fs.Stat(files, "src")
		if err != nil || dir.Mode().Perm() != 0700 {
			t.Fatal("directory not private")
		}
		return nil
	})
	if err != nil || summary.Files != 1 || summary.Bytes != 5 {
		t.Fatalf("%+v %v", summary, err)
	}
	assertClean(t, parent)
}
func TestArchiveRejectsUntrustedEntries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header tar.Header
		want   error
	}{
		{"traversal", tar.Header{Name: "../escape", Typeflag: tar.TypeReg}, ErrInvalid},
		{"absolute", tar.Header{Name: "/escape", Typeflag: tar.TypeReg}, ErrInvalid},
		{"normalized traversal", tar.Header{Name: "src/../escape", Typeflag: tar.TypeReg}, ErrInvalid},
		{"windows", tar.Header{Name: `C:\escape`, Typeflag: tar.TypeReg}, ErrInvalid},
		{"control", tar.Header{Name: "private\nname", Typeflag: tar.TypeReg}, ErrInvalid},
		{"symlink", tar.Header{Name: "link", Linkname: "../escape", Typeflag: tar.TypeSymlink}, ErrInvalid},
		{"hardlink", tar.Header{Name: "link", Linkname: "main.go", Typeflag: tar.TypeLink}, ErrInvalid},
		{"fifo", tar.Header{Name: "pipe", Typeflag: tar.TypeFifo}, ErrInvalid},
		{"device", tar.Header{Name: "device", Typeflag: tar.TypeChar}, ErrInvalid},
		{"environment", tar.Header{Name: "config/.env.production", Typeflag: tar.TypeReg}, ErrSensitive},
		{"git", tar.Header{Name: ".git/config", Typeflag: tar.TypeReg}, ErrSensitive},
		{"key", tar.Header{Name: "private.PEM", Typeflag: tar.TypeReg}, ErrSensitive},
		{"credential config", tar.Header{Name: ".npmrc", Typeflag: tar.TypeReg}, ErrSensitive},
		{"depth", tar.Header{Name: strings.Repeat("a/", 17) + "file", Typeflag: tar.TypeReg}, ErrLimit},
		{"pax", tar.Header{Name: "main.go", Typeflag: tar.TypeReg, Format: tar.FormatPAX, PAXRecords: map[string]string{"CUSTOM.hidden": "data"}}, ErrInvalid},
		{"file size", tar.Header{Name: "large", Typeflag: tar.TypeReg, Size: MaxFileBytes + 1}, ErrLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := temporaryParent(t)
			_, err := WithArchive(context.Background(), bytes.NewReader(archive(t, tc.header)), func(context.Context, fs.FS) error { t.Fatal("invalid input reached consumer"); return nil })
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			assertClean(t, parent)
		})
	}
}
func TestArchiveRejectsLimitsDuplicatesAndTruncation(t *testing.T) {
	file := tar.Header{Name: "main.go", Typeflag: tar.TypeReg, Size: 1024}
	repeated := archive(t, file, file)
	caseAlias := archive(t, file, tar.Header{Name: "MAIN.go", Typeflag: tar.TypeReg})
	var entries []tar.Header
	for i := 0; i < MaxEntries+1; i++ {
		entries = append(entries, tar.Header{Name: fmtName(i), Typeflag: tar.TypeReg})
	}
	total := []tar.Header{}
	for i := 0; i < 5; i++ {
		total = append(total, tar.Header{Name: fmtName(i), Typeflag: tar.TypeReg, Size: MaxFileBytes})
	}
	valid := archive(t, file)
	for _, tc := range []struct {
		name  string
		input []byte
		want  error
	}{
		{"duplicate", repeated, ErrInvalid}, {"case alias", caseAlias, ErrInvalid},
		{"truncated", valid[:600], ErrInvalid}, {"empty", archive(t), ErrInvalid},
		{"entry count", archive(t, entries...), ErrLimit}, {"total bytes", archive(t, total...), ErrLimit},
		{"trailing data", append(append([]byte{}, valid...), []byte("hidden")...), ErrInvalid},
		{"input bytes", append(append([]byte{}, valid...), make([]byte, MaxArchiveBytes)...), ErrLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := temporaryParent(t)
			_, err := WithArchive(context.Background(), bytes.NewReader(tc.input), func(context.Context, fs.FS) error { t.Fatal("invalid input consumed"); return nil })
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			assertClean(t, parent)
		})
	}
}
func fmtName(i int) string { return "file-" + strconv.Itoa(i) }

func TestArchiveCancellationAndConsumerFailure(t *testing.T) {
	for _, mode := range []string{"before", "during", "consumer", "panic"} {
		t.Run(mode, func(t *testing.T) {
			parent := temporaryParent(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			data := archive(t, tar.Header{Name: "main.go", Typeflag: tar.TypeReg, Size: 1024})
			var input io.Reader = bytes.NewReader(data)
			if mode == "before" {
				cancel()
			}
			if mode == "during" {
				input = &cancellingReader{reader: input, cancel: cancel}
			}
			failure := errors.New("consumer failed")
			execute := func() {
				_, err := WithArchive(ctx, input, func(context.Context, fs.FS) error {
					if mode == "panic" {
						panic("test-only panic")
					}
					return failure
				})
				expected := failure
				if mode == "before" || mode == "during" {
					expected = context.Canceled
				}
				if !errors.Is(err, expected) {
					t.Fatalf("got %v want %v", err, expected)
				}
			}
			if mode == "panic" {
				func() {
					defer func() {
						if recover() == nil {
							t.Error("missing panic")
						}
					}()
					execute()
				}()
			} else {
				execute()
			}
			assertClean(t, parent)
		})
	}
}

type cancellingReader struct {
	reader io.Reader
	cancel context.CancelFunc
}

func (r *cancellingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.cancel()
	return n, err
}
