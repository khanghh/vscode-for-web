package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	FileTypeFile    = "file"
	FileTypeDir     = "directory"
	FileTypeSymlink = "symlink"
	FileTypeSocket  = "socket"
	FileTypeFIFO    = "fifo"
	FileTypeUnknown = "unknown"
)

// FSHandler implements the File Explorer API under /api/fs
type FSHandler struct {
	svc LocalFileService
}

func NewFSHandler(svc LocalFileService) *FSHandler {
	return &FSHandler{svc: svc}
}

// helper: parse wildcard path from route, normalize to relative (no leading slash)
func (h *FSHandler) pathFromCtx(c *fiber.Ctx) string {
	p := c.Params("*")
	// if mounted at exact path without wildcard, fallback to empty
	if p == "" || p == "/" {
		return ""
	}
	// path may be URL-encoded by client
	if up, err := url.PathUnescape(p); err == nil {
		p = up
	}
	p = strings.TrimPrefix(p, "/")
	return p
}

// GET /api/v1/fs/*path
// - Directory: list as JSON array
// - File: return raw content; when download=true, set Content-Disposition
// - With stat=true: return JSON metadata for file or directory
func (h *FSHandler) Get(c *fiber.Ctx) error {
	rel := h.pathFromCtx(c)
	fi, err := h.svc.Stat(rel)
	if err != nil {
		return mapLocalFileServiceError(c, err)
	}

	if strings.EqualFold(c.Query("stat"), "true") {
		// Return metadata
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"type":         fileTypeOf(fi),
			"size":         fi.Size(),
			"lastModified": fi.ModTime().UTC().Format(time.RFC3339),
		})
	}

	if fi.IsDir() {
		items, err := h.svc.List(rel)
		if err != nil {
			return mapLocalFileServiceError(c, err)
		}
		// Format lastModified as RFC3339 per design doc
		type listEntry struct {
			Name         string `json:"name"`
			Type         string `json:"type"`
			Size         int64  `json:"size"`
			LastModified string `json:"lastModified"`
		}
		out := make([]listEntry, 0, len(items))
		for _, it := range items {
			out = append(out, listEntry{
				Name:         it.Name(),
				Type:         fileTypeOf(it),
				Size:         it.Size(),
				LastModified: it.ModTime().UTC().Format(time.RFC3339),
			})
		}
		return c.Status(fiber.StatusOK).JSON(out)
	}

	// File
	data, err := h.svc.ReadFile(rel)
	if err != nil {
		return mapLocalFileServiceError(c, err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	mime, _ := h.svc.DetectMIMEType(rel)
	if mime != "" {
		c.Set(fiber.HeaderContentType, mime)
	}
	c.Set(fiber.HeaderContentLength, fmt.Sprintf("%d", len(encoded)))
	if strings.EqualFold(c.Query("download"), "true") {
		// Force download
		c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filepath.Base(rel)))
	}
	return c.Send([]byte(encoded))
}

// fileTypeOf returns a string type for a given file info.
func fileTypeOf(info fs.FileInfo) string {
	mode := info.Mode()
	switch {
	case mode&fs.ModeSymlink != 0:
		return FileTypeSymlink
	case info.IsDir():
		return FileTypeDir
	case mode&fs.ModeSocket != 0:
		return FileTypeSocket
	case mode&fs.ModeNamedPipe != 0:
		return FileTypeFIFO
	case mode.IsRegular():
		return FileTypeFile
	default:
		return FileTypeUnknown
	}
}

// POST /api/v1/fs/*path
// - multipart/form-data with field "files": upload into directory path
// - application/json {"name": "new_folder"}: create subfolder under path
func (h *FSHandler) Post(c *fiber.Ctx) error {
	rel := h.pathFromCtx(c)
	ct := c.Get(fiber.HeaderContentType)
	overwrite := strings.EqualFold(c.Query("overwrite"), "true")

	if strings.HasPrefix(ct, fiber.MIMEMultipartForm) {
		// Upload files
		// Ensure target is a directory (or root)
		if rel != "" {
			if st, err := h.svc.Stat(rel); err != nil {
				return mapLocalFileServiceError(c, err)
			} else if !st.IsDir() {
				return badRequest(c, "target path is not a directory")
			}
		}
		mf, err := c.MultipartForm()
		if err != nil {
			return badRequest(c, "invalid multipart form")
		}
		files := mf.File["files"]
		if len(files) == 0 {
			return badRequest(c, "no files provided")
		}
		uploaded := make([]string, 0, len(files))
		for _, fh := range files {
			name := filepath.Base(fh.Filename)
			// open the uploaded file
			src, err := fh.Open()
			if err != nil {
				return mapLocalFileServiceError(c, err)
			}
			// destination rel path is rel/name (or just name at root)
			destRel := name
			if rel != "" {
				destRel = filepath.Join(rel, name)
			}
			err = h.svc.SaveStream(destRel, src, overwrite)
			_ = src.Close()
			if err != nil {
				return mapLocalFileServiceError(c, err)
			}
			uploaded = append(uploaded, name)
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"success":  true,
			"uploaded": uploaded,
		})
	}

	// Create folder from JSON
	var body struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil || strings.TrimSpace(body.Name) == "" {
		return badRequest(c, "invalid body: missing name")
	}
	newRel := body.Name
	if rel != "" {
		newRel = filepath.Join(rel, body.Name)
	}
	// ensure parent exists
	// MkdirAll will create parents, but we should ensure immediate parent exists per design's parent requirement
	parent := rel
	if parent != "" {
		if st, err := h.svc.Stat(parent); err != nil {
			return mapLocalFileServiceError(c, err)
		} else if !st.IsDir() {
			return badRequest(c, "parent is not a directory")
		}
	}
	// create folder (fail if exists)
	if st, err := h.svc.Stat(newRel); err == nil && st != nil {
		return c.Status(fiber.StatusConflict).JSON(errorMsg("folder exists"))
	}
	if err := h.svc.MkdirAll(newRel); err != nil {
		return mapLocalFileServiceError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"path":    newRel,
	})
}

// PUT /api/v1/fs/*path
// - File: replace content with request body
// - Directory: rename with query new_name
func (h *FSHandler) Put(c *fiber.Ctx) error {
	rel := h.pathFromCtx(c)
	fi, err := h.svc.Stat(rel)
	if err != nil {
		return mapLocalFileServiceError(c, err)
	}
	if fi.IsDir() {
		newName := c.Query("new_name")
		if strings.TrimSpace(newName) == "" {
			return badRequest(c, "missing new_name for directory rename")
		}
		if err := h.svc.RenameDir(rel, newName); err != nil {
			return mapLocalFileServiceError(c, err)
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true})
	}

	// File update
	// Use whole body for binary data
	data := c.Body()
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return badRequest(c, "invalid base64")
	}
	log.Printf("PUT %s: received %d bytes, decoded %d bytes, first 50: %x", rel, len(data), len(decoded), decoded[:min(50, len(decoded))])
	if err := h.svc.WriteFile(rel, decoded, false); err != nil {
		return mapLocalFileServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true})
}

// DELETE /api/v1/fs/*path
func (h *FSHandler) Delete(c *fiber.Ctx) error {
	rel := h.pathFromCtx(c)
	recursive := strings.EqualFold(c.Query("recursive"), "true")
	if recursive {
		if err := h.svc.DeleteRecursive(rel); err != nil {
			return mapLocalFileServiceError(c, err)
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true})
	}
	if err := h.svc.Delete(rel); err != nil {
		// Map directory-not-empty to 400 per design
		if strings.Contains(err.Error(), "directory not empty") {
			return c.Status(fiber.StatusBadRequest).JSON(errorMsg("directory not empty (use recursive=true)"))
		}
		return mapLocalFileServiceError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true})
}

// Helper functions
func mapLocalFileServiceError(c *fiber.Ctx, err error) error {
	if os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Path not found"})
	}
	if os.IsPermission(err) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Permission denied"})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
}

func badRequest(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
}

func errorMsg(msg string) fiber.Map {
	return fiber.Map{"error": msg}
}
