package client

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/phasehq/terraform-provider/internal/config"
)

type PhaseClient struct {
	HostURL    string
	HTTPClient *http.Client
	Token      string
	TokenType  string
}

func NewPhaseClient(
	host string,
	token string,
	skipTLSVerification bool,
) *PhaseClient {
	if host != config.DefaultHostURL {
		host = fmt.Sprintf("%s/service/public", host)
	}

	tokenType, bearerToken := extractTokenInfo(token)

	httpClient := &http.Client{}

	if skipTLSVerification {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec - explicitly configured by the user
			},
		}
	}

	return &PhaseClient{
		HostURL:    host,
		Token:      bearerToken,
		TokenType:  tokenType,
		HTTPClient: httpClient,
	}
}
