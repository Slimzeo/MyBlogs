package config

import (
	"strings"
	"testing"
)

func TestValidateAccessKey(t *testing.T) {
	base := &Config{SessionSecret: strings.Repeat("s", 32)}
	if err := base.Validate(); err != nil {
		t.Fatalf("empty access key should disable the feature: %v", err)
	}

	tooShort := *base
	tooShort.AccessKey = "short"
	if err := tooShort.Validate(); err == nil {
		t.Fatal("short access key unexpectedly passed validation")
	}

	valid := *base
	valid.AccessKey = "reader-key-12345"
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid access key was rejected: %v", err)
	}
}
