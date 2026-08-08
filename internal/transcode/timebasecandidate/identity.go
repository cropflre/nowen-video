package timebasecandidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func (c Contract) CanonicalJSON() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal encoder time-base candidate evidence: %w", err)
	}
	return string(content), nil
}

func Identity(c Contract) (version, hash, canonical string, err error) {
	canonical, err = c.CanonicalJSON()
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return c.SchemaVersion, hex.EncodeToString(digest[:]), canonical, nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
