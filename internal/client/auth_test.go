package client

import (
	"strings"
	"testing"
)

func TestExtractTokenInfo(t *testing.T) {
	token64 := strings.Repeat("a", 64)

	tests := []struct {
		name          string
		token         string
		wantTokenType string
		wantBearer    string
	}{
		{
			name: "user token",
			token: strings.Join([]string{
				"pss_user",
				"v1",
				token64,
				token64,
				token64,
				token64,
			}, ":"),
			wantTokenType: "User",
			wantBearer:    token64,
		},
		{
			name: "service token v1",
			token: strings.Join([]string{
				"pss_service",
				"v1",
				token64,
				token64,
				token64,
				token64,
			}, ":"),
			wantTokenType: "Service",
			wantBearer:    token64,
		},
		{
			name: "service token v2",
			token: strings.Join([]string{
				"pss_service",
				"v2",
				token64,
				token64,
				token64,
				token64,
			}, ":"),
			wantTokenType: "ServiceAccount",
			wantBearer:    token64,
		},
		{
			name:          "invalid token",
			token:         "invalid-token",
			wantTokenType: "",
			wantBearer:    "invalid-token",
		},
		{
			name:          "empty token",
			token:         "",
			wantTokenType: "",
			wantBearer:    "",
		},
		{
			name: "token with invalid hex",
			token: strings.Join([]string{
				"pss_user",
				"v1",
				strings.Repeat("g", 64),
				token64,
				token64,
				token64,
			}, ":"),
			wantTokenType: "",
			wantBearer: strings.Join([]string{
				"pss_user",
				"v1",
				strings.Repeat("g", 64),
				token64,
				token64,
				token64,
			}, ":"),
		},
		{
			name: "token with insufficient parts",
			token: strings.Join([]string{
				"pss_user",
				"v1",
				token64,
			}, ":"),
			wantTokenType: "",
			wantBearer: strings.Join([]string{
				"pss_user",
				"v1",
				token64,
			}, ":"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotBearer := extractTokenInfo(tt.token)

			if gotType != tt.wantTokenType {
				t.Errorf(
					"extractTokenInfo() type = %q, want %q",
					gotType,
					tt.wantTokenType,
				)
			}

			if gotBearer != tt.wantBearer {
				t.Errorf(
					"extractTokenInfo() bearer = %q, want %q",
					gotBearer,
					tt.wantBearer,
				)
			}
		})
	}
}

func TestPssUserPattern(t *testing.T) {
	token64 := strings.Repeat("a", 64)

	validToken := strings.Join([]string{
		"pss_user",
		"v1",
		token64,
		token64,
		token64,
		token64,
	}, ":")

	if !PssUserPattern.MatchString(validToken) {
		t.Fatal("expected valid user token to match PssUserPattern")
	}

	invalidToken := strings.Join([]string{
		"pss_service",
		"v1",
		token64,
		token64,
		token64,
		token64,
	}, ":")

	if PssUserPattern.MatchString(invalidToken) {
		t.Fatal("expected service token not to match PssUserPattern")
	}
}

func TestPssServicePattern(t *testing.T) {
	token64 := strings.Repeat("a", 64)

	validToken := strings.Join([]string{
		"pss_service",
		"v1",
		token64,
		token64,
		token64,
		token64,
	}, ":")

	if !PssServicePattern.MatchString(validToken) {
		t.Fatal("expected valid service token to match PssServicePattern")
	}

	invalidToken := strings.Join([]string{
		"pss_user",
		"v1",
		token64,
		token64,
		token64,
		token64,
	}, ":")

	if PssServicePattern.MatchString(invalidToken) {
		t.Fatal("expected user token not to match PssServicePattern")
	}
}
