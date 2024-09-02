#!/bin/bash

# Set the version
VERSION="0.1.1"

# Build the provider
go build -o terraform-provider-phase

# Create the plugin directory if it doesn't exist
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/phasehq/phase/${VERSION}/$(go env GOOS)_$(go env GOARCH)/

# Move the binary to the plugin directory
mv terraform-provider-phase ~/.terraform.d/plugins/registry.terraform.io/phasehq/phase/${VERSION}/$(go env GOOS)_$(go env GOARCH)/

# Remove the lock file if it exists
rm -f .terraform.lock.hcl

# Initialize Terraform
terraform init