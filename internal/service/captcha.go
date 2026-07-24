package service

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/big"
)

const (
	captchaLength = 5
	captchaWidth  = 190
	captchaHeight = 64
)

const (
	captchaLetters = "ABCDEFGHJKMNPQRSTUVWXYZ"
	captchaDigits  = "23456789"
	// Ambiguous characters 0/O, 1/I/L are intentionally excluded.
	captchaAlphabet = captchaLetters + captchaDigits
)

var captchaGlyphs = map[byte][7]string{
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'B': {"11110", "10001", "10001", "11110", "10001", "10001", "11110"},
	'C': {"01111", "10000", "10000", "10000", "10000", "10000", "01111"},
	'D': {"11110", "10001", "10001", "10001", "10001", "10001", "11110"},
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'F': {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
	'G': {"01111", "10000", "10000", "10111", "10001", "10001", "01111"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'J': {"00111", "00010", "00010", "00010", "00010", "10010", "01100"},
	'K': {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'P': {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
	'Q': {"01110", "10001", "10001", "10001", "10101", "10010", "01101"},
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
	'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'W': {"10001", "10001", "10001", "10101", "10101", "10101", "01010"},
	'X': {"10001", "10001", "01010", "00100", "01010", "10001", "10001"},
	'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
	'Z': {"11111", "00001", "00010", "00100", "01000", "10000", "11111"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
	'6': {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
}

func randomCaptchaCode() (string, error) {
	code := make([]byte, captchaLength)
	for i := 0; i < 2; i++ {
		value, err := secureChoice(captchaLetters)
		if err != nil {
			return "", err
		}
		code[i] = value
	}
	for i := 2; i < 4; i++ {
		value, err := secureChoice(captchaDigits)
		if err != nil {
			return "", err
		}
		code[i] = value
	}
	value, err := secureChoice(captchaAlphabet)
	if err != nil {
		return "", err
	}
	code[4] = value
	for i := len(code) - 1; i > 0; i-- {
		j, err := secureInt(i + 1)
		if err != nil {
			return "", err
		}
		code[i], code[j] = code[j], code[i]
	}
	return string(code), nil
}

func captchaPNGDataURL(code string) (string, error) {
	img := image.NewRGBA(image.Rect(0, 0, captchaWidth, captchaHeight))
	noise := make([]byte, captchaWidth*captchaHeight*3)
	if _, err := rand.Read(noise); err != nil {
		return "", err
	}
	for y := 0; y < captchaHeight; y++ {
		for x := 0; x < captchaWidth; x++ {
			i := (y*captchaWidth + x) * 3
			img.SetRGBA(x, y, color.RGBA{
				R: 226 + noise[i]%24,
				G: 228 + noise[i+1]%22,
				B: 231 + noise[i+2]%20,
				A: 255,
			})
		}
	}

	seed := make([]byte, 1024)
	if _, err := rand.Read(seed); err != nil {
		return "", err
	}
	cursor := 0
	next := func(max int) int {
		if max <= 1 {
			return 0
		}
		value := int(seed[cursor%len(seed)])
		cursor++
		return value % max
	}

	for i := 0; i < 3; i++ {
		drawNoiseCurve(img, next(captchaHeight), 4+next(8), float64(next(628))/100, next(3)+1,
			color.RGBA{R: uint8(80 + next(130)), G: uint8(60 + next(140)), B: uint8(80 + next(130)), A: uint8(65 + next(50))})
	}

	for i := 0; i < len(code); i++ {
		angle := (float64(next(35)) - 17) * math.Pi / 180
		centerX := 22 + i*37 + next(5) - 2
		centerY := captchaHeight/2 + next(9) - 4
		glyphColor := color.RGBA{R: uint8(18 + next(70)), G: uint8(25 + next(75)), B: uint8(35 + next(85)), A: 235}
		drawDistortedGlyph(img, code[i], centerX, centerY, angle, 4.2+float64(next(8))/10, 5.0+float64(next(10))/10, float64(next(628))/100, glyphColor)
	}

	for i := 0; i < 3; i++ {
		drawNoiseCurve(img, next(captchaHeight), 3+next(10), float64(next(628))/100, next(2)+1,
			color.RGBA{R: uint8(50 + next(150)), G: uint8(50 + next(150)), B: uint8(50 + next(150)), A: uint8(80 + next(70))})
	}
	for i := 0; i < 360; i++ {
		x, y := next(captchaWidth), next(captchaHeight)
		blendPixel(img, x, y, color.RGBA{R: uint8(next(210)), G: uint8(next(210)), B: uint8(next(210)), A: uint8(30 + next(100))})
		if i%17 == 0 {
			blendPixel(img, x+1, y, color.RGBA{R: uint8(next(180)), G: uint8(next(180)), B: uint8(next(180)), A: 75})
		}
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes()), nil
}

func drawDistortedGlyph(img *image.RGBA, character byte, centerX, centerY int, angle, scaleX, scaleY, phase float64, ink color.RGBA) {
	pattern, ok := captchaGlyphs[character]
	if !ok {
		return
	}
	cosAngle, sinAngle := math.Cos(angle), math.Sin(angle)
	for y := centerY - 29; y <= centerY+29; y++ {
		for x := centerX - 23; x <= centerX+23; x++ {
			wave := math.Sin(float64(x)*0.19+phase) * 2.2
			dx := float64(x - centerX)
			dy := float64(y-centerY) - wave
			sourceX := (cosAngle*dx+sinAngle*dy)/scaleX + 2.5
			sourceY := (-sinAngle*dx+cosAngle*dy)/scaleY + 3.5
			cellX, cellY := int(math.Floor(sourceX)), int(math.Floor(sourceY))
			if cellX < 0 || cellX >= 5 || cellY < 0 || cellY >= 7 || pattern[cellY][cellX] != '1' {
				continue
			}
			fractionX := sourceX - math.Floor(sourceX)
			fractionY := sourceY - math.Floor(sourceY)
			if fractionX < 0.88 && fractionY < 0.88 {
				blendPixel(img, x, y, ink)
			}
		}
	}
}

func drawNoiseCurve(img *image.RGBA, baseY, amplitude int, phase float64, thickness int, ink color.RGBA) {
	for x := 0; x < captchaWidth; x++ {
		y := baseY + int(math.Sin(float64(x)*0.055+phase)*float64(amplitude)) + (x-captchaWidth/2)/35
		for offset := -thickness; offset <= thickness; offset++ {
			blendPixel(img, x, y+offset, ink)
		}
	}
}

func blendPixel(img *image.RGBA, x, y int, overlay color.RGBA) {
	if !image.Pt(x, y).In(img.Bounds()) {
		return
	}
	base := img.RGBAAt(x, y)
	alpha := uint32(overlay.A)
	inverse := 255 - alpha
	img.SetRGBA(x, y, color.RGBA{
		R: uint8((uint32(overlay.R)*alpha + uint32(base.R)*inverse) / 255),
		G: uint8((uint32(overlay.G)*alpha + uint32(base.G)*inverse) / 255),
		B: uint8((uint32(overlay.B)*alpha + uint32(base.B)*inverse) / 255),
		A: 255,
	})
}

func secureChoice(alphabet string) (byte, error) {
	index, err := secureInt(len(alphabet))
	if err != nil {
		return 0, err
	}
	return alphabet[index], nil
}

func secureInt(max int) (int, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}
