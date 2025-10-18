package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAddShadowDatabaseToCompose(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")

	content := `services:
  db:
    image: postgres:16
    environment:
      POSTGRES_DB: notesapp
      POSTGRES_USER: appuser
      POSTGRES_PASSWORD: s3cret
    ports:
      - "5432:5432"
`
	if err := os.WriteFile(composePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write compose file: %v", err)
	}

	info, err := addShadowDatabaseToCompose(composePath)
	if err != nil {
		t.Fatalf("addShadowDatabaseToCompose returned error: %v", err)
	}

	if info.ServiceName != "shadow" {
		t.Fatalf("expected service name shadow, got %s", info.ServiceName)
	}
	if info.DatabaseName != "notesapp_shadow" {
		t.Fatalf("expected database name notesapp_shadow, got %s", info.DatabaseName)
	}
	if info.Port != 5433 {
		t.Fatalf("expected port 5433, got %d", info.Port)
	}

	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read compose file: %v", err)
	}

	var compose map[string]interface{}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("failed to parse compose file: %v", err)
	}

	services, err := ensureStringMap(compose["services"])
	if err != nil {
		t.Fatalf("failed to read services map: %v", err)
	}

	shadowRaw, ok := services["shadow"]
	if !ok {
		t.Fatalf("shadow service missing from compose output")
	}

	shadowService, err := ensureStringMap(shadowRaw)
	if err != nil {
		t.Fatalf("shadow service not a map: %v", err)
	}

	env := extractEnvironment(shadowService)
	if env["POSTGRES_DB"] != "notesapp_shadow" {
		t.Errorf("POSTGRES_DB expected notesapp_shadow, got %s", env["POSTGRES_DB"])
	}
	if env["POSTGRES_USER"] != "appuser" {
		t.Errorf("POSTGRES_USER expected appuser, got %s", env["POSTGRES_USER"])
	}
	if env["POSTGRES_PASSWORD"] != "s3cret" {
		t.Errorf("POSTGRES_PASSWORD expected s3cret, got %s", env["POSTGRES_PASSWORD"])
	}

	ports := extractPorts(shadowService)
	if len(ports) != 1 {
		t.Fatalf("expected 1 port mapping, got %d", len(ports))
	}
	hostPort, ok := parseHostPort(ports[0])
	if !ok {
		t.Fatalf("failed to parse host port from %q", ports[0])
	}
	if hostPort != info.Port {
		t.Errorf("expected host port %d, got %d", info.Port, hostPort)
	}
}

func TestAddShadowDatabaseToComposeAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")

	content := `services:
  db:
    image: postgres:16
  shadow:
    image: postgres:16
    environment:
      POSTGRES_DB: notesapp_shadow
`
	if err := os.WriteFile(composePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write compose file: %v", err)
	}

	_, err := addShadowDatabaseToCompose(composePath)
	if err == nil {
		t.Fatalf("expected error when shadow service already exists, got nil")
	}
	if !errors.Is(err, errShadowServiceExists) {
		t.Fatalf("expected errShadowServiceExists, got %v", err)
	}
}

func TestAddShadowDatabaseWithListEnvironment(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")

	content := `services:
  database:
    image: postgres:16
    environment:
      - POSTGRES_DB=appdb
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=password
    ports:
      - "5433:5432"
`
	if err := os.WriteFile(composePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write compose file: %v", err)
	}

	info, err := addShadowDatabaseToCompose(composePath)
	if err != nil {
		t.Fatalf("addShadowDatabaseToCompose returned error: %v", err)
	}

	if info.DatabaseName != "appdb_shadow" {
		t.Fatalf("expected database name appdb_shadow, got %s", info.DatabaseName)
	}
	if info.Port != 5434 {
		t.Fatalf("expected port 5434 (next free), got %d", info.Port)
	}

	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read compose file: %v", err)
	}

	var compose map[string]interface{}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("failed to parse compose file: %v", err)
	}

	services, err := ensureStringMap(compose["services"])
	if err != nil {
		t.Fatalf("failed to read services map: %v", err)
	}

	shadowRaw := services["shadow"]
	shadowService, err := ensureStringMap(shadowRaw)
	if err != nil {
		t.Fatalf("shadow service not a map: %v", err)
	}

	env := extractEnvironment(shadowService)
	if env["POSTGRES_PASSWORD"] != "password" {
		t.Errorf("expected POSTGRES_PASSWORD=password, got %s", env["POSTGRES_PASSWORD"])
	}
}
