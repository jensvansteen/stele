package stele

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// lspFileEntry is one saved file of the server's snapshot: its size and
// modification time, and its content once read.
type lspFileEntry struct {
	size    int64
	modTime time.Time
	content []byte
	loaded  bool
	// anchors are the file's anchors once scanned as anchorKind.
	anchors    []Anchor
	anchorKind string
}

// lspFiles is the server's snapshot of a project's input files: every file
// the verification read path can reach, read from disk once and again only
// after it changed, and the text of open documents on top.
type lspFiles struct {
	root     string
	entries  map[string]*lspFileEntry
	overlays map[string][]byte
	sorted   []string
}

// newLSPFiles lists a project's input files.
func newLSPFiles(root string) *lspFiles {
	return &lspFiles{root: root, entries: scanLSPFiles(root), overlays: map[string][]byte{}}
}

// lspTrackedRoots are the top-level directories whose files the snapshot keeps.
func lspTrackedRoots() []string {
	roots := slices.Concat([]string{"openspec"}, anchorScanRoots, digestScanRoots)
	slices.Sort(roots)
	return slices.Compact(roots)
}

// trackedPath reports whether a repository path is an input the snapshot
// keeps: any file under openspec/, a source or digest input under a scanned
// directory, a file in the root, or one of the listed artifacts.
//
// @implements req.languageserver.ab7fbe14be03
func trackedPath(relative string) bool {
	parts := strings.Split(relative, "/")
	if slices.ContainsFunc(parts[:len(parts)-1], ignoredScanDirectory) {
		return false
	}
	switch {
	case len(parts) == 1:
		return true
	case parts[0] == "openspec":
		return true
	case slices.Contains(lspTrackedRoots(), parts[0]):
		return digestSource(relative) || supportedSource(relative)
	default:
		return relative == defaultEvidencePath || slices.Contains(digestRootFiles, relative)
	}
}

// scanLSPFiles lists the tracked files of a project with their size and
// modification time.
func scanLSPFiles(root string) map[string]*lspFileEntry {
	entries := make(map[string]*lspFileEntry)
	add := func(path string) {
		relative := repositoryPath(root, path)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && trackedPath(relative) {
			entries[relative] = &lspFileEntry{size: info.Size(), modTime: info.ModTime()}
		}
	}
	for _, directory := range lspTrackedRoots() {
		for _, path := range walkFiles(filepath.Join(root, directory), func(string) bool { return true }) {
			add(path)
		}
	}
	names, _ := diskFiles{}.children(root)
	for _, name := range append(names, defaultEvidencePath) {
		add(filepath.Join(root, filepath.FromSlash(name)))
	}
	for _, name := range digestRootFiles {
		add(filepath.Join(root, filepath.FromSlash(name)))
	}
	return entries
}

// relative returns the repository path of an absolute path under the root.
func (files *lspFiles) relative(path string) (string, bool) {
	slashed := filepath.ToSlash(path)
	root := filepath.ToSlash(files.root)
	if slashed == root {
		return "", true
	}
	relative, found := strings.CutPrefix(slashed, root+"/")
	return relative, found
}

// keys returns the listed paths, sorted.
func (files *lspFiles) keys() []string {
	if files.sorted == nil {
		files.sorted = make([]string, 0, len(files.entries))
		for key := range files.entries {
			files.sorted = append(files.sorted, key)
		}
		sort.Strings(files.sorted)
	}
	return files.sorted
}

// update re-reads one file's state from disk: a new or changed file is listed
// without content, and a missing one is dropped with everything below it.
func (files *lspFiles) update(relative string) {
	files.sorted = nil
	path := filepath.Join(files.root, filepath.FromSlash(relative))
	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		for _, file := range walkFiles(path, func(string) bool { return true }) {
			files.update(repositoryPath(files.root, file))
		}
	case err == nil && trackedPath(relative):
		files.entries[relative] = &lspFileEntry{size: info.Size(), modTime: info.ModTime()}
	default:
		delete(files.entries, relative)
		for key := range files.entries {
			if strings.HasPrefix(key, relative+"/") {
				delete(files.entries, key)
			}
		}
	}
}

// refresh compares the listing with the disk by size and modification time
// and reports whether anything changed.
func (files *lspFiles) refresh() bool {
	fresh := scanLSPFiles(files.root)
	changed := len(fresh) != len(files.entries)
	for key, entry := range fresh {
		old, listed := files.entries[key]
		if listed && old.size == entry.size && old.modTime.Equal(entry.modTime) {
			fresh[key] = old
			continue
		}
		changed = true
	}
	if changed {
		files.entries = fresh
		files.sorted = nil
	}
	return changed
}

// lspView reads the snapshot as repository files: the saved files, and with
// overlays the text of open documents instead.
type lspView struct {
	files    *lspFiles
	overlays bool
}

// view returns the snapshot as repository files.
func (files *lspFiles) view(overlays bool) repoFiles {
	return lspView{files: files, overlays: overlays}
}

func (view lspView) overlay(relative string) ([]byte, bool) {
	if !view.overlays {
		return nil, false
	}
	content, open := view.files.overlays[relative]
	return content, open
}

func (view lspView) readFile(path string) ([]byte, error) {
	relative, _ := view.files.relative(path)
	if content, open := view.overlay(relative); open {
		return content, nil
	}
	entry, listed := view.files.entries[relative]
	if !listed {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}
	if !entry.loaded {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		entry.content, entry.loaded = content, true
	}
	return entry.content, nil
}

// cachedAnchors returns a saved file's anchors from its last scan, and scans
// open documents and changed files again.
func (view lspView) cachedAnchors(file anchorFile, scan func() ([]Anchor, error)) ([]Anchor, error) {
	relative, _ := view.files.relative(file.path)
	entry := view.files.entries[relative]
	if _, open := view.overlay(relative); open || entry == nil {
		return scan()
	}
	if entry.anchorKind != file.kind {
		anchors, err := scan()
		if err != nil {
			return nil, err
		}
		entry.anchors, entry.anchorKind = anchors, file.kind
	}
	return entry.anchors, nil
}

func (view lspView) isFile(path string) bool {
	relative, _ := view.files.relative(path)
	_, open := view.overlay(relative)
	_, listed := view.files.entries[relative]
	return open || listed
}

// paths lists the snapshot's paths below a directory, with open documents.
func (view lspView) paths(directory string) []string {
	relative, inside := view.files.relative(directory)
	if !inside {
		return nil
	}
	prefix := choose(relative == "", "", relative+"/")
	paths := make([]string, 0)
	for _, key := range view.files.keys() {
		if strings.HasPrefix(key, prefix) {
			paths = append(paths, key)
		}
	}
	if view.overlays {
		for key := range view.files.overlays {
			if strings.HasPrefix(key, prefix) && view.files.entries[key] == nil {
				paths = append(paths, key)
			}
		}
		sort.Strings(paths)
	}
	return paths
}

func (view lspView) walk(directory string, accept func(string) bool) []string {
	files := make([]string, 0)
	for _, key := range view.paths(directory) {
		path := filepath.Join(view.files.root, filepath.FromSlash(key))
		if accept(path) {
			files = append(files, path)
		}
	}
	return files
}

func (view lspView) children(directory string) ([]string, []string) {
	relative, _ := view.files.relative(directory)
	prefix := choose(relative == "", "", relative+"/")
	files := make([]string, 0)
	directories := make([]string, 0)
	for _, key := range view.paths(directory) {
		name, _, nested := strings.Cut(strings.TrimPrefix(key, prefix), "/")
		switch {
		case !nested:
			files = append(files, name)
		case !slices.Contains(directories, name):
			directories = append(directories, name)
		}
	}
	// Like os.ReadDir, names are sorted as names, not as paths.
	sort.Strings(files)
	sort.Strings(directories)
	return files, directories
}
