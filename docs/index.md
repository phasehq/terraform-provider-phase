# Phase Provider

The Phase provider is used to interact with the resources supported by Phase. The provider needs to be configured with the proper credentials before it can be used.

## Example Usage

```hcl
terraform {
  required_providers {
    phase = {
      source  = "phasehq/phase"
      version = "~> 0.1.0"
    }
  }
}

# Configure the Phase Provider
provider "phase" {
  service_token = "pss_service:v1:..."
}

# Retrieve secrets
data "phase_secrets" "example" {
  env             = "Development"
  application_id  = "your-app-id"
  path            = "/"
}

# Use a secret
output "example_secret" {
  value     = data.phase_secrets.example.secrets["YOUR_SECRET_KEY"]
  sensitive = true
}
```

## Argument Reference

The following arguments are supported in the provider configuration:

* `service_token` - (Required) The Phase service token. This can also be specified with the `PHASE_SERVICE_TOKEN` environment variable.
* `host` - (Optional) The Phase API host. Defaults to `https://console.phase.dev`. This can also be specified with the `PHASE_HOST` environment variable.

## Data Sources

### phase_secrets

Retrieve secrets from Phase.

#### Argument Reference

The following arguments are supported:

* `env` - (Required) The environment name.
* `application_id` - (Optional) The application ID. This takes precedence over `application_name` if both are provided.
* `application_name` - (Optional) The application name. Only used if `application_id` is not provided.
* `path` - (Optional) The path to fetch secrets from. Defaults to "/".

#### Attribute Reference

The following attributes are exported:

* `secrets` - A map of secret keys to their corresponding values.