package thumbnail

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	MaxDimension = 512
	MIMEType     = "image/png"
)

type Image struct {
	Data      []byte
	MIMEType  string
	Ext       string
	SizeBytes int64
	Width     int
	Height    int
}

func Generate(data []byte, mimeType string) (Image, error) {
	source, err := decode(data, mimeType)
	if err != nil {
		return fallbackBoundedOriginal(data, mimeType, err)
	}
	width, height := boundedDimensions(source.Bounds().Dx(), source.Bounds().Dy(), MaxDimension)
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), source, source.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return Image{}, err
	}
	return Image{
		Data:      buf.Bytes(),
		MIMEType:  MIMEType,
		Ext:       "png",
		SizeBytes: int64(buf.Len()),
		Width:     width,
		Height:    height,
	}, nil
}

func ObjectKey(tenantID string, projectID string, assetID string, ext string) string {
	ext = strings.Trim(strings.ToLower(strings.TrimSpace(ext)), ".")
	if ext == "" {
		ext = "img"
	}
	return "tenants/" + tenantID + "/projects/" + projectID + "/assets/" + assetID + "/thumbnail." + ext
}

func URL(assetID string) string {
	if strings.TrimSpace(assetID) == "" {
		return ""
	}
	return "/api/v1/assets/" + assetID + "/thumbnail"
}

func decode(data []byte, mimeType string) (image.Image, error) {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return jpeg.Decode(bytes.NewReader(data))
	case "image/png":
		return png.Decode(bytes.NewReader(data))
	case "image/webp":
		return webp.Decode(bytes.NewReader(data))
	default:
		return nil, image.ErrFormat
	}
}

func fallbackBoundedOriginal(data []byte, mimeType string, decodeErr error) (Image, error) {
	if strings.ToLower(strings.TrimSpace(mimeType)) != "image/webp" {
		return Image{}, decodeErr
	}
	cfg, err := webp.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Image{}, decodeErr
	}
	if cfg.Width > MaxDimension || cfg.Height > MaxDimension {
		return Image{}, decodeErr
	}
	return Image{
		Data:      data,
		MIMEType:  "image/webp",
		Ext:       "webp",
		SizeBytes: int64(len(data)),
		Width:     cfg.Width,
		Height:    cfg.Height,
	}, nil
}

func boundedDimensions(width int, height int, maxDimension int) (int, int) {
	if width <= 0 || height <= 0 || maxDimension <= 0 {
		return 1, 1
	}
	if width <= maxDimension && height <= maxDimension {
		return width, height
	}
	if width >= height {
		scaledHeight := height * maxDimension / width
		if scaledHeight < 1 {
			scaledHeight = 1
		}
		return maxDimension, scaledHeight
	}
	scaledWidth := width * maxDimension / height
	if scaledWidth < 1 {
		scaledWidth = 1
	}
	return scaledWidth, maxDimension
}
