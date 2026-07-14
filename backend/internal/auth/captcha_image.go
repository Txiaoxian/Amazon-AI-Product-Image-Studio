package auth

import (
	"bytes"
	cryptorand "crypto/rand"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math/big"
)

type CaptchaGenerator func() (code string, imagePNG []byte, err error)

const captchaDigits = "23456789"

var digitGlyphs = map[byte][7]byte{
	'2': {0b11111, 0b00001, 0b00001, 0b11111, 0b10000, 0b10000, 0b11111},
	'3': {0b11111, 0b00001, 0b00001, 0b11111, 0b00001, 0b00001, 0b11111},
	'4': {0b10001, 0b10001, 0b10001, 0b11111, 0b00001, 0b00001, 0b00001},
	'5': {0b11111, 0b10000, 0b10000, 0b11111, 0b00001, 0b00001, 0b11111},
	'6': {0b11111, 0b10000, 0b10000, 0b11111, 0b10001, 0b10001, 0b11111},
	'7': {0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b01000, 0b01000},
	'8': {0b11111, 0b10001, 0b10001, 0b11111, 0b10001, 0b10001, 0b11111},
	'9': {0b11111, 0b10001, 0b10001, 0b11111, 0b00001, 0b00001, 0b11111},
}

func generateCaptcha() (string, []byte, error) {
	codeBytes := make([]byte, 4)
	for index := range codeBytes {
		value, err := cryptoRandomInt(len(captchaDigits))
		if err != nil {
			return "", nil, err
		}
		codeBytes[index] = captchaDigits[value]
	}
	imagePNG, err := renderCaptchaPNG(codeBytes)
	if err != nil {
		return "", nil, err
	}
	return string(codeBytes), imagePNG, nil
}

func renderCaptchaPNG(code []byte) ([]byte, error) {
	if len(code) != 4 {
		return nil, errors.New("captcha code must contain four digits")
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 160, 56))
	background := color.RGBA{R: 245, G: 247, B: 250, A: 255}
	for y := 0; y < canvas.Bounds().Dy(); y++ {
		for x := 0; x < canvas.Bounds().Dx(); x++ {
			canvas.SetRGBA(x, y, background)
		}
	}

	for index := 0; index < 9; index++ {
		x0, err := cryptoRandomInt(160)
		if err != nil {
			return nil, err
		}
		y0, err := cryptoRandomInt(56)
		if err != nil {
			return nil, err
		}
		x1, err := cryptoRandomInt(160)
		if err != nil {
			return nil, err
		}
		y1, err := cryptoRandomInt(56)
		if err != nil {
			return nil, err
		}
		drawCaptchaLine(canvas, x0, y0, x1, y1, color.RGBA{R: 186, G: 198, B: 214, A: 255})
	}

	for index, digit := range code {
		jitter, err := cryptoRandomInt(5)
		if err != nil {
			return nil, err
		}
		drawCaptchaDigit(canvas, digit, 12+index*37, 8+jitter-2, 5, color.RGBA{R: 36, G: 50, B: 72, A: 255})
	}

	for index := 0; index < 90; index++ {
		x, err := cryptoRandomInt(160)
		if err != nil {
			return nil, err
		}
		y, err := cryptoRandomInt(56)
		if err != nil {
			return nil, err
		}
		canvas.SetRGBA(x, y, color.RGBA{R: 142, G: 159, B: 184, A: 255})
	}

	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func drawCaptchaDigit(canvas *image.RGBA, digit byte, originX int, originY int, scale int, fill color.RGBA) {
	glyph, ok := digitGlyphs[digit]
	if !ok {
		return
	}
	for row, bits := range glyph {
		for column := 0; column < 5; column++ {
			if bits&(1<<uint(4-column)) == 0 {
				continue
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					canvas.SetRGBA(originX+column*scale+dx, originY+row*scale+dy, fill)
				}
			}
		}
	}
}

func drawCaptchaLine(canvas *image.RGBA, x0 int, y0 int, x1 int, y1 int, fill color.RGBA) {
	dx := absoluteInt(x1 - x0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	dy := -absoluteInt(y1 - y0)
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		canvas.SetRGBA(x0, y0, fill)
		if x0 == x1 && y0 == y1 {
			break
		}
		twice := 2 * err
		if twice >= dy {
			err += dy
			x0 += sx
		}
		if twice <= dx {
			err += dx
			y0 += sy
		}
	}
}

func cryptoRandomInt(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("random upper bound must be positive")
	}
	value, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}

func absoluteInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
