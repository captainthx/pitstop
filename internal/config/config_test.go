package config

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"pitstop/internal/model"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

const validConfig = `{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}`

func TestLoadStrictAndMigration(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, ".pitstop.json")
	if err := os.WriteFile(path, []byte(validConfig), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(validConfig+"\n{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err == nil {
		t.Fatal("expected trailing document rejection")
	}
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"workflow":"ci.yml"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err == nil {
		t.Fatal("expected migration error")
	} else {
		var me *model.Error
		if !errors.As(err, &me) || me.Code != "CONFIG_INVALID" || !strings.Contains(me.Message, "schemaVersion 2") {
			t.Fatal(err)
		}
	}
}
func TestLoadRejectsUnknownFields(t *testing.T) {
	d := t.TempDir()
	b := []byte(`{"schemaVersion":2,"workflow":"ci.yml","runner":{"image":"x","architecture":"linux/arm64"},"jobs":["x"],"checks":[{"id":"x","steps":[{"id":"s","run":"true"}]}]}`)
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestLoadRejectsSpecialFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file types are outside the supported platform promise")
	}
	t.Run("missing", func(t *testing.T) {
		if _, err := Load(t.TempDir()); err == nil {
			t.Fatal("missing config accepted")
		}
	})
	t.Run("directory", func(t *testing.T) {
		d := t.TempDir()
		if err := os.Mkdir(filepath.Join(d, ".pitstop.json"), 0700); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(d); err == nil {
			t.Fatal("directory accepted")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		d := t.TempDir()
		target := filepath.Join(d, "target")
		if err := os.WriteFile(target, []byte(validConfig), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(d, ".pitstop.json")); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(d); err == nil {
			t.Fatal("symlink accepted")
		}
	})
	t.Run("fifo", func(t *testing.T) {
		d := t.TempDir()
		path := filepath.Join(d, ".pitstop.json")
		if err := syscall.Mkfifo(path, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLoadFIFOHelper$")
		cmd.Env = append(os.Environ(), "PITSTOP_FIFO_REPO="+d)
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				t.Fatalf("FIFO read did not return before deadline: %v", ctx.Err())
			}
			t.Fatalf("FIFO helper failed: %v", err)
		}
	})
	t.Run("socket", func(t *testing.T) {
		d, err := os.MkdirTemp("/tmp", "ps-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(d)
		path := filepath.Join(d, ".pitstop.json")
		listener, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		if _, err := Load(d); err == nil {
			t.Fatal("socket accepted")
		}
	})
}

func TestLoadFIFOHelper(t *testing.T) {
	repo := os.Getenv("PITSTOP_FIFO_REPO")
	if repo == "" {
		return
	}
	if _, err := Load(repo); err == nil {
		t.Fatal("FIFO accepted")
	}
}

func TestLoadBoundsOpenedFile(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, ".pitstop.json"), []byte(strings.Repeat("x", model.MaxConfigBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(d); err == nil {
		t.Fatal("oversized config accepted")
	}
}
