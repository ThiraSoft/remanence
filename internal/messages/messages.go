package messages

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"remanence/internal/config"
	"remanence/internal/logger"
)

type Message struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Date      time.Time `json:"date"`
	LifeLimit int       `json:"lifeLimit"` // in minutes
	IsOneShot bool      `json:"isOneShot"`
}

const (
	idChars            = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	randomContentChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 .,!?;:'-()@#&"
	// A real encrypted envelope is base64( IV[12] || ciphertext+tag[16+] ),
	// so its smallest possible size is 29 bytes (12 + 16 + 1 of plaintext).
	minEnvelopeBytes = 29
)

var (
	messages = map[string]Message{}
	mu       = sync.RWMutex{}
)

// generateID creates a cryptographically random base62 ID.
func generateID() string {
	b := make([]byte, config.MessageIDLength)
	max := big.NewInt(int64(len(idChars)))
	for i := range b {
		n, _ := rand.Int(rand.Reader, max)
		b[i] = idChars[n.Int64()]
	}
	return string(b)
}

// validID checks that an ID contains only base62 characters.
// The length is intentionally NOT constrained: accepting IDs of any
// length maximizes confusion, since lookups for unknown IDs return
// random content rather than revealing the expected ID format.
func validID(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		if !strings.ContainsRune(idChars, c) {
			return false
		}
	}
	return true
}

func GetMessageByID(id string) (Message, bool) {
	mu.RLock()
	message, exists := messages[id]
	mu.RUnlock()
	return message, exists
}

func GetMessageContentByID(id string) (string, error) {
	if !validID(id) {
		return "", fmt.Errorf("invalid id format")
	}

	message, exists := GetMessageByID(id)

	if !exists {
		return RandomContent(), nil
	}

	if time.Since(message.Date) > time.Duration(message.LifeLimit)*time.Minute {
		deleteMessage(message.ID)
		return RandomContent(), nil
	}

	c := message.Content

	if message.IsOneShot {
		deleteMessage(message.ID)
	}

	return c, nil
}

func NewMessageWithRandomID(content string, isOneShot bool, lifeLimit int) (string, error) {
	logger.Log.Debug("Creating new message", slog.Bool("isOneShot", isOneShot), slog.Int("lifeLimit", lifeLimit))

	id := generateID()
	for messageExists(id) {
		id = generateID()
	}

	err := NewMessage(id, content, isOneShot, lifeLimit)
	if err != nil {
		return "", err
	}
	return id, nil
}

func NewMessage(id, content string, isOneShot bool, lifeLimit int) error {
	finalContent := strings.TrimSpace(content)

	if utf8.RuneCountInString(finalContent) > config.MaxMessageLength {
		return fmt.Errorf("content exceeds %d characters", config.MaxMessageLength)
	}
	if len(finalContent) == 0 {
		return fmt.Errorf("content cannot be empty")
	}

	if lifeLimit < 0 {
		return fmt.Errorf("life limit must be positive")
	}
	if lifeLimit > config.MaxLifeLimit {
		return fmt.Errorf("life limit exceeds maximum of %d minutes", config.MaxLifeLimit)
	}
	if lifeLimit == 0 {
		lifeLimit = config.MaxLifeLimit
	}

	m := Message{
		Content:   finalContent,
		ID:        id,
		Date:      time.Now(),
		LifeLimit: lifeLimit,
		IsOneShot: isOneShot,
	}

	mu.Lock()
	messages[id] = m
	mu.Unlock()
	return nil
}

// RandomContent generates cryptographically random content that is
// indistinguishable from a real message to an outside observer — the
// "confusion" decoy returned for unknown or expired IDs.
//
// When encryption is enabled, real messages are base64-encoded encrypted
// envelopes. The decoy is therefore built by base64-encoding random bytes
// of a plausible envelope size: this matches the real content's alphabet,
// its padding (=) and its length distribution, so an attacker probing IDs
// cannot tell a decoy from real ciphertext.
func RandomContent() string {
	if config.EncryptionEnabled {
		// Largest envelope whose base64 form still fits MaxMessageLength.
		maxEnvelope := config.MaxMessageLength / 4 * 3
		if maxEnvelope <= minEnvelopeBytes {
			maxEnvelope = minEnvelopeBytes + 1
		}
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(maxEnvelope-minEnvelopeBytes)))
		b := make([]byte, int(n.Int64())+minEnvelopeBytes)
		rand.Read(b)
		return base64.StdEncoding.EncodeToString(b)
	}

	charset := []byte(randomContentChars)
	max := big.NewInt(int64(len(charset)))

	// Random length between 10 and MAX_MESSAGE_LENGTH
	lengthRange := big.NewInt(int64(config.MaxMessageLength - 10))
	n, _ := rand.Int(rand.Reader, lengthRange)
	length := int(n.Int64()) + 10

	b := make([]byte, length)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, max)
		b[i] = charset[idx.Int64()]
	}
	return string(b)
}

func messageExists(id string) bool {
	mu.RLock()
	_, exists := messages[id]
	mu.RUnlock()
	return exists
}

func deleteMessage(id string) {
	mu.Lock()
	delete(messages, id)
	mu.Unlock()
}

// StartCleanup launches a background goroutine that periodically removes expired messages.
func StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			mu.Lock()
			for id, msg := range messages {
				if now.Sub(msg.Date) > time.Duration(msg.LifeLimit)*time.Minute {
					delete(messages, id)
					logger.Log.Info("Expired message cleaned up", slog.String("id", id))
				}
			}
			mu.Unlock()
		}
	}()
}
