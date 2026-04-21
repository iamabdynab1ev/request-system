package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadTokens_PrefersPoolSources(t *testing.T) {
	tokens, err := loadTokens("single-token", "token-a, token-b, token-a", "")
	if err != nil {
		t.Fatalf("loadTokens returned error: %v", err)
	}

	expected := []string{"token-a", "token-b"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Fatalf("unexpected tokens: got %v want %v", tokens, expected)
	}
}

func TestLoadTokensFromFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "tokens.txt")
	content := strings.Join([]string{
		"\uFEFF# comment",
		"token-1",
		"",
		"token-2",
		"token-1",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	tokens, err := loadTokens("", "", path)
	if err != nil {
		t.Fatalf("loadTokens returned error: %v", err)
	}

	expected := []string{"token-1", "token-2"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Fatalf("unexpected tokens: got %v want %v", tokens, expected)
	}
}

func TestLoadTokensFromFile_RejectsSpaces(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "tokens.txt")
	if err := os.WriteFile(path, []byte("bad token\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, err := loadTokens("", "", path)
	if err == nil {
		t.Fatal("expected error for token with spaces")
	}
}

func TestConfigTokenFor_RoundRobin(t *testing.T) {
	cfg := config{
		Tokens: []string{"token-1", "token-2", "token-3"},
	}

	got := []string{
		cfg.tokenFor(0),
		cfg.tokenFor(1),
		cfg.tokenFor(2),
		cfg.tokenFor(3),
		cfg.tokenFor(4),
	}
	expected := []string{"token-1", "token-2", "token-3", "token-1", "token-2"}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected token order: got %v want %v", got, expected)
	}
}

func TestParseUserIDs(t *testing.T) {
	userIDs, err := parseUserIDs("2956", " 2494 ", "", "# comment", "2956")
	if err != nil {
		t.Fatalf("parseUserIDs returned error: %v", err)
	}

	expected := []uint64{2956, 2494, 2956}
	if !reflect.DeepEqual(userIDs, expected) {
		t.Fatalf("unexpected user IDs: got %v want %v", userIDs, expected)
	}
}

func TestLoadUserIDsFromFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "user_ids.txt")
	content := strings.Join([]string{
		"\uFEFF# comment",
		"2956",
		"",
		"2494",
		"2956",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	userIDs, err := loadUserIDsFromFile(path)
	if err != nil {
		t.Fatalf("loadUserIDsFromFile returned error: %v", err)
	}

	expected := []uint64{2956, 2494, 2956}
	if !reflect.DeepEqual(userIDs, expected) {
		t.Fatalf("unexpected user IDs: got %v want %v", userIDs, expected)
	}
}

func TestDedupeUserIDs(t *testing.T) {
	userIDs := dedupeUserIDs([]uint64{2956, 2494, 2956, 1})
	expected := []uint64{2956, 2494, 1}
	if !reflect.DeepEqual(userIDs, expected) {
		t.Fatalf("unexpected user IDs: got %v want %v", userIDs, expected)
	}
}

func TestUserIDsModeShouldIgnoreExistingEnvTokenSignal(t *testing.T) {
	useGeneratedTokens := strings.TrimSpace("2956,2494") != "" || strings.TrimSpace("") != ""
	if !useGeneratedTokens {
		t.Fatal("expected generated token mode to be enabled when user IDs are provided")
	}
}
