package shadow

import (
	"os"
	"testing"
	"time"
)

func TestReservationSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWD) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	res := &Reservation{
		Environment:  "local",
		ShadowURL:    "postgres://shadow",
		ShadowSchema: "lockplane_shadow",
		CreatedAt:    time.Now().UTC().Round(time.Second),
	}
	if err := SaveReservation(res); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadReservation()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected reservation")
	}
	if loaded.Environment != res.Environment || loaded.ShadowURL != res.ShadowURL || loaded.ShadowSchema != res.ShadowSchema {
		t.Fatalf("loaded reservation mismatch: %+v", loaded)
	}

	if err := ClearReservation(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	loaded, err = LoadReservation()
	if err != nil {
		t.Fatalf("load after clear: %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil reservation after clear")
	}
}
