package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type FileType int

const (
	FileTypeFile         FileType = 1
	FileTypeDirectory    FileType = 2
	FileTypeSymbolicLink FileType = 64
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
			Name         string   `json:"name"`
			Type         FileType `json:"type"`
			Size         int64    `json:"size"`
			LastModified string   `json:"lastModified"`
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
	mime, _ := h.svc.DetectMIMEType(rel)
	if mime != "" {
		c.Set(fiber.HeaderContentType, mime)
	}
	c.Set(fiber.HeaderContentLength, fmt.Sprintf("%d", len(data)))
	if strings.EqualFold(c.Query("download"), "true") {
		// Force download
		c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filepath.Base(rel)))
	}
	return c.Send(data)
}

// fileTypeOf returns a int type for a given file info.
func fileTypeOf(info fs.FileInfo) FileType {
	mode := info.Mode()
	if mode&fs.ModeSymlink != 0 {
		return FileTypeSymbolicLink
	}
	if info.IsDir() {
		return FileTypeDirectory
	}
	return FileTypeFile
}

// POST /api/v1/fs/*path?create=<bool>&overwrite=<bool>
func (h *FSHandler) Post(ctx *fiber.Ctx) error {
	rel := h.pathFromCtx(ctx)
	overwrite := strings.EqualFold(ctx.Query("overwrite"), "true")

	st, err := h.svc.Stat(rel)
	if err != nil {
		if os.IsNotExist(err) {
			return ctx.Status(fiber.StatusNotFound).JSON(errorMsg("target path not found"))
		}
		return mapLocalFileServiceError(ctx, err)
	}
	if !st.IsDir() {
		return badRequest(ctx, "target path is not a directory")
	}

	// Multipart upload -> upload files into directory

	ct := ctx.Get(fiber.HeaderContentType)
	if strings.HasPrefix(ct, fiber.MIMEMultipartForm) {
		return h.uploadFile(ctx, rel, overwrite)
	}

	// Try to parse JSON body for folder creation
	var body struct {
		Name string `json:"name"`
	}
	// If body has a "name" field, create subfolder
	if err := json.Unmarshal(ctx.Body(), &body); err == nil && strings.TrimSpace(body.Name) != "" {
		return h.createSubFolder(ctx, rel, strings.TrimSpace(body.Name))
	}

	// Unsupported body/type for POST
	return badRequest(ctx, "invalid request body")
}

// uploadFile handles multipart file uploads into an existing directory.
func (h *FSHandler) uploadFile(ctx *fiber.Ctx, rel string, overwrite bool) error {
	mf, err := ctx.MultipartForm()
	if err != nil {
		return badRequest(ctx, "invalid multipart form")
	}
	files := mf.File["files"]
	if len(files) == 0 {
		// Also accept single-file key for compatibility
		files = mf.File["file"]
	}
	if len(files) == 0 {
		return badRequest(ctx, "no files provided")
	}

	toUpload := files[0]
	name := filepath.Base(toUpload.Filename)
	destRel := filepath.Join(rel, name)

	// If overwrite is false, check existence and return 409 with code
	if !overwrite {
		if _, err := h.svc.Stat(destRel); err == nil {
			return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": fmt.Sprintf("file '%s' already exists", name),
				"code":  "FILE_EXISTS",
			})
		} else if !os.IsNotExist(err) {
			return mapLocalFileServiceError(ctx, err)
		}
	}

	src, err := toUpload.Open()
	if err != nil {
		return mapLocalFileServiceError(ctx, err)
	}
	if err := h.svc.SaveStream(destRel, src, overwrite); err != nil {
		_ = src.Close()
		return mapLocalFileServiceError(ctx, err)
	}
	_ = src.Close()
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "uploaded": name})
}

// createSubFolder creates a new subfolder under rel with the provided name.
func (h *FSHandler) createSubFolder(ctx *fiber.Ctx, rel, name string) error {
	newRel := filepath.Join(rel, name)
	if st, err := h.svc.Stat(newRel); err == nil && st != nil {
		return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "folder exists", "code": "FILE_EXISTS"})
	} else if err != nil && !os.IsNotExist(err) {
		return mapLocalFileServiceError(ctx, err)
	}
	if err := h.svc.MkdirAll(newRel); err != nil {
		return mapLocalFileServiceError(ctx, err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "path": newRel})
}

func (h *FSHandler) Put(ctx *fiber.Ctx) error {
	rel := h.pathFromCtx(ctx)
	overwrite := strings.EqualFold(ctx.Query("overwrite"), "true")

	if ctx.Get(fiber.HeaderContentType) != "application/octet-stream" {
		return badRequest(ctx, "expected application/octet-stream")
	}

	err := h.svc.SaveStream(rel, bytes.NewReader(ctx.Body()), overwrite)
	if err != nil {
		return mapLocalFileServiceError(ctx, err)
	}

	return ctx.SendStatus(fiber.StatusOK)
}

// PATCH /api/v1/fs/*path
// - Rename file or directory with body {"name": <new_name>}
func (h *FSHandler) Patch(c *fiber.Ctx) error {
	rel := h.pathFromCtx(c)
	var body struct {
		NewName string `json:"name"`
	}
	if err := c.BodyParser(&body); err != nil {
		return badRequest(c, "invalid json")
	}
	if strings.TrimSpace(body.NewName) == "" {
		return badRequest(c, "missing new name")
	}
	// Rename file or directory
	if err := h.svc.RenameDir(rel, body.NewName); err != nil {
		return mapLocalFileServiceError(c, err)
	}
	return c.SendStatus(fiber.StatusOK)
}

// DELETE /api/v1/fs/*path
func (h *FSHandler) Delete(c *fiber.Ctx) error {
	rel := h.pathFromCtx(c)
	recursive := strings.EqualFold(c.Query("recursive"), "true")
	if recursive {
		if err := h.svc.DeleteRecursive(rel); err != nil {
			return mapLocalFileServiceError(c, err)
		}
		return c.SendStatus(fiber.StatusOK)
	}
	if err := h.svc.Delete(rel); err != nil {
		// Map directory-not-empty to 400 per design
		if strings.Contains(err.Error(), "directory not empty") {
			return c.Status(fiber.StatusBadRequest).JSON(errorMsg("directory not empty (use recursive=true)"))
		}
		return mapLocalFileServiceError(c, err)
	}
	return c.SendStatus(fiber.StatusOK)
}

// Helper functions
func mapLocalFileServiceError(c *fiber.Ctx, err error) error {
	if os.IsNotExist(err) || strings.Contains(err.Error(), "not found") {
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
