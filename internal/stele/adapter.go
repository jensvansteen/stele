package stele

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const defaultAdapter = "openspec"

// specificationBackend is the seam between Stele and the tool that owns the
// behavior specification. Every command reaches specification files, plans,
// validation, and backend artifacts through it.
type specificationBackend interface {
	Name() string
	// SpecFiles lists the specification files of a scope.
	SpecFiles(root string, scope verificationScope) []string
	// ParseSpecs reads the requirements and scenarios of a scope.
	ParseSpecs(root string, scope verificationScope) (ParsedSpecs, error)
	// DeclaredIdentities returns every identity declared anywhere in the backend.
	DeclaredIdentities(root string) (map[string]bool, error)
	// IdentityFiles lists the files whose identity tokens a new ID must avoid.
	IdentityFiles(root string) []string
	// Capability names the capability of a change's specification file.
	Capability(scope verificationScope, relative string) string
	// CurrentSpecFile locates the current specification of a capability.
	CurrentSpecFile(root, capability string) string
	// PlanPaths lists the linkage plans of a scope, oldest first.
	PlanPaths(root string, scope verificationScope) []string
	// Validate runs the backend's own strict validation.
	Validate(root string, scope verificationScope) (bool, error)
	// Install prepares the backend's files for the Stele workflow.
	Install(root string, setup openSpecSetup) ([]string, error)
	// VersionDrift describes backend files, or with lookPath a backend tool on
	// PATH, that do not match the pinned backend version.
	VersionDrift(root string, lookPath bool) []string
}

// specificationBackends registers the supported backends by adapter name.
var specificationBackends = map[string]specificationBackend{defaultAdapter: openSpecBackend{}}

// resolveBackend returns the backend named by the adapter setting, OpenSpec
// when the setting is empty, and an error listing the supported names
// otherwise.
//
// @implements req.backend.b5ff52883615
func resolveBackend(adapter string) (specificationBackend, error) {
	if adapter == "" {
		adapter = defaultAdapter
	}
	if backend, supported := specificationBackends[adapter]; supported {
		return backend, nil
	}
	names := make([]string, 0, len(specificationBackends))
	for name := range specificationBackends {
		names = append(names, name)
	}
	sort.Strings(names)
	return nil, fmt.Errorf("unsupported adapter %q in stele.config.json; supported adapters: %s",
		adapter, strings.Join(names, ", "))
}

// readConfig reads stele.config.json, returning an empty configuration when
// the file does not exist.
func readConfig(root string) (Config, error) {
	var config Config
	content, err := os.ReadFile(filepath.Join(root, "stele.config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(content, &config); err != nil {
		return config, fmt.Errorf("invalid stele.config.json: %w", err)
	}
	return config, nil
}

// configuredBackend resolves the backend from an existing configuration file,
// for init, which runs before a configuration is required.
func configuredBackend(root string) (specificationBackend, error) {
	config, err := readConfig(root)
	if err != nil {
		return nil, err
	}
	return resolveBackend(config.Adapter)
}

// openSpecBackend reads and validates OpenSpec projects with the bundled,
// pinned OpenSpec CLI.
type openSpecBackend struct{}

func (openSpecBackend) Name() string {
	return defaultAdapter
}

func (openSpecBackend) specsRoot(root string, scope verificationScope) string {
	if scope.currentSpecs {
		return filepath.Join(root, "openspec", "specs")
	}
	return filepath.Join(root, "openspec", "changes", scope.changeID, "specs")
}

func (backend openSpecBackend) SpecFiles(root string, scope verificationScope) []string {
	return walkFiles(backend.specsRoot(root, scope), isMarkdown)
}

func (backend openSpecBackend) ParseSpecs(root string, scope verificationScope) (ParsedSpecs, error) {
	return parseSpecFiles(root, backend.SpecFiles(root, scope))
}

func (openSpecBackend) DeclaredIdentities(root string) (map[string]bool, error) {
	return declaredIdentities(root)
}

func (openSpecBackend) IdentityFiles(root string) []string {
	return walkFiles(filepath.Join(root, "openspec"), isMarkdown)
}

// Capability returns "todo" for specs/todo/spec.md and "platform/auth" for
// specs/platform/auth/spec.md.
func (openSpecBackend) Capability(scope verificationScope, relative string) string {
	withinSpecs := strings.TrimPrefix(relative, "openspec/changes/"+scope.changeID+"/specs/")
	directory := path.Dir(withinSpecs)
	if directory == "." {
		return strings.TrimSuffix(withinSpecs, path.Ext(withinSpecs))
	}
	return directory
}

func (openSpecBackend) CurrentSpecFile(root, capability string) string {
	return filepath.Join(root, "openspec", "specs", filepath.FromSlash(capability), "spec.md")
}

func (openSpecBackend) PlanPaths(root string, scope verificationScope) []string {
	if scope.currentSpecs {
		return walkFiles(filepath.Join(root, "openspec", "changes", "archive"), func(file string) bool {
			return filepath.Base(file) == linkagePlanFile
		})
	}
	return []string{filepath.Join(root, "openspec", "changes", scope.changeID, linkagePlanFile)}
}

func (openSpecBackend) Validate(root string, scope verificationScope) (bool, error) {
	return runOpenSpec(root, scope)
}

func (openSpecBackend) Install(root string, setup openSpecSetup) ([]string, error) {
	return setUpOpenSpec(root, setup)
}

func (openSpecBackend) VersionDrift(root string, lookPath bool) []string {
	return openSpecVersionDrift(root, lookPath)
}

func isMarkdown(file string) bool {
	return strings.EqualFold(filepath.Ext(file), ".md")
}

var (
	generatedByPattern = regexp.MustCompile(`(?m)^\s*generatedBy:\s*["']?([^"'\s]+)["']?\s*$`)
	schemaForkPattern  = regexp.MustCompile(`forked from OpenSpec (\S+) `)
	lookPathFunction   = exec.LookPath
	openSpecVersionOf  = runOpenSpecVersion
)

const openSpecVersionTimeout = 10 * time.Second

// openSpecVersionDrift compares the versions recorded in a project's OpenSpec
// skills and stele schema fork, and with lookPath the openspec on PATH, with
// the pinned OpenSpec version.
//
// @implements req.backend.17e80d964275
func openSpecVersionDrift(root string, lookPath bool) []string {
	findings := make([]string, 0)
	// OpenSpec writes skills to <tool directory>/skills; a linked directory is read once.
	directories, _ := filepath.Glob(filepath.Join(root, ".*", "skills"))
	seen := make(map[string]bool)
	for _, directory := range directories {
		resolved, err := filepath.EvalSymlinks(directory)
		if err != nil || seen[resolved] {
			continue
		}
		seen[resolved] = true
		if version := skillsVersion(directory); version != "" {
			relative := filepath.ToSlash(strings.TrimPrefix(directory, root+string(filepath.Separator)))
			findings = append(findings, fmt.Sprintf(
				"OpenSpec skills in %s were generated by OpenSpec %s, but Stele pins OpenSpec %s. "+
					"Run `npx openspec update` to regenerate them with the pinned version.",
				relative, version, OpenSpecVersion))
		}
	}
	schema, _ := os.ReadFile(filepath.Join(root, "openspec", "schemas", "stele", "schema.yaml"))
	if match := schemaForkPattern.FindSubmatch(schema); match != nil && string(match[1]) != OpenSpecVersion {
		findings = append(findings, fmt.Sprintf(
			"The stele schema was forked from OpenSpec %s, but Stele pins OpenSpec %s. "+
				"Run `stele init --refresh-schema` to fork it again.",
			match[1], OpenSpecVersion))
	}
	if lookPath {
		findings = append(findings, pathOpenSpecDrift(root)...)
	}
	return findings
}

// skillsVersion returns the first OpenSpec version other than the pinned one
// that the OpenSpec skills in a directory record, or "".
func skillsVersion(directory string) string {
	skills, _ := filepath.Glob(filepath.Join(directory, "openspec-*", "SKILL.md"))
	sort.Strings(skills)
	for _, skill := range skills {
		content, err := os.ReadFile(skill)
		if err != nil {
			continue
		}
		if match := generatedByPattern.FindSubmatch(content); match != nil && string(match[1]) != OpenSpecVersion {
			return string(match[1])
		}
	}
	return ""
}

// pathOpenSpecDrift reports an openspec on PATH, other than the bundled CLI,
// that reports another version.
func pathOpenSpecDrift(root string) []string {
	found, err := lookPathFunction("openspec")
	if err != nil {
		return nil
	}
	resolved, err := filepath.EvalSymlinks(found)
	if err != nil {
		resolved = found
	}
	if bundled, err := openSpecCLI(root); err == nil && resolved == bundled {
		return nil
	}
	version := openSpecVersionOf(found)
	if version == "" || version == OpenSpecVersion {
		return nil
	}
	return []string{fmt.Sprintf(
		"The openspec on PATH (%s) is version %s, but Stele uses its bundled OpenSpec %s. "+
			"Run `npx openspec` to use the version pinned with Stele.",
		found, version, OpenSpecVersion)}
}

func runOpenSpecVersion(executable string) string {
	ctx, cancel := context.WithTimeout(context.Background(), openSpecVersionTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "--version")
	command.Env = append(os.Environ(), "OPENSPEC_TELEMETRY=0")
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
