# Terraform Provider for Phase

This Terraform provider allows you to manage secrets in Phase from your Terraform configurations.

## Usage

To use the latest version of the provider in your Terraform configuration, add the following terraform block:

```hcl
terraform {
  required_providers {
    phase = {
      source  = "phasehq/phase"
      version = "0.1.0" // replace with latest version
    }
  }
}

# Configure the Phase Provider
provider "phase" {
  phase_token = "pss_service:v1:..." # or "pss_user:v1:..." // A Phase Service Token or a Phase User Token (PAT)
}

# Retrieve all secrets under a specific path
data "phase_secrets" "all" {
  env    = "development"
  app_id = "your-app-id"
  path   = "/"
}

# Retrieve a specific secret
data "phase_secrets" "single" {
  env    = "development"
  app_id = "your-app-id"
  path   = "/specific/path"
  key    = "SPECIFIC_SECRET_KEY"
}

// Fetch all secrets
# Use secrets
output "all_secret_keys" {
  value = keys(data.phase_secrets.all.secrets)
}

// Single secret retrieval 
output "specific_secret" {
  value     = data.phase_secrets.single.secrets["SPECIFIC_SECRET_KEY"]
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
3. Build the provider using the Go `build` command:
   ```sh
    go build -o terraform-provider-phase
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