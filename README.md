# Terraform Provider for Phase

This Terraform provider allows you to manage Phase applications and secrets from your Terraform configurations.

## Usage

To use the provider in your Terraform configuration, add the following:

```hcl
terraform {
  required_providers {
    phase = {
      source  = "phasehq/phase"
      version = "0.2.0"
    }
  }
}

provider "phase" {
  phase_token = "pss_service:v1:..." # or "pss_user:v1:..."
}

resource "phase_application" "example" {
  name        = "my-application"
  description = "Application managed by Terraform"
}

resource "phase_secret" "database_password" {
  app_id = phase_application.example.id
  env    = "production"
  key    = "DATABASE_PASSWORD"
  value  = "my-secret-password"
  path   = "/database/"
  tags   = ["database", "credentials"]
  comment = "Managed by Terraform"
}

data "phase_secrets" "database" {
  app_id = phase_application.example.id
  env    = "production"
  path   = "/database/"
}

output "database_password" {
  value     = data.phase_secrets.database.secrets["DATABASE_PASSWORD"]
  sensitive = true
}
```

See the [Phase Provider documentation](docs/index.md) for all available resources, data sources, and configuration options.

## Requirements

* [Terraform](https://www.terraform.io/downloads.html) >= 0.13.x
* [Go](https://go.dev/doc/install) >= 1.18

## Building the Provider

1. Clone the repository.
2. Enter the repository directory.
3. Build the provider:

```sh
go build -o terraform-provider-phase
```

## Developing the Provider

If you wish to work on the provider, make sure [Go](https://go.dev/doc/install) is installed on your machine.

To install the provider binary locally:

```sh
go install
```

To run the test suite:

```sh
go test -count=1 ./...
```

The `-count=1` flag forces Go to run the tests without using the test cache.

To generate or update the provider documentation:

```sh
go generate
```

### Local Provider Installation

1. Create a local plugin directory:

```sh
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/phasehq/phase/0.2.0/$(go env GOOS)_$(go env GOARCH)
```

2. Build the provider:

```sh
go build -o terraform-provider-phase
```

3. Move the binary to the plugin directory:

```sh
mv terraform-provider-phase ~/.terraform.d/plugins/registry.terraform.io/phasehq/phase/0.2.0/$(go env GOOS)_$(go env GOARCH)
```

4. Configure Terraform to use the local provider version:

```hcl
terraform {
  required_providers {
    phase = {
      source  = "registry.terraform.io/phasehq/phase"
      version = "0.2.0"
    }
  }
}
```

5. Initialize Terraform:

```sh
terraform init
```

6. Review the execution plan:

```sh
terraform plan
```

7. Apply the configuration:

```sh
terraform apply
```

## License

This provider is distributed under the [MIT License](LICENSE).
