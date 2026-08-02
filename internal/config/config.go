package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Environment, HTTPAddress, DatabaseURL, OIDCIssuer, OIDCAudience string
	IdentifierHMACKey []byte
	AuthDisabled bool
}

func Load() (Config, error) {
	c := Config{
		Environment: value("PARTY_ENVIRONMENT", "local"), HTTPAddress: value("PARTY_HTTP_ADDRESS", ":8080"),
		DatabaseURL: strings.TrimSpace(os.Getenv("PARTY_DATABASE_URL")), OIDCIssuer: strings.TrimRight(strings.TrimSpace(os.Getenv("PARTY_OIDC_ISSUER")), "/"),
		OIDCAudience: strings.TrimSpace(os.Getenv("PARTY_OIDC_AUDIENCE")), AuthDisabled: strings.EqualFold(os.Getenv("PARTY_AUTH_DISABLED"), "true"),
	}
	raw, err := secret("PARTY_IDENTIFIER_HMAC_KEY")
	if err != nil { return Config{}, err }
	c.IdentifierHMACKey, err = base64.StdEncoding.DecodeString(raw)
	if err != nil || len(c.IdentifierHMACKey) < 32 { return Config{}, fmt.Errorf("PARTY_IDENTIFIER_HMAC_KEY must be base64 encoding of at least 32 bytes") }
	if c.DatabaseURL == "" { return Config{}, fmt.Errorf("PARTY_DATABASE_URL is required") }
	if c.AuthDisabled && c.Environment != "local" && c.Environment != "sandbox" { return Config{}, fmt.Errorf("authentication can only be disabled locally or in sandbox") }
	if !c.AuthDisabled && (c.OIDCIssuer == "" || c.OIDCAudience == "") { return Config{}, fmt.Errorf("OIDC configuration is required") }
	return c, nil
}
func value(k, fallback string) string { if v := strings.TrimSpace(os.Getenv(k)); v != "" { return v }; return fallback }
func secret(k string) (string, error) {
	direct, path := strings.TrimSpace(os.Getenv(k)), strings.TrimSpace(os.Getenv(k+"_FILE"))
	if direct != "" && path != "" { return "", fmt.Errorf("%s and %s_FILE cannot both be set", k, k) }
	if path == "" { return direct, nil }
	b, err := os.ReadFile(path); if err != nil { return "", err }; return strings.TrimSpace(string(b)), nil
}
