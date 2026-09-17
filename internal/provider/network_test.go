package provider

import (
	"fmt"
	"net/http"
	"os"
	"testing"
)

func TestCreateApplication(t *testing.T) {
	// must have a valid PAT token from phase
	// EXPORT PHASE_REST_API_TOKEN="pat_token"
	restToken := os.Getenv("PHASE_REST_API_TOKEN")
	if restToken == "" {
		t.Fatal("PHASE_REST_API_TOKEN is not set")
	}

	tokenType, bearerToken := extractTokenInfo(restToken)

	client := &PhaseClient{
		HostURL:    DefaultHostURL + "/service/public",
		HTTPClient: &http.Client{},
		Token:      bearerToken,
		TokenType:  tokenType,
	}

	input := Application{
		Name: "teste",
	}

	application, err := client.CreateApplication(input, fmt.Sprintf("Bearer %s", client.TokenType))
	if err != nil {
		t.Fatal(err)
	}

	if application.Name != input.Name {
		t.Fatal("name should be equal")
	}

	if application.ID == "" {
		t.Fatal("id should exists")
	}
}
