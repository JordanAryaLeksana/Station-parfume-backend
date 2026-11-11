package config

import (
	"strings"
	"testing"
)

func TestValidateJWTSecrets(t *testing.T) {
	strong := strings.Repeat("a", 32)
	other := strings.Repeat("b", 40)
	if err := ValidateJWTSecrets(strong, other); err != nil {
		t.Fatalf("secret kuat harus lolos: %v", err)
	}
	for name, c := range map[string][2]string{
		"kosong":         {"", other},
		"refresh kosong": {strong, ""},
		"pendek":         {"rahasia", other},
		"sama":           {other, other},
	} {
		if err := ValidateJWTSecrets(c[0], c[1]); err == nil {
			t.Errorf("%s: harus error", name)
		}
	}
}
