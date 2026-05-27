package thumbnail

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestGenerateReturnsBoundedJPEGForSupportedFormats(t *testing.T) {
	for _, tc := range []struct {
		name       string
		data       []byte
		mimeType   string
		wantWidth  int
		wantHeight int
	}{
		{name: "jpeg", data: testJPEG(t, 640, 320), mimeType: "image/jpeg", wantWidth: 512, wantHeight: 256},
		{name: "png", data: testPNG(t, 300, 900), mimeType: "image/png", wantWidth: 170, wantHeight: 512},
		{name: "webp", data: testWebP(t), mimeType: "image/webp", wantWidth: 75, wantHeight: 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			thumb, err := Generate(tc.data, tc.mimeType)
			if err != nil {
				t.Fatalf("Generate returned error: %v", err)
			}
			if thumb.MIMEType != MIMEType || thumb.Ext != "jpg" {
				t.Fatalf("thumbnail format = %s/%s, want image/jpeg jpg", thumb.MIMEType, thumb.Ext)
			}
			if thumb.Width != tc.wantWidth || thumb.Height != tc.wantHeight {
				t.Fatalf("thumbnail dimensions = %dx%d, want %dx%d", thumb.Width, thumb.Height, tc.wantWidth, tc.wantHeight)
			}
			if thumb.Width > MaxDimension || thumb.Height > MaxDimension {
				t.Fatalf("thumbnail dimensions exceed bounds: %dx%d", thumb.Width, thumb.Height)
			}
			if thumb.SizeBytes <= 0 || thumb.SizeBytes > MaxSizeBytes || int64(len(thumb.Data)) != thumb.SizeBytes {
				t.Fatalf("thumbnail size = %d len=%d max=%d", thumb.SizeBytes, len(thumb.Data), MaxSizeBytes)
			}
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(thumb.Data))
			if err != nil {
				t.Fatalf("thumbnail is not decodable JPEG: %v", err)
			}
			if cfg.Width != thumb.Width || cfg.Height != thumb.Height {
				t.Fatalf("decoded dimensions = %dx%d, want %dx%d", cfg.Width, cfg.Height, thumb.Width, thumb.Height)
			}
		})
	}
}

func TestGenerateDoesNotUpscaleSmallImages(t *testing.T) {
	thumb, err := Generate(testPNG(t, 32, 24), "image/png")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if thumb.Width != 32 || thumb.Height != 24 {
		t.Fatalf("small thumbnail dimensions = %dx%d, want 32x24", thumb.Width, thumb.Height)
	}
}

func TestGenerateRejectsUnsupportedOrInvalidImages(t *testing.T) {
	for _, tc := range []struct {
		name     string
		data     []byte
		mimeType string
	}{
		{name: "unsupported mime", data: testPNG(t, 2, 2), mimeType: "image/gif"},
		{name: "invalid webp", data: []byte("not-webp"), mimeType: "image/webp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Generate(tc.data, tc.mimeType); err == nil {
				t.Fatal("Generate returned nil error")
			}
		})
	}
}

func TestObjectKeyUsesJPGThumbnailPath(t *testing.T) {
	got := ObjectKey("tenant-a", "project-a", "asset-a", "png")
	want := "tenants/tenant-a/projects/project-a/assets/asset-a/thumbnail.jpg"
	if got != want {
		t.Fatalf("ObjectKey = %q, want %q", got, want)
	}
}

func testPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := testImage(width, height)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func testJPEG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := testImage(width, height)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func testImage(width int, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 160, A: 255})
		}
	}
	return img
}

func testWebP(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA==")
	if err != nil {
		t.Fatalf("decode webp fixture: %v", err)
	}
	return data
}
