package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"jdShopServer/config"
	"jdShopServer/internal/repository"
)

const (
	SMSPurposeRegister      = "register"
	SMSPurposePasswordReset = "password_reset"
	CaptchaTTL              = 5 * time.Minute
	SMSCodeTTL              = 5 * time.Minute
	SMSInterval             = time.Minute
	SMSDailyLimit           = 6
	SMSIPHourlyLimit        = 20
	SMSMaxAttempts          = 5
)

var (
	ErrInvalidPhone        = errors.New("invalid phone")
	ErrCaptchaInvalid      = errors.New("captcha invalid")
	ErrSMSNotConfigured    = errors.New("sms service not configured")
	ErrSMSInsecureEndpoint = errors.New("sms endpoint must use https")
	ErrSMSTooFrequent      = errors.New("sms too frequent")
	ErrSMSDailyLimit       = errors.New("sms daily limit reached")
	ErrSMSIPLimit          = errors.New("sms ip limit reached")
	ErrSMSProvider         = errors.New("sms provider error")
	ErrSMSCodeInvalid      = errors.New("sms code invalid")
	ErrSMSCodeExpired      = errors.New("sms code expired")
	ErrSMSCodeNotFound     = errors.New("sms code not found")
)

var phonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

type captchaRecord struct {
	Code      string
	ExpiresAt time.Time
}

type SMSService struct {
	repo     *repository.SMSVerificationRepo
	cfg      config.SMSConfig
	client   *http.Client
	mu       sync.Mutex
	sendMu   sync.Mutex
	captchas map[string]captchaRecord
	pepper   []byte
}

func NewSMSService(repo *repository.SMSVerificationRepo, cfg config.SMSConfig, codePepper string) *SMSService {
	return &SMSService{
		repo:     repo,
		cfg:      cfg,
		client:   &http.Client{Timeout: 10 * time.Second},
		captchas: make(map[string]captchaRecord),
		pepper:   []byte(codePepper),
	}
}

func NormalizePhone(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	if !phonePattern.MatchString(phone) {
		return "", ErrInvalidPhone
	}
	return phone, nil
}

func (s *SMSService) Captcha() (id, imageDataURL, testCode string, expiresAt time.Time, err error) {
	code, err := randomCaptchaCode()
	if err != nil {
		return "", "", "", time.Time{}, err
	}
	id, err = randomToken(18)
	if err != nil {
		return "", "", "", time.Time{}, err
	}
	imageDataURL, err = captchaPNGDataURL(code)
	if err != nil {
		return "", "", "", time.Time{}, err
	}
	expiresAt = time.Now().UTC().Add(CaptchaTTL)
	s.mu.Lock()
	s.cleanupCaptchasLocked(time.Now().UTC())
	s.captchas[id] = captchaRecord{Code: code, ExpiresAt: expiresAt}
	s.mu.Unlock()
	if s.cfg.ExposeCaptchaCodeForTesting {
		testCode = code
	}
	return id, imageDataURL, testCode, expiresAt, nil
}

func (s *SMSService) VerifyCaptcha(id, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.captchas[id]
	delete(s.captchas, id)
	if !ok || time.Now().UTC().After(record.ExpiresAt) || !strings.EqualFold(strings.TrimSpace(code), record.Code) {
		return ErrCaptchaInvalid
	}
	return nil
}

func (s *SMSService) Send(phone, purpose, captchaID, captchaCode, requestIP string) error {
	phone, err := NormalizePhone(phone)
	if err != nil {
		return err
	}
	if purpose != SMSPurposeRegister && purpose != SMSPurposePasswordReset {
		return errors.New("invalid sms purpose")
	}
	if err := s.VerifyCaptcha(captchaID, captchaCode); err != nil {
		return err
	}
	// Serialize the rate-limit check, provider request and persistence step. Without
	// this lock, multiple valid captchas submitted concurrently could all pass the
	// same last-sent/count checks before any request is recorded.
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	if !s.cfg.Enabled || strings.TrimSpace(s.cfg.Endpoint) == "" || strings.TrimSpace(s.cfg.Username) == "" || strings.TrimSpace(s.cfg.APIKey) == "" || strings.Count(s.cfg.ContentTemplate, "%s") != 1 {
		return ErrSMSNotConfigured
	}
	endpointURL, err := url.Parse(s.cfg.Endpoint)
	if err != nil || (endpointURL.Scheme != "https" && endpointURL.Hostname() != "127.0.0.1" && endpointURL.Hostname() != "localhost") {
		return ErrSMSInsecureEndpoint
	}
	now := time.Now().UTC()
	lastSent, err := s.repo.LastSentAt(phone)
	if err != nil {
		return err
	}
	if lastSent != nil && now.Sub(*lastSent) < SMSInterval {
		return ErrSMSTooFrequent
	}
	chinaTime := now.In(time.FixedZone("Asia/Shanghai", 8*60*60))
	chinaDayStart := time.Date(chinaTime.Year(), chinaTime.Month(), chinaTime.Day(), 0, 0, 0, 0, chinaTime.Location()).UTC()
	count, err := s.repo.CountSentSince(phone, chinaDayStart)
	if err != nil {
		return err
	}
	if count >= SMSDailyLimit {
		return ErrSMSDailyLimit
	}
	ipCount, err := s.repo.CountSentByIPSince(requestIP, now.Add(-time.Hour))
	if err != nil {
		return err
	}
	if ipCount >= SMSIPHourlyLimit {
		return ErrSMSIPLimit
	}
	lastCodeHash, err := s.repo.LastCodeHash(phone)
	if err != nil {
		return err
	}
	var code string
	for attempts := 0; attempts < 5; attempts++ {
		code, err = randomDigits(6)
		if err != nil {
			return err
		}
		if s.hashCode(code) != lastCodeHash {
			break
		}
	}
	if s.hashCode(code) == lastCodeHash {
		return errors.New("could not generate a distinct sms code")
	}
	content := fmt.Sprintf(s.cfg.ContentTemplate, code)
	params := url.Values{}
	params.Set("u", s.cfg.Username)
	params.Set("p", s.cfg.APIKey)
	params.Set("m", phone)
	params.Set("c", content)
	if s.cfg.GoodsID != "" {
		params.Set("g", s.cfg.GoodsID)
	}
	endpointURL.RawQuery = params.Encode()
	resp, err := s.client.Get(endpointURL.String())
	if err != nil {
		// The provider uses a query-string credential. Do not wrap the transport
		// error because it may contain the full URL (and therefore the API key).
		return ErrSMSProvider
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 || strings.TrimSpace(string(body)) != "0" {
		return ErrSMSProvider
	}
	hash := s.hashCode(code)
	return s.repo.Create(phone, purpose, hash, now, now.Add(SMSCodeTTL), requestIP)
}

func (s *SMSService) VerifyCode(phone, purpose, code string) error {
	phone, err := NormalizePhone(phone)
	if err != nil {
		return err
	}
	verification, err := s.repo.Latest(phone, purpose)
	if err != nil {
		return err
	}
	if verification == nil {
		return ErrSMSCodeNotFound
	}
	if verification.Consumed || time.Now().UTC().After(verification.ExpiresAt) {
		return ErrSMSCodeExpired
	}
	if verification.Attempts >= SMSMaxAttempts || !hmac.Equal([]byte(s.hashCode(code)), []byte(verification.CodeHash)) {
		consume := verification.Attempts+1 >= SMSMaxAttempts
		if err := s.repo.IncrementAttempts(verification.ID, consume); err != nil {
			return err
		}
		return ErrSMSCodeInvalid
	}
	return s.repo.Consume(verification.ID)
}

func (s *SMSService) hashCode(code string) string {
	mac := hmac.New(sha256.New, s.pepper)
	_, _ = mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomDigits(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, value := range bytes {
		b.WriteByte('0' + value%10)
	}
	return b.String(), nil
}

func randomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *SMSService) cleanupCaptchasLocked(now time.Time) {
	for id, record := range s.captchas {
		if now.After(record.ExpiresAt) {
			delete(s.captchas, id)
		}
	}
}
