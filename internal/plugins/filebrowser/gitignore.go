package filebrowser

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// GitIgnoreFilter loads and applies .gitignore rules.
type GitIgnoreFilter struct {
	root   string
	specs  []pathSpec
}

type pathSpec struct {
	path string
	patterns []pattern
}

type pattern struct {
	raw      string
	negate   bool
	dirOnly  bool
}

// NewGitIgnoreFilter creates a filter from the given path up to the nearest .git root.
func NewGitIgnoreFilter(path string) (*GitIgnoreFilter, error) {
	filter := &GitIgnoreFilter{root: path}

	// Walk up to find .git root and collect .gitignore files
	current := path
	for {
		gitDir := filepath.Join(current, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			filter.root = current
			break
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	// Collect all .gitignore files from root down to path
	if err := filepath.WalkDir(filter.root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && !strings.HasPrefix(p, filter.root) {
			return filepath.SkipDir
		}
		if d.Name() == ".gitignore" {
			if spec, err := loadGitIgnoreFile(p); err == nil {
				filter.specs = append(filter.specs, spec)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return filter, nil
}

func loadGitIgnoreFile(path string) (pathSpec, error) {
	f, err := os.Open(path)
	if err != nil {
		return pathSpec{}, err
	}
	defer f.Close()

	spec := pathSpec{path: filepath.Dir(path)}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pat := pattern{raw: line}
		if strings.HasPrefix(line, "!") {
			pat.negate = true
			pat.raw = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			pat.dirOnly = true
			pat.raw = strings.TrimSuffix(pat.raw, "/")
		}
		spec.patterns = append(spec.patterns, pat)
	}
	return spec, scanner.Err()
}

// Match returns true if the path should be ignored.
func (f *GitIgnoreFilter) Match(relPath string, isDir bool) bool {
	if f == nil {
		return false
	}

	// Check each spec (from root down)
	ignored := false
	for _, spec := range f.specs {
		// Only apply specs from directories that are ancestors of the target path
		if !strings.HasPrefix(relPath, spec.path) && spec.path != f.root {
			continue
		}

		for _, pat := range spec.patterns {
			if pat.dirOnly && !isDir {
				continue
			}
			if matchPattern(relPath, pat.raw) {
				ignored = !pat.negate
			}
		}
	}
	return ignored
}

// Simple glob matching for gitignore patterns.
func matchPattern(path, pattern string) bool {
	// Handle **/ prefix
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[3:]
		return strings.HasSuffix(path, suffix) || strings.Contains(path, "/"+suffix)
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(path, parts[0]) && strings.HasSuffix(path, parts[1])
		}
	}

	// Handle trailing /*
	if strings.HasSuffix(pattern, "/*") {
		prefix := pattern[:len(pattern)-2]
		return strings.HasPrefix(path, prefix+"/")
	}

	// Exact match or basename match
	if path == pattern {
		return true
	}
	if filepath.Base(path) == pattern {
		return true
	}

	return false
}
