package config

import (
	"errors"
	"os"
	"path/filepath"
	"pitstop/internal/model"
	"strings"
	"testing"
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
