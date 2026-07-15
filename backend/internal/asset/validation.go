package asset

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/config"
	"golang.org/x/image/webp"
)

const (
	maxFilenameRunes  = 255
	multipartOverhead = 10 * 1024 * 1024
)

type uploadValidator struct {
	config  config.UploadConfig
	allowed map[string]struct{}
}

type validatedUpload struct {
	Data      []byte
	MimeType  string
	Ext       string
	SizeBytes int64
	Width     int
	Height    int
	SHA256    string
	Filename  string
}

func newUploadValidator(uploadConfig config.UploadConfig) uploadValidator {
	uploadConfig = config.NormalizeUploadConfig(uploadConfig)
	allowed := make(map[string]struct{}, len(uploadConfig.AllowedMIMETypes))
	for _, mimeType := range uploadConfig.AllowedMIMETypes {
		allowed[strings.ToLower(strings.TrimSpace(mimeType))] = struct{}{}
	}
	return uploadValidator{config: uploadConfig, allowed: allowed}
}

func (v uploadValidator) maxRequestBytes() int64 {
	return v.config.MaxFileSizeBytes + multipartOverhead
}

func (v uploadValidator) validate(fileHeader *multipart.FileHeader) (validatedUpload, error) {
	if fileHeader == nil {
		return validatedUpload{}, ErrValidation
	}

	declaredMIME, err := normalizeMIME(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		return validatedUpload{}, err
	}
	if _, ok := v.allowed[declaredMIME]; !ok {
		return validatedUpload{}, ErrValidation
	}

	file, err := fileHeader.Open()
	if err != nil {
		return validatedUpload{}, ErrValidation
	}
	defer file.Close()

	data, err := readLimited(file, v.config.MaxFileSizeBytes)
	if err != nil {
		return validatedUpload{}, err
	}
	detectedMIME, ext, err := detectImageType(data)
	if err != nil {
		return validatedUpload{}, err
	}
	if declaredMIME != detectedMIME {
		return validatedUpload{}, ErrValidation
	}
	if _, ok := v.allowed[detectedMIME]; !ok {
		return validatedUpload{}, ErrValidation
	}

	width, height, err := decodeDimensions(detectedMIME, data)
	if err != nil {
		return validatedUpload{}, ErrValidation
	}
	if width <= 0 || height <= 0 || width > v.config.MaxWidth || height > v.config.MaxHeight {
		return validatedUpload{}, ErrValidation
	}
	if int64(width)*int64(height) > v.config.MaxPixels {
		return validatedUpload{}, ErrValidation
	}

	sum := sha256.Sum256(data)
	return validatedUpload{
		Data:      data,
		MimeType:  detectedMIME,
		Ext:       ext,
		SizeBytes: int64(len(data)),
		Width:     width,
		Height:    height,
		SHA256:    hex.EncodeToString(sum[:]),
		Filename:  sanitizeFilename(fileHeader.Filename, ext),
	}, nil
}

func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, ErrValidation
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, ErrValidation
	}
	if int64(len(data)) > maxBytes || len(data) == 0 {
		return nil, ErrValidation
	}
	return data, nil
}

func normalizeMIME(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrValidation
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return "", ErrValidation
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if strings.Contains(mediaType, "svg") {
		return "", ErrValidation
	}
	return mediaType, nil
}

func detectImageType(data []byte) (string, string, error) {
	switch {
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg", "jpg", nil
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return "image/png", "png", nil
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp", "webp", nil
	default:
		return "", "", ErrValidation
	}
}

func decodeDimensions(mimeType string, data []byte) (int, int, error) {
	switch mimeType {
	case "image/jpeg":
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
		return cfg.Width, cfg.Height, err
	case "image/png":
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		return cfg.Width, cfg.Height, err
	case "image/webp":
		cfg, err := webp.DecodeConfig(bytes.NewReader(data))
		return cfg.Width, cfg.Height, err
	default:
		return 0, 0, ErrValidation
	}
}

func cleanOptional(value string, maxRunes int) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > maxRunes {
		return "", ErrValidation
	}
	for _, r := range value {
		if r < 32 || r == 127 {
			return "", ErrValidation
		}
	}
	return value, nil
}

func sanitizeFilename(raw string, fallbackExt string) string {
	raw = strings.ReplaceAll(raw, "\\", "/")
	raw = path.Base(raw)
	raw = strings.TrimSpace(raw)
	raw = strings.Map(func(r rune) rune {
		switch {
		case r < 32 || r == 127:
			return -1
		case r == '/' || r == '\\':
			return '_'
		default:
			return r
		}
	}, raw)
	if raw == "" || raw == "." || raw == "/" {
		raw = "upload." + fallbackExt
	}
	return truncateRunes(raw, maxFilenameRunes)
}

func filenameForMIME(raw string, mimeType string) string {
	extension := extensionForMIME(mimeType)
	filename := sanitizeFilename(raw, extension)
	stem := strings.TrimSpace(strings.TrimSuffix(filename, path.Ext(filename)))
	if stem == "" {
		stem = "image"
	}
	suffix := "." + extension
	return truncateRunes(stem, maxFilenameRunes-utf8.RuneCountInString(suffix)) + suffix
}

func truncateRunes(value string, maxRunes int) string {
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	out := make([]rune, 0, maxRunes)
	for _, r := range value {
		if len(out) == maxRunes {
			break
		}
		out = append(out, r)
	}
	return string(out)
}
