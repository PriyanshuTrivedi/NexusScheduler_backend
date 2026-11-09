package dbbyte

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/stretchr/testify/require"
)

func TestNew_RejectsInvalidBaseURL(t *testing.T) {
	_, err := New("api-key", "://bad-url", "bucket")
	require.Error(t, err)

	_, err = New("api-key", "ftp://example.com", "bucket")
	require.Error(t, err)
}

func TestUpload_SendsMultipartAndAuthorization(t *testing.T) {
	content := []byte("file-content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.True(t, strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;"))

		require.NoError(t, r.ParseMultipartForm(1<<20))
		file, header, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()
		require.Equal(t, "resume.pdf", header.Filename)
		got, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, content, got)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entity.DBByteFile{
			Success:  true,
			Key:      "bucket/resume.pdf",
			FileID:   "file-1",
			Name:     "stored-resume.pdf",
			Size:     int64(len(content)),
			MIMEType: "application/pdf",
		})
	}))
	defer server.Close()

	client, err := New("test-key", server.URL, "bucket")
	require.NoError(t, err)

	got, err := client.Upload(context.Background(), "resume.pdf", "application/pdf", content)
	require.NoError(t, err)
	require.Equal(t, "stored-resume.pdf", got.Name)
	require.Equal(t, "file-1", got.FileID)
}

func TestUpload_FillsMissingNameAndMIMEType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true}`)
	}))
	defer server.Close()

	client, err := New("key", server.URL, "bucket")
	require.NoError(t, err)

	got, err := client.Upload(context.Background(), "photo.png", "image/png", []byte("image"))
	require.NoError(t, err)
	require.Equal(t, "photo.png", got.Name)
	require.Equal(t, "image/png", got.MIMEType)
}

func TestUpload_RejectsUnsuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":false}`)
	}))
	defer server.Close()

	client, err := New("key", server.URL, "bucket")
	require.NoError(t, err)

	_, err = client.Upload(context.Background(), "file.pdf", "application/pdf", []byte("data"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "did not confirm success")
}

func TestUpload_RejectsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "not-json")
	}))
	defer server.Close()

	client, err := New("key", server.URL, "bucket")
	require.NoError(t, err)

	_, err = client.Upload(context.Background(), "file.pdf", "application/pdf", []byte("data"))
	require.Error(t, err)
}

func TestDownload_UsesEscapedNameAndReturnsBytes(t *testing.T) {
	content := []byte("downloaded-content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "/storage/bucket/download/file%20name.pdf", r.URL.EscapedPath())
		_, _ = w.Write(content)
	}))
	defer server.Close()

	client, err := New("test-key", server.URL, "bucket")
	require.NoError(t, err)

	got, err := client.Download(context.Background(), "file name.pdf")
	require.NoError(t, err)
	require.Equal(t, content, got)
}

func TestDelete_SendsDeleteRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "/storage/bucket/delete/file.pdf", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := New("test-key", server.URL, "bucket")
	require.NoError(t, err)
	require.NoError(t, client.Delete(context.Background(), "file.pdf"))
}

func TestRequests_ReturnErrorForNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := New("key", server.URL, "bucket")
	require.NoError(t, err)

	_, err = client.Download(context.Background(), "file.pdf")
	require.Error(t, err)
}

func TestUpload_ContentDispositionUsesValidMIMEEncoding(t *testing.T) {
	// Ensure the multipart filename parameter is encoded by the standard library.
	value := mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": "résumé.pdf"})
	_, params, err := mime.ParseMediaType(value)
	require.NoError(t, err)
	require.Equal(t, "résumé.pdf", params["filename"])

	_, err = multipart.NewWriter(io.Discard).CreateFormFile("file", "x.pdf")
	require.NoError(t, err)
}
