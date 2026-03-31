package importer

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type importFile struct {
	entry  os.DirEntry
	format string // "fdc" or "off"
}

// ScanAndImport loops through the given importDir, parses FDC (.json) and OFF (.jsonl) files,
// and moves processed files to importedDir.
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

	var files []importFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") {
			files = append(files, importFile{entry, "fdc"})
		} else if strings.HasSuffix(name, ".jsonl") {
			files = append(files, importFile{entry, "off"})
		}
	}

	jsonCount, jsonlCount := 0, 0
	for _, f := range files {
		if f.format == "fdc" {
			jsonCount++
		} else {
			jsonlCount++
		}
	}
	slog.Info("import scan found files", "dir", importDir, "json_count", jsonCount, "jsonl_count", jsonlCount)

	for i, f := range files {
		entry := f.entry
		filePath := filepath.Join(importDir, entry.Name())

		fi, statErr := entry.Info()
		if statErr == nil {
			slog.Info("starting import of file",
				"file", entry.Name(),
				"format", f.format,
				"file_num", i+1,
				"total_files", len(files),
				"size_mb", fmt.Sprintf("%.2f", float64(fi.Size())/1e6),
			)
		} else {
			slog.Info("starting import of file", "file", entry.Name(), "format", f.format, "file_num", i+1, "total_files", len(files))
		}

		var parseErr error
		if f.format == "off" {
			parseErr = parseOFFFile(filePath, done)
		} else {
			parseErr = parseFDCFile(filePath, done)
		}
		if parseErr != nil {
			slog.Error("failed to parse import file", "file", filePath, "format", f.format, "error", parseErr)
			continue
		}

		// Move file to importedDir with timestamp
		baseExt := filepath.Ext(entry.Name())
		baseName := strings.TrimSuffix(entry.Name(), baseExt)
		timestamp := time.Now().Format("20060102_150405")
		newName := fmt.Sprintf("%s_%s%s", baseName, timestamp, baseExt)

		destPath := filepath.Join(importedDir, newName)
		if err := os.Rename(filePath, destPath); err != nil {
			slog.Error("failed to move imported file", "from", filePath, "to", destPath, "error", err)
		} else {
			slog.Info("successfully imported and moved file", "file", destPath)
		}
	}

	return nil
}
