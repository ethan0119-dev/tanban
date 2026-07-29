package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
)

func jsonMarshal(value any) ([]byte, error) { return json.Marshal(value) }
func requestFingerprint(value any) string {
	body, _ := json.Marshal(value)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
func strconvFormatInt(value int64) string { return strconv.FormatInt(value, 10) }
func strconvParseInt(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}
