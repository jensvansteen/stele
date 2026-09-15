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

func walkFiles(root string, accept func(string) bool) []string {
	files := make([]string, 0)
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Optional scan roots may not exist in every consumer project.
		}
		if entry.IsDir() {
			if entry.Name() == "node_modules" || entry.Name() == ".git" || entry.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if accept(path) {
			files = append(files, path)
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

func ComputeInputDigest(root string) (string, error) {
	files := make([]string, 0)
	for _, item := range []string{"openspec", "bin", "cmd", "internal", "src", "public", "tools", "tests", "scripts"} {
		files = append(files, walkFiles(filepath.Join(root, item), func(path string) bool {
			ext := strings.ToLower(filepath.Ext(path))
			return ext == ".md" || ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".mjs" || ext == ".js" || ext == ".go" || ext == ".ts" || ext == ".tsx" || ext == ".jsx"
		})...)
	}
	for _, item := range []string{"server.mjs", "package.json", "package-lock.json", "go.mod", "go.sum", "docs/.stele/config.toml", "stele.config.json", "artifacts/linkage-plan.json"} {
		path := filepath.Join(root, item)
		if fileExists(path) {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	hash := sha256.New()
	for _, file := range files {
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
