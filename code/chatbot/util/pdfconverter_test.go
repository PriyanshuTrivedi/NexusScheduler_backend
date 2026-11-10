package util

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func makeTestPDF(text string) []byte {
	var buffer bytes.Buffer
	offsets := []int{0}

	writeObject := func(id int, body string) {
		offsets = append(offsets, buffer.Len())
		fmt.Fprintf(&buffer, "%d 0 obj\n%s\nendobj\n", id, body)
	}

	buffer.WriteString("%PDF-1.4\n")
	writeObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>")
	writeObject(4, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	stream := fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", text)
	writeObject(5, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))

	xrefOffset := buffer.Len()
	fmt.Fprintf(&buffer, "xref\n0 %d\n", len(offsets))
	buffer.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&buffer, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buffer, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(offsets), xrefOffset)

	return buffer.Bytes()
}

func TestExtractPDFText(t *testing.T) {
	t.Run("extracts text from a valid PDF", func(t *testing.T) {
		got, err := ExtractPDFText(makeTestPDF("Nexus Scheduler document"))
		require.NoError(t, err)
		require.Contains(t, got, "Nexus Scheduler document")
	})

	t.Run("rejects empty content", func(t *testing.T) {
		got, err := ExtractPDFText(nil)
		require.Empty(t, got)
		require.EqualError(t, err, "PDF content is empty")
	})

	t.Run("rejects content without PDF header", func(t *testing.T) {
		got, err := ExtractPDFText([]byte("not a PDF"))
		require.Empty(t, got)
		require.EqualError(t, err, "invalid PDF content: missing PDF header")
	})

	t.Run("rejects malformed PDF", func(t *testing.T) {
		got, err := ExtractPDFText([]byte("%PDF-1.4\nthis is not a valid PDF"))
		require.Empty(t, got)
		require.ErrorContains(t, err, "open PDF")
	})
}
