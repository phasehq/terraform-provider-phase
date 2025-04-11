#!/bin/bash

# Set the version
VERSION="0.2.0"

# Build the provider with the expected naming convention
go build -o terraform-provider-phase_v${VERSION}

# Create a symlink with the exact naming convention Terraform expects
# Format: terraform-provider-{NAME}_v{VERSION}_{OS}_{ARCH}
OS=$(go env GOOS)
ARCH=$(go env GOARCH)
ln -sf terraform-provider-phase_v${VERSION} terraform-provider-phase_v${VERSION}_${OS}_${ARCH}

# Remove the lock file if it exists
rm -f .terraform.lock.hcl
