# Phase Provider

The Phase provider is used to interact with secrets stored in Phase. The provider needs to be configured with the proper credentials before it can be used.

## Example Usage

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

## Argument Reference

The following arguments are supported in the provider configuration:

* `phase_token` - (Required) The Phase authentication token. This can be either a service token or a personal access token. It can be specified with the `PHASE_SERVICE_TOKEN` or `PHASE_PAT_TOKEN` environment variable.
* `host` - (Optional) The Phase API host. Defaults to `https://api.phase.dev` for Phase Cloud. This can be specified with the `PHASE_HOST` environment variable. If a custom host is provided, "/service/public" will be appended to the URL.

## Data Sources

### phase_secrets

Retrieve secrets from Phase.

#### Argument Reference

The following arguments are supported:

* `env` - (Required) The environment name.
* `app_id` - (Required) The application ID.
* `path` - (Optional) The path to fetch secrets from. Defaults to "/".
* `key` - (Optional) A specific secret key to fetch. If provided, only this secret will be returned.

#### Attribute Reference

The following attributes are exported:

* `secrets` - A map of secret keys to their corresponding values.

## Fetching Secrets

### Fetching All Secrets

To fetch all secrets under a specific path:

```hcl
data "phase_secrets" "all" {
  env    = "development"
  app_id = "your-app-id"
  path   = "/"
}

output "all_secret_keys" {
  value = keys(data.phase_secrets.all.secrets)
}
```

This will fetch all secrets under the root path ("/") and output their keys.

### Fetching a Single Secret

To fetch a specific secret:

```hcl
data "phase_secrets" "single" {
  env    = "development"
  app_id = "your-app-id"
  path   = "/path/to/secret"
  key    = "SPECIFIC_SECRET_KEY"
}

output "specific_secret" {
  value     = data.phase_secrets.single.secrets["SPECIFIC_SECRET_KEY"]
  sensitive = true
}
```

This will fetch only the specified secret and output its value.

### Using Secrets

You can use the fetched secrets in your Terraform configurations like this:

```hcl
resource "some_resource" "example" {
  sensitive_field = data.phase_secrets.all.secrets["SENSITIVE_SECRET_KEY"]
}
```

Always mark outputs containing secret values as sensitive to prevent them from being displayed in console output or logs.