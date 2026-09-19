package stele

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

var relativePath = filepath.Rel

var digestScanRoots = []string{
	"openspec",
	"bin",
	"cmd",
	"internal",
	"pkg",
	"src",
	"public",
	"tools",
	"tests",
	"scripts",
}

var digestRootFiles = []string{
	"server.mjs",
	"package.json",
	"package-lock.json",
	"go.mod",
	"go.sum",
	"docs/.stele/config.toml",
	"stele.config.json",
	"artifacts/linkage-plan.json",
}

// repoFiles is how the verification read path reaches repository files. The
// CLI reads the disk; the language server reads its snapshot of the saved
// files, with the text of open documents on top. Paths are absolute.
type repoFiles interface {
	readFile(path string) ([]byte, error)
	// walk lists the accepted files below a directory like walkFiles.
	walk(directory string, accept func(string) bool) []string
	isFile(path string) bool
	// children lists the names of a directory's files and subdirectories.
	children(directory string) (files, directories []string)
}

// diskFiles reads the repository from disk.
type diskFiles struct{}

func (diskFiles) readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (diskFiles) walk(directory string, accept func(string) bool) []string {
	return walkFiles(directory, accept)
}

func (diskFiles) isFile(path string) bool {
	return fileExists(path)
}

func (diskFiles) children(directory string) ([]string, []string) {
	entries, _ := os.ReadDir(directory)
	files := make([]string, 0)
	directories := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, entry.Name())
		} else {
			files = append(files, entry.Name())
		}
	}
	return files, directories
}

func walkFiles(root string, accept func(string) bool) []string {
	files := make([]string, 0)
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Optional scan roots may not exist in every consumer project.
		}
		if !entry.IsDir() {
			if accept(path) {
				files = append(files, path)
			}
			return nil
		}
		if ignoredScanDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func ignoredScanDirectory(name string) bool {
	switch name {
	case ".git", "dist", "node_modules":
		return true
	default:
		return false
	}
}

func digestSource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".js", ".jsx", ".json", ".md", ".mjs", ".ts", ".tsx", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func inputFiles(repo repoFiles, root string) []string {
	files := make([]string, 0)
	for _, directory := range digestScanRoots {
		files = append(files, repo.walk(filepath.Join(root, directory), digestSource)...)
	}
	// Target folders outside the conventional roots are inputs too; a target
	// at the project root adds nothing, so generated artifacts stay out.
	for _, directory := range configuredTargetRoots(repo, root) {
		if directory != "." {
			files = append(files, repo.walk(filepath.Join(root, directory), digestSource)...)
		}
	}
	files = append(files, rootGoFiles(repo, root)...)
	for _, name := range digestRootFiles {
		path := filepath.Join(root, name)
		if repo.isFile(path) {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return slices.Compact(files)
}

// @implements req.execution.7e4755bd8f60
func ComputeInputDigest(root string) (string, error) {
	return computeInputDigest(diskFiles{}, root)
}

// computeInputDigest fingerprints the verified inputs as the repository
// files give them.
func computeInputDigest(repo repoFiles, root string) (string, error) {
	hash := sha256.New()
	for _, file := range inputFiles(repo, root) {
		relative, err := relativePath(root, file)
		if err != nil {
			return "", err
		}
		content, err := repo.readFile(file)
		if err != nil {
			return "", err
		}
		_, _ = hash.Write([]byte(filepath.ToSlash(relative)))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(content)
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
