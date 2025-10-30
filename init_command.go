package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	errShadowServiceExists = errors.New("shadow database service already exists")

	dockerComposeCandidates = []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}
)

func runInit(args []string) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintf(os.Stderr, "Usage: lockplane init <target> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Initialize Lockplane configuration and setup.\n\n")
		fmt.Fprintf(os.Stderr, "TARGETS:\n")
		fmt.Fprintf(os.Stderr, "  docker-compose    Add shadow database service to Docker Compose file\n\n")
		fmt.Fprintf(os.Stderr, "EXAMPLES:\n")
		fmt.Fprintf(os.Stderr, "  # Add shadow database to docker-compose.yml (auto-detected)\n")
		fmt.Fprintf(os.Stderr, "  lockplane init docker-compose\n\n")
		fmt.Fprintf(os.Stderr, "  # Specify custom docker compose file\n")
		fmt.Fprintf(os.Stderr, "  lockplane init docker-compose --file docker-compose.prod.yml\n\n")
		fmt.Fprintf(os.Stderr, "DESCRIPTION:\n\n")
		fmt.Fprintf(os.Stderr, "  The init command helps set up Lockplane for your project.\n\n")
		fmt.Fprintf(os.Stderr, "  docker-compose:\n")
		fmt.Fprintf(os.Stderr, "    Finds your Docker Compose file (docker-compose.yml, docker-compose.yaml,\n")
		fmt.Fprintf(os.Stderr, "    compose.yml, or compose.yaml) and adds a shadow database service.\n\n")
		fmt.Fprintf(os.Stderr, "    The shadow database is used to test migrations before applying them\n")
		fmt.Fprintf(os.Stderr, "    to your production database. Lockplane will:\n\n")
		fmt.Fprintf(os.Stderr, "    1. Detect your primary Postgres service (highest scoring match)\n")
		fmt.Fprintf(os.Stderr, "    2. Clone its configuration (image, environment variables, healthcheck)\n")
		fmt.Fprintf(os.Stderr, "    3. Add a \"shadow\" service on port 5433 (or next available port)\n")
		fmt.Fprintf(os.Stderr, "    4. Name the shadow database with \"_shadow\" suffix\n\n")
		fmt.Fprintf(os.Stderr, "    If a shadow service already exists, no changes are made.\n\n")
		if len(args) == 0 {
			os.Exit(1)
		}
		return
	}

	target := args[0]
	switch target {
	case "docker-compose":
		runInitDockerCompose(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown init target %q\n\n", target)
		fmt.Fprintf(os.Stderr, "Usage: lockplane init <target> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Available targets:\n")
		fmt.Fprintf(os.Stderr, "  docker-compose    Add shadow database service to Docker Compose file\n\n")
		os.Exit(1)
	}
}

func runInitDockerCompose(args []string) {
	fs := flag.NewFlagSet("init docker-compose", flag.ExitOnError)
	fileFlag := fs.String("file", "", "Path to docker compose file (auto-detected if not specified)")

	// Custom usage function
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lockplane init docker-compose [options]\n\n")
		fmt.Fprintf(os.Stderr, "Add a shadow database service to your Docker Compose file.\n\n")
		fmt.Fprintf(os.Stderr, "The shadow database is used by Lockplane to test migrations before\n")
		fmt.Fprintf(os.Stderr, "applying them to your production database. This ensures migrations are\n")
		fmt.Fprintf(os.Stderr, "safe and reversible.\n\n")
		fmt.Fprintf(os.Stderr, "HOW IT WORKS:\n\n")
		fmt.Fprintf(os.Stderr, "  1. Finds your Docker Compose file (looks for: docker-compose.yml,\n")
		fmt.Fprintf(os.Stderr, "     docker-compose.yaml, compose.yml, compose.yaml)\n")
		fmt.Fprintf(os.Stderr, "  2. Detects your primary Postgres service (by scoring image name,\n")
		fmt.Fprintf(os.Stderr, "     POSTGRES_DB, POSTGRES_USER, POSTGRES_PASSWORD)\n")
		fmt.Fprintf(os.Stderr, "  3. Creates a \"shadow\" service that:\n")
		fmt.Fprintf(os.Stderr, "     - Uses the same Postgres image\n")
		fmt.Fprintf(os.Stderr, "     - Clones environment variables from primary service\n")
		fmt.Fprintf(os.Stderr, "     - Names database with \"_shadow\" suffix (e.g., myapp_shadow)\n")
		fmt.Fprintf(os.Stderr, "     - Runs on port 5433 (or next available port)\n")
		fmt.Fprintf(os.Stderr, "     - Copies healthcheck configuration if present\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Auto-detect docker-compose.yml and add shadow service\n")
		fmt.Fprintf(os.Stderr, "  lockplane init docker-compose\n\n")
		fmt.Fprintf(os.Stderr, "  # Specify custom compose file\n")
		fmt.Fprintf(os.Stderr, "  lockplane init docker-compose --file docker-compose.prod.yml\n\n")
		fmt.Fprintf(os.Stderr, "  # After initialization, start databases:\n")
		fmt.Fprintf(os.Stderr, "  docker compose up -d\n\n")
	}

	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	composePath, err := locateDockerCompose(*fileFlag)
	if err != nil {
		log.Fatalf("Failed to locate docker compose file: %v", err)
	}

	info, err := addShadowDatabaseToCompose(composePath)
	if err != nil {
		if errors.Is(err, errShadowServiceExists) {
			fmt.Printf("Shadow database service already present in %s — no changes made.\n", composePath)
			return
		}
		log.Fatalf("Failed to add shadow database: %v", err)
	}

	fmt.Printf("Added shadow database service %q (DB=%s, port=%d) to %s\n", info.ServiceName, info.DatabaseName, info.Port, composePath)
}

type shadowAddition struct {
	ServiceName  string
	DatabaseName string
	Port         int
}

func locateDockerCompose(explicit string) (string, error) {
	if explicit != "" {
		if fileExists(explicit) {
			return explicit, nil
		}
		return "", fmt.Errorf("no docker compose file found at %q", explicit)
	}

	for _, candidate := range dockerComposeCandidates {
		if fileExists(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no docker compose file found (looked for %s)", strings.Join(dockerComposeCandidates, ", "))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func addShadowDatabaseToCompose(path string) (*shadowAddition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read compose file: %w", err)
	}

	var compose map[string]interface{}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, fmt.Errorf("parse compose file: %w", err)
	}

	servicesVal, ok := compose["services"]
	if !ok {
		servicesVal = map[string]interface{}{}
		compose["services"] = servicesVal
	}

	services, err := ensureStringMap(servicesVal)
	if err != nil {
		return nil, fmt.Errorf("expected services to be a mapping: %w", err)
	}

	const shadowServiceName = "shadow"
	if _, exists := services[shadowServiceName]; exists {
		return nil, errShadowServiceExists
	}

	_, primaryService, err := selectPrimaryService(services)
	if err != nil {
		return nil, err
	}

	baseEnv := extractEnvironment(primaryService)
	dbName := baseEnv["POSTGRES_DB"]
	if dbName == "" {
		dbName = "lockplane"
	}

	shadowDB := dbName
	if !strings.HasSuffix(strings.ToLower(shadowDB), "_shadow") {
		shadowDB = shadowDB + "_shadow"
	} else if shadowDB == dbName {
		shadowDB = dbName + "_shadow"
	}

	shadowEnv := make(map[string]string, len(baseEnv))
	for k, v := range baseEnv {
		shadowEnv[k] = v
	}
	shadowEnv["POSTGRES_DB"] = shadowDB
	if shadowEnv["POSTGRES_USER"] == "" {
		shadowEnv["POSTGRES_USER"] = "lockplane"
	}
	if shadowEnv["POSTGRES_PASSWORD"] == "" {
		shadowEnv["POSTGRES_PASSWORD"] = "lockplane"
	}

	image := getString(primaryService["image"])
	if image == "" {
		image = "postgres:16"
	}

	usedPorts := collectHostPorts(services)
	port := findNextPort(5433, usedPorts)

	shadowService := map[string]interface{}{
		"image":       image,
		"environment": shadowEnv,
		"ports":       []string{fmt.Sprintf("%d:5432", port)},
	}

	if healthcheck, ok := primaryService["healthcheck"]; ok {
		shadowService["healthcheck"] = healthcheck
	}

	services[shadowServiceName] = shadowService

	out, err := yaml.Marshal(compose)
	if err != nil {
		return nil, fmt.Errorf("serialize compose file: %w", err)
	}

	if err := os.WriteFile(path, out, 0o644); err != nil {
		return nil, fmt.Errorf("write compose file: %w", err)
	}

	return &shadowAddition{
		ServiceName:  shadowServiceName,
		DatabaseName: shadowDB,
		Port:         port,
	}, nil
}

func ensureStringMap(value interface{}) (map[string]interface{}, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		return v, nil
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(v))
		for key, val := range v {
			out[fmt.Sprint(key)] = val
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unexpected type %T", value)
	}
}

func selectPrimaryService(services map[string]interface{}) (string, map[string]interface{}, error) {
	if len(services) == 0 {
		return "", nil, fmt.Errorf("docker compose file has no services defined")
	}

	type candidate struct {
		name string
		svc  map[string]interface{}
	}

	serviceNames := make([]string, 0, len(services))
	for name := range services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)

	var best candidate
	bestScore := -1

	for _, name := range serviceNames {
		raw := services[name]
		svc, err := ensureStringMap(raw)
		if err != nil {
			continue
		}
		score := scoreService(svc)
		if score > bestScore {
			best = candidate{name: name, svc: svc}
			bestScore = score
		}
	}

	if bestScore == -1 {
		return "", nil, fmt.Errorf("no suitable postgres service found")
	}

	return best.name, best.svc, nil
}

func scoreService(service map[string]interface{}) int {
	score := 0
	image := strings.ToLower(getString(service["image"]))
	if strings.Contains(image, "postgres") {
		score += 2
	}

	env := extractEnvironment(service)
	if env["POSTGRES_DB"] != "" {
		score += 3
	}
	if env["POSTGRES_USER"] != "" {
		score++
	}
	if env["POSTGRES_PASSWORD"] != "" {
		score++
	}

	return score
}

func extractEnvironment(service map[string]interface{}) map[string]string {
	raw, ok := service["environment"]
	if !ok {
		return make(map[string]string)
	}

	result := make(map[string]string)

	switch v := raw.(type) {
	case map[string]interface{}:
		for key, val := range v {
			result[key] = getString(val)
		}
	case map[interface{}]interface{}:
		for key, val := range v {
			result[fmt.Sprint(key)] = getString(val)
		}
	case []interface{}:
		for _, entry := range v {
			parts := strings.SplitN(getString(entry), "=", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
		}
	case []string:
		for _, entry := range v {
			parts := strings.SplitN(entry, "=", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
		}
	}

	return result
}

func getString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

func collectHostPorts(services map[string]interface{}) map[int]struct{} {
	used := make(map[int]struct{})
	for _, raw := range services {
		svc, err := ensureStringMap(raw)
		if err != nil {
			continue
		}

		for _, port := range extractPorts(svc) {
			hostPort, ok := parseHostPort(port)
			if ok {
				used[hostPort] = struct{}{}
			}
		}
	}
	return used
}

func extractPorts(service map[string]interface{}) []string {
	raw, ok := service["ports"]
	if !ok {
		return nil
	}

	var ports []string
	switch v := raw.(type) {
	case []interface{}:
		for _, p := range v {
			ports = append(ports, getString(p))
		}
	case []string:
		ports = append(ports, v...)
	}
	return ports
}

func parseHostPort(entry string) (int, bool) {
	// Port definitions commonly look like "5432:5432" or "5432:5432/tcp".
	hostPart := entry
	if idx := strings.Index(entry, ":"); idx != -1 {
		hostPart = entry[:idx]
	}
	hostPart = strings.TrimSpace(hostPart)
	hostPart = strings.SplitN(hostPart, "/", 2)[0]
	if hostPart == "" {
		return 0, false
	}

	port, err := strconv.Atoi(hostPart)
	if err != nil {
		return 0, false
	}
	return port, true
}

func findNextPort(start int, used map[int]struct{}) int {
	port := start
	for {
		if _, ok := used[port]; !ok {
			return port
		}
		port++
	}
}
