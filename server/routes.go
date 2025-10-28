package main

import (
	"io"
	"os"

	"github.com/gofiber/fiber/v2"
)

type LocalFileService interface {
	Stat(relPath string) (os.FileInfo, error)
	List(relPath string) ([]os.FileInfo, error)
	Open(relPath string) (*os.File, os.FileInfo, error)
	ReadFile(relPath string) ([]byte, error)
	WriteFile(relPath string, data []byte, create bool) error
	SaveStream(relPath string, reader io.Reader, overwrite bool) error
	Delete(relPath string) error
	DeleteRecursive(relPath string) error
	MkdirAll(relPath string) error
	RenameDir(oldRelPath, newRelPath string) error
	DetectMIMEType(relPath string) (string, error)
}

func SetupRoutes(router fiber.Router, lfs LocalFileService) error {
	fsHandler := NewFSHandler(lfs)
	api := router.Group("/api/v1")
	// File system
	api.Get("/fs/*", fsHandler.Get)
	api.Post("/fs/", fsHandler.Post)
	api.Post("/fs/*", fsHandler.Post)
	api.Put("/fs/", fsHandler.Put)
	api.Put("/fs/*", fsHandler.Put)
	api.Delete("/fs/", fsHandler.Delete)
	api.Delete("/fs/*", fsHandler.Delete)
	return nil
}
