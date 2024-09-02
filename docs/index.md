# Phase Provider

The Phase provider is used to interact with secrets stored in Phase. The provider needs to be configured with the proper credentials before it can be used.

## Example Usage

```hcl
terraform {
  required_providers {
    phase = {
      source  = "phasehq/phase"
      version = "0.1.1" // replace with latest version
    }
  }
}

# Configure the Phase Provider
provider "phase" {
  phase_token = "pss_service:v1:..." # or "pss_user:v1:..." // A Phase Service Token or a Phase User Token (PAT)
}

# Retrieve all secrets for an app
data "phase_secrets" "all" {
  env    = "development"
  app_id = "your-app-id"
  path   = ""
}

# Get all secrets
output "all_secret_keys" {
  value = data.phase_secrets.all.secrets
  sensitive = true
}

# Alternatively, retrieve all secrets under a specific path
data "phase_secrets" "path_secrets" {
  env    = "development"
  app_id = "your-app-id"
  path   = "/backend"
}

# Get a single secret from that path
output "backend_secret_keys" {
  value = data.phase_secrets.path_secrets.secrets["JWT_SECRET"]
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
* `path` - (Optional) The path to fetch secrets from. If not provided, fetches secrets from all paths.
* `key` - (Optional) A specific secret key to fetch. If provided, only this secret will be returned.

#### Attribute Reference

The following attributes are exported:

* `secrets` - A map of secret keys to their corresponding values.

## Fetching Secrets

### Fetching All Secrets for an App

To fetch all secrets for a given app:

```hcl
data "phase_secrets" "all" {
  env    = "development"
  app_id = "your-app-id"
  path   = ""
}

output "all_secret_keys" {
  value = data.phase_secrets.all.secrets
  sensitive = true
}
```

This will fetch all secrets for the specified app and environment, and output their keys.

### Fetching a Single Secret

To fetch a specific secret:

```hcl
data "phase_secrets" "single" {
  env    = "development"
  app_id = "your-app-id"
}

output "database_url" {
  value     = data.phase_secrets.single.secrets["DATABASE_URL"]
  sensitive = true
}
```

This will fetch only the specified secret and output its value.

### Fetching Secrets from a Specific Path

To fetch all secrets under a specific path:

```hcl
data "phase_secrets" "path_secrets" {
  env    = "development"
  app_id = "your-app-id"
  path   = "/backend"
}

output "backend_secret_keys" {
  value = data.phase_secrets.path_secrets.secrets["JWT_SECRET"]
  sensitive = true
}
```

This will fetch all secrets under the specified path and output their keys.

### Using Secrets

You can use the fetched secrets in your Terraform configurations like this:

```hcl
resource "some_resource" "example" {
  database_url   = data.phase_secrets.single.secrets["DATABASE_URL"]
  api_key        = data.phase_secrets.all.secrets["API_KEY"]
  backend_config = data.phase_secrets.path_secrets.secrets["BACKEND_CONFIG"]
}
```

Always mark outputs containing secret values as sensitive to prevent them from being displayed in console output or logs.

## Personal Secret Overrides

Personal Secret Overrides allow individual users to temporarily override the value of a secret for their own use, without affecting the secret's value for other users or systems. Here are some important points to note about Personal Secret Overrides:

1. **User Token Requirement**: To use Personal Secret Overrides, you must authenticate with a Phase User Token (Personal Access Token or PAT). Service tokens do not support Personal Secret Overrides.

2. **Activation**: Personal Secret Overrides must be activated through the Phase Console or the Phase CLI. They cannot be directly triggered or modified through the Terraform provider.

3. **Behavior**: When a Personal Secret Override is active for a user, the Terraform provider will automatically use the overridden value instead of the main secret value when fetching secrets.

4. **Visibility**: Personal Secret Overrides are only visible and applicable to the user who created them. Other users and systems will continue to see and use the main secret value.

5. **Temporary Nature**: Personal Secret Overrides are intended for temporary use, such as during development or testing. They should not be relied upon for production configurations.

Remember that the presence and value of Personal Secret Overrides depend on the authenticated user and the state of overrides in the Phase system, not on the Terraform configuration itself.