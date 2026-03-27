package importer

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ScanAndImport loops through the given importDir, parses FDC files, and moves them to importedDir
func ScanAndImport(importDir, importedDir string, done <-chan struct{}) error {
	if _, err := os.Stat(importDir); os.IsNotExist(err) {
		slog.Warn("import directory does not exist, skipping scan", "dir", importDir)
		return nil
	}

	if err := os.MkdirAll(importedDir, 0755); err != nil {
		return fmt.Errorf("creating imported dir: %w", err)
	}

	entries, err := os.ReadDir(importDir)
	if err != nil {
		return fmt.Errorf("reading import dir: %w", err)
	}

	var jsonFiles []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			jsonFiles = append(jsonFiles, entry)
		}
	}
	slog.Info("import scan found files", "dir", importDir, "json_count", len(jsonFiles))

	for i, entry := range jsonFiles {
		filePath := filepath.Join(importDir, entry.Name())

		fi, statErr := entry.Info()
		if statErr == nil {
			slog.Info("starting import of file",
				"file", entry.Name(),
				"file_num", i+1,
				"total_files", len(jsonFiles),
				"size_mb", fmt.Sprintf("%.2f", float64(fi.Size())/1e6),
			)
		} else {
			slog.Info("starting import of file", "file", entry.Name(), "file_num", i+1, "total_files", len(jsonFiles))
		}

		if err := parseFDCFile(filePath, done); err != nil {
			slog.Error("failed to parse FDC file", "file", filePath, "error", err)
			continue
		}

		// Move file to importedDir with timestamp
		baseExt := filepath.Ext(entry.Name())
		baseName := strings.TrimSuffix(entry.Name(), baseExt)
		timestamp := time.Now().Format("20060102_150405")
		newName := fmt.Sprintf("%s_%s%s", baseName, timestamp, baseExt)
		
		destPath := filepath.Join(importedDir, newName)
		if err := os.Rename(filePath, destPath); err != nil {
			// If cross-device link error, os.Rename fails. But here we assume same FS.
			slog.Error("failed to move imported file", "from", filePath, "to", destPath, "error", err)
		} else {
			slog.Info("successfully imported and moved file", "file", destPath)
		}
	}

	return nil
}


