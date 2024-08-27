package main

import (
	"fmt"
	"runtime"

	"github.com/phasehq/golang-sdk/phase/misc"
)

// Version of the provider
const Version = "0.1.0"

// GetUserAgent returns the user agent string for the Terraform provider
func GetUserAgent() string {
	return fmt.Sprintf("terraform-provider-phase/%s phase-golang-sdk/%s (%s/%s)",
		Version,
		misc.Version,
		runtime.GOOS,
		runtime.GOARCH)
}