package main

import "testing"

var testBody = `payload`

func TestGithubSecret(t *testing.T) {

	if checkGithubSig("123", "sha256=5908ccfcc78e69944fd954f569473d5cf65ad2a9dc52056fea7e814b133dbad2", []byte(testBody)) {
		t.Logf("Github token check ok")
	} else {
		t.Fatalf("Github sig check failed")
	}

}
