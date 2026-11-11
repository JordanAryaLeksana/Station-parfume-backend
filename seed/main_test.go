package main

import (
	"strings"
	"testing"
)

func TestCheckArgs(t *testing.T) {
	if err := checkArgs(nil); err != nil {
		t.Fatalf("tanpa argumen harus lolos: %v", err)
	}
	err := checkArgs([]string{"admin"})
	if err == nil || !strings.Contains(err.Error(), "maksud Anda -admin?") {
		t.Fatalf("'admin' tanpa minus harus ditolak dengan saran: %v", err)
	}
	if err := checkArgs([]string{"katalog.csv"}); err == nil || strings.Contains(err.Error(), "maksud Anda") {
		t.Fatalf("argumen asing ditolak tanpa saran flag: %v", err)
	}
}
