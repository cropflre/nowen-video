package recoverystress

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

func ScenarioIdentity(contract ScenarioContract, spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) (version, hash, canonical string, err error) {
	if err := contract.ValidateFor(spec, manifest); err != nil {
		return "", "", "", err
	}
	content, err := json.Marshal(contract)
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256(content)
	return contract.SchemaVersion, hex.EncodeToString(digest[:]), string(content), nil
}

func AggregateIdentity(contract AggregateContract, spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) (version, hash, canonical string, err error) {
	if err := contract.ValidateFor(spec, manifest); err != nil {
		return "", "", "", err
	}
	content, err := json.Marshal(contract)
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256(content)
	return contract.SchemaVersion, hex.EncodeToString(digest[:]), string(content), nil
}

func CanonicalHash(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

func TokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func findAsset(manifest transcodecorpus.Manifest, caseID string) (transcodecorpus.Asset, bool) {
	for _, asset := range manifest.Assets {
		if asset.CaseID == caseID {
			return asset, true
		}
	}
	return transcodecorpus.Asset{}, false
}

func isSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
