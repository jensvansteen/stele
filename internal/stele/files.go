package stele

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var relativePath = filepath.Rel

var digestScanRoots = []string{
	"openspec",
	"bin",
	"cmd",
	"internal",
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

func inputFiles(root string) []string {
	files := make([]string, 0)
	for _, directory := range digestScanRoots {
		files = append(files, walkFiles(filepath.Join(root, directory), digestSource)...)
	}
	for _, name := range digestRootFiles {
		path := filepath.Join(root, name)
		if fileExists(path) {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files
}

func ComputeInputDigest(root string) (string, error) {
	hash := sha256.New()
	for _, file := range inputFiles(root) {
		relative, err := relativePath(root, file)
		if err != nil {
			return "", err
		}
		content, err := os.ReadFile(file)
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
