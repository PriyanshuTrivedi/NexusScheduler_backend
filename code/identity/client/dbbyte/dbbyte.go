package dbbyte

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/util"
)

//go:generate mockgen -source=dbbyte.go -destination=../../../../gen/mocks/identity/client/dbbyte/dbbyte_mock.go -package=mocks

type Client interface {
	Upload(ctx context.Context, name, mimeType string, content []byte) (entity.DBByteFile, error)
	Download(ctx context.Context, name string) ([]byte, error)
	Delete(ctx context.Context, name string) error
}

type client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func New(apiKey, baseURL, bucket string) (Client, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, fmt.Errorf("dbbyte: invalid base URL")
	}
	return &client{
		baseURL: strings.TrimRight(baseURL, "/") + "/storage/" + bucket,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *client) Upload(ctx context.Context, name, mimeType string, content []byte) (entity.DBByteFile, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     "file",
		"filename": name,
	}))
	header.Set("Content-Type", mimeType)

	part, err := writer.CreatePart(header)
	if err != nil {
		return entity.DBByteFile{}, fmt.Errorf("dbbyte: create multipart part: %w", err)
	}

	if _, err := part.Write(content); err != nil {
		return entity.DBByteFile{}, fmt.Errorf("dbbyte: write file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return entity.DBByteFile{}, fmt.Errorf("dbbyte: close multipart writer: %w", err)
	}

	resp, err := c.request(
		ctx,
		http.MethodPost,
		c.baseURL+"/upload",
		&body,
		writer.FormDataContentType(),
	)
	if err != nil {
		return entity.DBByteFile{}, err
	}
	defer resp.Body.Close()

	var file entity.DBByteFile
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return entity.DBByteFile{}, fmt.Errorf("dbbyte: decode upload response: %w", err)
	}

	if !file.Success {
		return entity.DBByteFile{}, fmt.Errorf("dbbyte: upload response did not confirm success")
	}

	if file.Name == "" {
		file.Name = name
	}

	if file.MIMEType == "" {
		file.MIMEType = mimeType
	}

	return file, nil
}

func (c *client) Download(ctx context.Context, name string) ([]byte, error) {
	resp, err := c.request(
		ctx,
		http.MethodGet,
		c.baseURL+"/download/"+url.PathEscape(name),
		nil,
		"",
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("dbbyte: read downloaded file: %w", err)
	}

	return content, nil
}

func (c *client) Delete(ctx context.Context, name string) error {
	resp, err := c.request(
		ctx,
		http.MethodDelete,
		c.baseURL+"/delete/"+url.PathEscape(name),
		nil,
		"",
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *client) request(ctx context.Context, method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("dbbyte: create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dbbyte: execute request: %w", err)
	}

	if err := util.CheckHTTPResponse(resp, "dbbyte "+method); err != nil {
		return nil, err
	}

	return resp, nil
}
