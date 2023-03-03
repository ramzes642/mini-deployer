package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

var testBody = `payload`

func TestGithubSecret(t *testing.T) {

	if checkGithubSig("123", "sha256=5908ccfcc78e69944fd954f569473d5cf65ad2a9dc52056fea7e814b133dbad2", []byte(testBody)) {
		t.Logf("Github token check ok")
	} else {
		t.Fatalf("Github sig check failed")
	}

}

func MarshalSigned(v any, secret []byte) (string, error) {
	hasher := hmac.New(sha256.New, secret)

	header, _ := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})

	payload, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(header)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)

	hasher.Write(bytes.Join([][]byte{[]byte(headerB64), []byte(payloadB64)}, []byte(".")))
	signature := hasher.Sum(nil)

	sigB64 := base64.RawURLEncoding.EncodeToString(signature)

	jwt := strings.Join([]string{
		headerB64, payloadB64, sigB64,
	}, ".")

	return jwt, nil
}

func TestJwt(t *testing.T) {
	token, _ := MarshalSigned(map[string]string{"grp": "root"}, []byte("shared_secret"))
	Cfg.JwtHmac = "shared_secret"
	Cfg.JwtClaimAny = []string{"root"}
	Cfg.JwtClaim = "grp"

	assert.Equal(t, true, checkJwt("Bearer "+token))
	Cfg.JwtClaimAny = []string{"admin"}
	assert.Equal(t, false, checkJwt("Bearer "+token))
	Cfg.JwtClaimAny = []string{""}
	assert.Equal(t, false, checkJwt("Bearer "+token))
	Cfg.JwtClaimAny = []string{}
	assert.Equal(t, false, checkJwt("Bearer "+token))

	assert.Equal(t, false, checkJwt("Bearer random string"))

	token, _ = MarshalSigned(map[string]string{"noclaim": "root"}, []byte("shared_secret"))
	assert.Equal(t, false, checkJwt("Bearer "+token))

}
