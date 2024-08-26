# Terraform Provider for Phase

This Terraform provider allows you to manage secrets in Phase from your Terraform configurations.

## Usage

To use the latest version of the provider in your Terraform configuration, add the following terraform block:

```hcl
// Import
terraform {
  required_providers {
    phase = {
      source  = "phasehq/phase"
      version = "~> 0.1.0"
    }
  }
}


// Initialize
provider "phase" {
  service_token = "pss_service:v1:..."
}

// Fetch secrets
data "phase_secrets" "starlink-command" {
  env = "Production"
  application_id = "b6ad8824-7133-4839-8013-f87c2182fc61"
  path = "/"
}

output "secrets" {
  value = data.phase_secrets.starlink-command.secrets
  sensitive = true
}
```

See the [Phase Provider documentation](docs/index.md) for all the available options and data sources.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 0.13.x
- [Go](https://golang.org/doc/install) >= 1.18

## Building The Provider

1. Clone the repository
2. Enter the repository directory
3. Build the provider using the Go `install` command:
   ```sh
   go install
   ```

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

1. Create a local plugin directory for Terraform:
   ```sh
   mkdir -p ~/.terraform.d/plugins/registry.terraform.io/phasehq/phase/0.1.0/$(go env GOOS)_$(go env GOARCH)
   ```

2. Move the compiled binary to the plugin directory:
   ```sh
   mv terraform-provider-phase ~/.terraform.d/plugins/registry.terraform.io/phasehq/phase/0.1.0/$(go env GOOS)_$(go env GOARCH)
   ```

3. In your Terraform configuration, specify the local version:
   ```hcl
   terraform {
     required_providers {
       phase = {
         source  = "registry.terraform.io/phasehq/phase"
         version = "0.1.0"
       }
     }
   }
   ```

4. Initialize terraform
    ```
    terraform init
    ```

5. Run terraform plan
    ```
    terraform plan
    ```

6. Initialize terraform
    ```
    terraform apply
    ```

## License

This provider is distributed under the [MIT License](LICENSE).