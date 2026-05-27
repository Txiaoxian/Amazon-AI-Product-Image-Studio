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
	MaxSizeBytes = 1024 * 1024
	MIMEType     = "image/jpeg"
	Quality      = 85
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
		return Image{}, err
	}
	width, height := boundedDimensions(source.Bounds().Dx(), source.Bounds().Dy(), MaxDimension)
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), source, source.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: Quality}); err != nil {
		return Image{}, err
	}
	if buf.Len() > MaxSizeBytes {
		return Image{}, image.ErrFormat
	}
	return Image{
		Data:      buf.Bytes(),
		MIMEType:  MIMEType,
		Ext:       "jpg",
		SizeBytes: int64(buf.Len()),
		Width:     width,
		Height:    height,
	}, nil
}

func ObjectKey(tenantID string, projectID string, assetID string, ext string) string {
	return "tenants/" + tenantID + "/projects/" + projectID + "/assets/" + assetID + "/thumbnail.jpg"
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
