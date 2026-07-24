package service

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"regexp"
	"strings"
	"testing"
)

func TestCaptchaCodeAlwaysMixesLettersAndDigits(t *testing.T) {
	allowed := regexp.MustCompile(`^[A-HJ-KM-NP-Z2-9]{5}$`)
	letter := regexp.MustCompile(`[A-Z]`)
	digit := regexp.MustCompile(`[0-9]`)
	seen := make(map[string]struct{})
	for i := 0; i < 50; i++ {
		code, err := randomCaptchaCode()
		if err != nil {
			t.Fatalf("generate captcha code: %v", err)
		}
		if !allowed.MatchString(code) || !letter.MatchString(code) || !digit.MatchString(code) {
			t.Fatalf("captcha code is not an unambiguous letter/digit mix: %q", code)
		}
		seen[code] = struct{}{}
	}
	if len(seen) < 45 {
		t.Fatalf("captcha generator produced too many duplicates: unique=%d", len(seen))
	}
}

func TestCaptchaImageIsRasterizedNoisyPNGWithoutPlaintextAnswer(t *testing.T) {
	code := "A7K3Z"
	dataURL, err := captchaPNGDataURL(code)
	if err != nil {
		t.Fatalf("render captcha: %v", err)
	}
	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected data URL prefix")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, "data:image/png;base64,"))
	if err != nil {
		t.Fatalf("decode captcha: %v", err)
	}
	if bytes.Contains(raw, []byte(code)) {
		t.Fatal("PNG payload contains the plaintext captcha answer")
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if img.Bounds().Dx() != captchaWidth || img.Bounds().Dy() != captchaHeight {
		t.Fatalf("unexpected captcha dimensions: %v", img.Bounds())
	}
	colors := make(map[uint32]struct{})
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			colors[(r&0xff00)<<8|(g&0xff00)|(b>>8)] = struct{}{}
		}
	}
	if len(colors) < 500 {
		t.Fatalf("captcha lacks expected color/noise variation: colors=%d", len(colors))
	}
}
