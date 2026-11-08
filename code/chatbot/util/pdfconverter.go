package util

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

func ExtractPDFText(pdfBytes []byte) (string, error) {
	if len(pdfBytes) == 0 {
		return "", fmt.Errorf("PDF content is empty")
	}

	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		return "", fmt.Errorf("invalid PDF content: missing PDF header")
	}

	reader, err := pdf.NewReader(bytes.NewReader(pdfBytes), int64(len(pdfBytes)))
	if err != nil {
		return "", fmt.Errorf("open PDF: %w", err)
	}

	textReader, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract PDF text: %w", err)
	}

	var textBuilder strings.Builder
	if _, err := io.Copy(&textBuilder, textReader); err != nil {
		return "", fmt.Errorf("read extracted PDF text: %w", err)
	}

	text := strings.TrimSpace(textBuilder.String())
	if text == "" {
		return "", fmt.Errorf("PDF contains no extractable text; scanned PDFs are not supported")
	}

	return text, nil
}
