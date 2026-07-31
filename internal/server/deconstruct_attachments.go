package server

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"well-ambient/internal/db"
)

const (
	maxDeconstructAttachmentSize  = int64(20 << 20)
	maxDeconstructAttachmentCount = 3
	maxDeconstructMultipartSize   = int64(64 << 20)
)

var allowedDeconstructAttachmentExtensions = map[string]string{
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".txt":  "text/plain",
	".md":   "text/markdown",
	".json": "application/json",
	".csv":  "text/csv",
	".xml":  "application/xml",
	".html": "text/html",
}

type deconstructRequestError struct {
	status int
	err    error
}

func (e *deconstructRequestError) Error() string {
	return e.err.Error()
}

func requestError(status int, format string, args ...interface{}) error {
	return &deconstructRequestError{status: status, err: fmt.Errorf(format, args...)}
}

func deconstructRequestErrorStatus(err error) int {
	if requestErr, ok := err.(*deconstructRequestError); ok {
		return requestErr.status
	}
	return http.StatusBadRequest
}

func (s *Server) attachmentRoot() string {
	root := strings.TrimSpace(s.config.Server.AttachmentDir)
	if root == "" {
		root = filepath.Join("data", "demand-attachments")
	}
	return filepath.Clean(root)
}

func (s *Server) readDeconstructRequest(w http.ResponseWriter, r *http.Request) (DeconstructRequest, []db.DemandAttachment, error) {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		var req DeconstructRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return req, nil, requestError(http.StatusBadRequest, "Bad Request: %v", err)
		}
		return req, nil, nil
	}

	if db.DB == nil {
		return DeconstructRequest{}, nil, requestError(http.StatusInternalServerError, "Database not initialized")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxDeconstructMultipartSize)
	reader, err := r.MultipartReader()
	if err != nil {
		return DeconstructRequest{}, nil, requestError(http.StatusBadRequest, "Invalid multipart request: %v", err)
	}

	request := DeconstructRequest{}
	request.DemandID = ""
	request.TaskGroupID = ""
	attachments := make([]db.DemandAttachment, 0, maxDeconstructAttachmentCount)
	uploadedBy := strings.TrimSpace(r.Header.Get("x-authenticated-user-id"))
	if uploadedBy == "" {
		uploadedBy = "unknown"
	}

	cleanup := func() {
		s.removeDemandAttachments(attachments)
	}
	for {
		part, nextErr := reader.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			cleanup()
			return request, nil, requestError(http.StatusBadRequest, "Invalid multipart body: %v", nextErr)
		}

		fieldName := part.FormName()
		if part.FileName() == "" {
			value, readErr := io.ReadAll(io.LimitReader(part, 256<<10))
			part.Close()
			if readErr != nil {
				cleanup()
				return request, nil, requestError(http.StatusBadRequest, "Failed to read field %s: %v", fieldName, readErr)
			}
			switch fieldName {
			case "text":
				request.Text = string(value)
			case "demand_id":
				request.DemandID = strings.TrimSpace(string(value))
			case "task_group_id":
				request.TaskGroupID = strings.TrimSpace(string(value))
			}
			continue
		}

		if fieldName != "files" && fieldName != "file" {
			part.Close()
			continue
		}
		if len(attachments) >= maxDeconstructAttachmentCount {
			part.Close()
			cleanup()
			return request, nil, requestError(http.StatusRequestEntityTooLarge, "最多上传 %d 个附件", maxDeconstructAttachmentCount)
		}
		attachment, persistErr := s.persistDemandAttachment(part, uploadedBy, request.DemandID, request.TaskGroupID)
		part.Close()
		if persistErr != nil {
			cleanup()
			return request, nil, persistErr
		}
		attachments = append(attachments, attachment)
	}

	return request, attachments, nil
}

func (s *Server) persistDemandAttachment(part *multipart.Part, uploadedBy, demandID, taskGroupID string) (db.DemandAttachment, error) {
	originalName := filepath.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
	extension := strings.ToLower(filepath.Ext(originalName))
	defaultMime, allowed := allowedDeconstructAttachmentExtensions[extension]
	if !allowed {
		return db.DemandAttachment{}, requestError(http.StatusUnsupportedMediaType, "不支持附件格式 %s", extension)
	}
	mimeType := strings.TrimSpace(strings.Split(part.Header.Get("Content-Type"), ";")[0])
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = defaultMime
	}

	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return db.DemandAttachment{}, requestError(http.StatusInternalServerError, "Failed to generate attachment ID: %v", err)
	}
	relativeDir := filepath.Join(time.Now().Format("2006"), time.Now().Format("01"))
	relativePath := filepath.Join(relativeDir, hex.EncodeToString(randomBytes)+".gz")
	absolutePath := filepath.Join(s.attachmentRoot(), relativePath)
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o750); err != nil {
		return db.DemandAttachment{}, requestError(http.StatusInternalServerError, "Failed to create attachment directory: %v", err)
	}

	file, err := os.OpenFile(absolutePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return db.DemandAttachment{}, requestError(http.StatusInternalServerError, "Failed to store attachment: %v", err)
	}
	hash := sha256.New()
	gzipWriter := gzip.NewWriter(file)
	written, copyErr := io.Copy(io.MultiWriter(gzipWriter, hash), io.LimitReader(part, maxDeconstructAttachmentSize+1))
	closeGzipErr := gzipWriter.Close()
	closeFileErr := file.Close()
	if copyErr != nil || closeGzipErr != nil || closeFileErr != nil || written > maxDeconstructAttachmentSize {
		_ = os.Remove(absolutePath)
		if written > maxDeconstructAttachmentSize {
			return db.DemandAttachment{}, requestError(http.StatusRequestEntityTooLarge, "附件 %s 不能超过 20 MB", originalName)
		}
		return db.DemandAttachment{}, requestError(http.StatusInternalServerError, "Failed to compress attachment %s", originalName)
	}
	stat, err := os.Stat(absolutePath)
	if err != nil {
		_ = os.Remove(absolutePath)
		return db.DemandAttachment{}, requestError(http.StatusInternalServerError, "Failed to inspect stored attachment: %v", err)
	}

	attachment := db.DemandAttachment{
		DemandID:       demandID,
		TaskGroupID:    taskGroupID,
		UploadedBy:     uploadedBy,
		OriginalName:   originalName,
		MimeType:       mimeType,
		Extension:      extension,
		StoragePath:    relativePath,
		SHA256:         hex.EncodeToString(hash.Sum(nil)),
		OriginalSize:   written,
		CompressedSize: stat.Size(),
		Status:         "stored",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.DB.Create(&attachment).Error; err != nil {
		_ = os.Remove(absolutePath)
		return db.DemandAttachment{}, requestError(http.StatusInternalServerError, "Failed to persist attachment metadata: %v", err)
	}
	return attachment, nil
}

func (s *Server) removeDemandAttachments(attachments []db.DemandAttachment) {
	for _, attachment := range attachments {
		_ = os.Remove(filepath.Join(s.attachmentRoot(), attachment.StoragePath))
		if db.DB != nil && attachment.ID > 0 {
			_ = db.DB.Delete(&db.DemandAttachment{}, attachment.ID).Error
		}
	}
}

func providerFilesURL(apiURL string) (string, error) {
	parsed, err := url.Parse(apiURL)
	if err != nil {
		return "", err
	}
	path := strings.TrimRight(parsed.Path, "/")
	if index := strings.LastIndex(strings.ToLower(path), "/responses"); index >= 0 {
		path = path[:index] + "/files"
	} else if index := strings.LastIndex(strings.ToLower(path), "/chat/completions"); index >= 0 {
		path = path[:index] + "/files"
	} else {
		return "", fmt.Errorf("cannot derive Files API URL from %s", apiURL)
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func providerResponsesURL(apiURL string) (string, error) {
	parsed, err := url.Parse(apiURL)
	if err != nil {
		return "", err
	}
	path := strings.TrimRight(parsed.Path, "/")
	if index := strings.LastIndex(strings.ToLower(path), "/chat/completions"); index >= 0 {
		path = path[:index] + "/responses"
	} else if !strings.HasSuffix(strings.ToLower(path), "/responses") {
		return "", fmt.Errorf("cannot derive Responses API URL from %s", apiURL)
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func (s *Server) uploadAttachmentToProvider(ctx context.Context, client *http.Client, filesURL string, attachment db.DemandAttachment) (string, error) {
	compressed, err := os.Open(filepath.Join(s.attachmentRoot(), attachment.StoragePath))
	if err != nil {
		return "", err
	}
	gzipReader, err := gzip.NewReader(compressed)
	if err != nil {
		compressed.Close()
		return "", err
	}

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	go func() {
		defer compressed.Close()
		defer gzipReader.Close()
		if err := multipartWriter.WriteField("purpose", "user_data"); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, strings.ReplaceAll(attachment.OriginalName, `"`, `'`)))
		header.Set("Content-Type", attachment.MimeType)
		filePart, err := multipartWriter.CreatePart(header)
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		if _, err := io.Copy(filePart, gzipReader); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		if err := multipartWriter.Close(); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		_ = pipeWriter.Close()
	}()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, filesURL, pipeReader)
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(s.config.AI.APIToken))
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("Files API returned status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil || strings.TrimSpace(result.ID) == "" {
		return "", fmt.Errorf("Files API returned an invalid file response")
	}
	return result.ID, nil
}

func (s *Server) uploadAttachmentsToProvider(ctx context.Context, attachments []db.DemandAttachment) ([]string, string, error) {
	if len(attachments) == 0 {
		return nil, "", nil
	}
	filesURL, err := providerFilesURL(s.config.AI.GetRealAPIURL())
	if err != nil {
		return nil, "", requestError(http.StatusBadRequest, "无法定位 LLM Files API: %v", err)
	}
	client := &http.Client{Timeout: 90 * time.Second}
	fileIDs := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		fileID, uploadErr := s.uploadAttachmentToProvider(ctx, client, filesURL, attachment)
		if uploadErr != nil {
			s.deleteProviderFiles(context.Background(), client, filesURL, fileIDs)
			return nil, filesURL, uploadErr
		}
		fileIDs = append(fileIDs, fileID)
	}
	return fileIDs, filesURL, nil
}

func (s *Server) deleteProviderFiles(ctx context.Context, client *http.Client, filesURL string, fileIDs []string) {
	for _, fileID := range fileIDs {
		request, err := http.NewRequestWithContext(ctx, http.MethodDelete, strings.TrimRight(filesURL, "/")+"/"+url.PathEscape(fileID), nil)
		if err != nil {
			continue
		}
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(s.config.AI.APIToken))
		response, err := client.Do(request)
		if err == nil && response != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
			response.Body.Close()
		}
	}
}
