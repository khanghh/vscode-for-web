package core

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrPathTraversal  = errors.New("invalid path: traversal outside root is not allowed")
	ErrNotFound       = errors.New("path not found")
	ErrIsDirectory    = errors.New("path is a directory")
	ErrNotDirectory   = errors.New("path is not a directory")
	ErrAlreadyExists  = errors.New("already exists")
	ErrDirNotEmpty    = errors.New("directory not empty")
	ErrMissingNewName = errors.New("missing new name")
)

// LocalFileServiceImpl provides OS-backed file operations rooted at RootDir.
type LocalFileServiceImpl struct {
	RootDir string
}

// NewLocalFileService constructs a LocalFileServiceImpl with a sanitized absolute root.
func NewLocalFileService(rootDir string) *LocalFileServiceImpl {
	return &LocalFileServiceImpl{RootDir: rootDir}
}

// resolve joins the root and relative path, cleans it, and ensures it stays within RootDir.
func (s *LocalFileServiceImpl) resolve(rel string) (string, error) {
	// treat leading slash as relative from root, trim it
	rel = strings.TrimPrefix(rel, "/")
	joined := filepath.Join(s.RootDir, rel)
	cleaned := filepath.Clean(joined)
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", err
	}
	rootWithSep := s.RootDir
	if !strings.HasSuffix(rootWithSep, string(os.PathSeparator)) {
		rootWithSep += string(os.PathSeparator)
	}
	if abs != s.RootDir && !strings.HasPrefix(abs, rootWithSep) {
		return "", ErrPathTraversal
	}
	return abs, nil
}

// Stat returns os.FileInfo for the given relative path.
func (s *LocalFileServiceImpl) Stat(rel string) (os.FileInfo, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return fi, nil
}

// List lists a directory relative to root.
func (s *LocalFileServiceImpl) List(rel string) ([]os.FileInfo, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !fi.IsDir() {
		return nil, ErrNotDirectory
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	out := make([]os.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

// Open returns an opened file for reading; caller must Close.
func (s *LocalFileServiceImpl) Open(rel string) (*os.File, os.FileInfo, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	if fi.IsDir() {
		return nil, nil, ErrIsDirectory
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	return f, fi, nil
}

// ReadFile reads entire file into memory. For large files, prefer Open and streaming.
func (s *LocalFileServiceImpl) ReadFile(rel string) ([]byte, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if fi.IsDir() {
		return nil, ErrIsDirectory
	}
	return os.ReadFile(abs)
}

// WriteFile writes bytes to a file at rel. If create is false and the file doesn't exist, returns ErrNotFound.
func (s *LocalFileServiceImpl) WriteFile(rel string, data []byte, create bool) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	// ensure parent exists
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if !create {
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return ErrNotFound
			}
			return err
		}
	}
	return os.WriteFile(abs, data, 0o644)
}

// SaveStream writes an io.Reader to the destination file. Overwrites when overwrite==true.
func (s *LocalFileServiceImpl) SaveStream(rel string, r io.Reader, overwrite bool) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if !overwrite {
		if _, err := os.Stat(abs); err == nil {
			return ErrAlreadyExists
		}
	}
	tmp := abs + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, abs)
}

// Delete deletes a file or an empty directory.
func (s *LocalFileServiceImpl) Delete(rel string) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	if fi.IsDir() {
		// only remove if empty
		dir, err := os.Open(abs)
		if err != nil {
			return err
		}
		names, _ := dir.Readdirnames(1)
		dir.Close()
		if len(names) > 0 {
			return ErrDirNotEmpty
		}
		return os.Remove(abs)
	}
	return os.Remove(abs)
}

// DeleteRecursive deletes a file or directory recursively.
func (s *LocalFileServiceImpl) DeleteRecursive(rel string) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return os.RemoveAll(abs)
}

// MkdirAll creates a directory (and parents) at rel.
func (s *LocalFileServiceImpl) MkdirAll(rel string) error {
	abs, err := s.resolve(rel)
	if err != nil {
		return err
	}
	return os.MkdirAll(abs, 0o755)
}

// RenameDir renames/moves a file or directory to newPath
func (s *LocalFileServiceImpl) Rename(relPath string, newPath string, overwrite bool) error {
	if strings.TrimSpace(newPath) == "" {
		return ErrMissingNewName
	}

	// Check if current path exists
	absSrcPath, err := s.resolve(relPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absSrcPath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}

	// Ensure destination remains within root
	absDstPath, err := s.resolve(newPath)
	if err != nil {
		return err
	}

	// if overwrite is false, check existence and return error
	if !overwrite {
		if _, err := os.Stat(absDstPath); err == nil {
			return ErrAlreadyExists
		}
	}

	return os.Rename(absSrcPath, absDstPath)
}

// DetectMIME tries to infer MIME type by extension or content.
func (s *LocalFileServiceImpl) DetectMIMEType(rel string) (string, error) {
	abs, err := s.resolve(rel)
	if err != nil {
		return "", err
	}
	if ext := filepath.Ext(abs); ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" {
			return mt, nil
		}
	}
	// Fallback: read a small sample
	f, err := os.Open(abs)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := io.ReadFull(f, buf)
	return http.DetectContentType(buf[:n]), nil
}
