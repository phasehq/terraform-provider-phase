package client

import (
	"regexp"
	"strings"
)

var (
	// Compiled regex patterns
	PssUserPattern    = regexp.MustCompile(`^pss_user:v(\d+):([a-fA-F0-9]{64}):([a-fA-F0-9]{64}):([a-fA-F0-9]{64}):([a-fA-F0-9]{64})$`)
	PssServicePattern = regexp.MustCompile(`^pss_service:v(\d+):([a-fA-F0-9]{64}):([a-fA-F0-9]{64}):([a-fA-F0-9]{64}):([a-fA-F0-9]{64})$`)
)

func extractTokenInfo(phaseToken string) (string, string) {
	// First, check if it's a service token
	if PssServicePattern.MatchString(phaseToken) {
		parts := strings.Split(phaseToken, ":")

		version := parts[1]
		bearerToken := parts[2]

		// For service tokens with v2
		if version == "v2" {
			return "ServiceAccount", bearerToken
		}

		return "Service", bearerToken
	}

	// Then check if it's a user token
	if PssUserPattern.MatchString(phaseToken) {
		parts := strings.Split(phaseToken, ":")
		return "User", parts[2]
	}

	return "", phaseToken
}
