package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func EnsureCorrectMime(filePath string) error {
	if filepath.Ext(filePath) != ".pdf" {
		return nil
	}

	tempFile := filePath + ".fixed"
	cmd := exec.Command("gs", "-o", tempFile, "-sDEVICE=pdfwrite", "-dPDFSETTINGS=/prepress", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fix PDF: %w", err)
	}

	if err := os.Rename(tempFile, filePath); err != nil {
		return fmt.Errorf("failed to replace original file: %w", err)
	}

	return nil
}
