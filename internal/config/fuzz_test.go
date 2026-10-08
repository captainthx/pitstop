package config

import (
	"encoding/json"
	"reflect"
	"testing"

	"pitstop/internal/model"
)

func FuzzParse(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}`),
		[]byte(`{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"line 1\nline 2"}]}]}`),
		[]byte(`{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true","id":"other"}]}]}`),
		[]byte(`{"schemaVersion":1,"workflow":"ci.yml"}`),
		[]byte(`null`),
		[]byte("\ufeff{}"),
		[]byte(`{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"\u0000"}]}]}`),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := Parse(data)
		if err != nil {
			if !reflect.DeepEqual(got, model.Config{}) {
				t.Fatalf("error returned partial config: %#v", got)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted config does not validate: %v", err)
		}
	})
}

func FuzzConfigRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{"schemaVersion":2,"runner":{"image":"alpine:3.22","architecture":"linux/arm64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","run":"true"}]}]}`),
		[]byte(`{"schemaVersion":2,"runner":{"image":"node:22-bookworm","architecture":"linux/amd64"},"jobs":["verify"],"checks":[{"id":"verify","steps":[{"id":"check","name":"ทดสอบ","run":"printf '%s' \"$HOME\""}]}]}`),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := Parse(data)
		if err != nil {
			if !reflect.DeepEqual(got, model.Config{}) {
				t.Fatalf("error returned partial config: %#v", got)
			}
			return
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		round, err := Parse(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, round) {
			t.Fatalf("round trip changed config: %#v != %#v", got, round)
		}
		for i := range got.Checks {
			for j := range got.Checks[i].Steps {
				if got.Checks[i].Steps[j].Run != round.Checks[i].Steps[j].Run {
					t.Fatalf("run command changed during round trip")
				}
			}
		}
	})
}
