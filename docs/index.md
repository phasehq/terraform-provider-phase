# Phase Provider Documentation

The Phase Terraform provider allows you to manage secrets and interact with the Phase API directly from your Terraform configurations.

## Example Usage

Here's a basic example of configuring the provider and managing a secret:

```hcl
terraform {
  required_providers {
    phase = {
      source  = "phasehq/phase"
      version = ">= 0.2.0" // Use the latest appropriate version
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

# Configure the Phase Provider
# Ensure PHASE_TOKEN environment variable is set, or provide phase_token directly.
provider "phase" {
  # host = "https://your-self-hosted-phase.com" # Optional: for self-hosted instances
  # skip_tls_verification = true              # Optional: if using self-signed certs
}

# Generate a random value for a secret
resource "random_password" "db_password" {
  length           = 32
  special          = true
  override_special = "_%@"
}

# Create or manage a secret in Phase
resource "phase_secret" "database_password" {
  app_id = "your-app-id"       # Replace with your actual App ID
  env    = "production"        # Specify the environment
  key    = "DATABASE_PASSWORD" # The key for the secret
  value  = random_password.db_password.result
  path   = "/database"         # Optional: specify a path (defaults to "/")
  tags   = ["database", "credentials"] # Optional: add tags that have already been created 
  comment = "Managed by Terraform"      # Optional: add a comment
}

# Fetch secrets (example: all secrets at a specific path)
data "phase_secrets" "database_secrets" {
  app_id = phase_secret.database_password.app_id # Use values from managed resources
  env    = phase_secret.database_password.env
  path   = phase_secret.database_password.path
  tags   = ["database"] # Optional: filter by tags
}

# Output a specific secret fetched by the data source
output "db_password_read" {
  value     = data.phase_secrets.database_secrets.secrets["DATABASE_PASSWORD"]
  sensitive = true # Always mark sensitive outputs
}

# Output attributes of the managed secret
output "managed_secret_version" {
  value = phase_secret.database_password.version
}

output "managed_secret_updated_at" {
  value = phase_secret.database_password.updated_at
}
```

## Provider Configuration

The following arguments are supported in the `provider "phase"` block:

*   `phase_token` - (Optional, **Required** if env var not set) The Phase authentication token. This can be a Service Token (`pss_service:...`) or a Personal Access Token (`pss_user:...`).
    *   **Environment Variables:** This value can be provided via `PHASE_TOKEN`, `PHASE_SERVICE_TOKEN`, or `PHASE_PAT_TOKEN` environment variables (checked in that order). Providing it in the configuration block takes precedence.
    *   **Sensitive:** This value is sensitive.
*   `host` - (Optional) The base URL for the Phase API.
    *   **Default:** `https://api.phase.dev` (for Phase Cloud).
    *   **Environment Variable:** Can be set using `PHASE_HOST`.
    *   **Behavior:** If a custom host is provided (not the default), the provider automatically appends `/service/public` to the URL to target the correct API endpoint (e.g., `https://your-host.com/service/public`).
*   `skip_tls_verification` - (Optional) Set to `true` to disable SSL/TLS certificate validation for the `host`. Useful for self-hosted instances with self-signed certificates. **Use with caution.** Defaults to `false`.

## Resources

### `phase_secret`

Manages a single secret within a specific application and environment in Phase.

The provider handles create, read, update, and delete operations. If a `phase_secret` resource is defined for a secret that already exists (based on `app_id`, `env`, `path`, `key`), the provider will manage the existing secret and update it if necessary, rather than failing.

#### Argument Reference

*   `app_id` - (Required, ForceNew) The UUID of the Phase application where the secret resides. Changing this forces a new resource to be created.
*   `env` - (Required, ForceNew) The name of the environment within the application (e.g., `development`, `production`). Changing this forces a new resource to be created.
*   `key` - (Required) The key (name) of the secret (e.g., `DATABASE_URL`, `API_KEY`).
*   `value` - (Required, Sensitive) The value of the secret.
*   `path` - (Optional) The path where the secret is stored within the environment. Defaults to `/` (root). Example: `/database/credentials`.
*   `comment` - (Optional) A description or comment for the secret.
*   `tags` - (Optional) A list of strings to tag the secret with. Tags can be used for filtering when reading secrets.
*   `override` - (Optional) A block to configure a **Personal Secret Override**. This requires authenticating with a User Token (PAT). **Note:** This block *configures* the override value in Phase; its *activation* must still be done via the Phase Console or CLI.
    *   `value` - (Required, Sensitive) The value to use when this override is active for the authenticated user.
    *   `is_active` - (Required, Boolean) Must be set to `true` to configure the override. The provider currently only supports setting active overrides via this block. Setting it to `false` may not explicitly deactivate it via the API, but removes the override configuration from the state.

#### Attribute Reference

In addition to the arguments above, the following attributes are exported:

*   `id` - The unique UUID assigned to the secret by Phase upon creation.
*   `version` - The current version number of the secret. Incremented on each update.
*   `created_at` - The timestamp (UTC RFC3339 format) when the secret was first created.
*   `updated_at` - The timestamp (UTC RFC3339 format) when the secret was last updated.

## Data Sources

### `phase_secrets`

Fetches multiple secrets from Phase based on specified filters.

#### Argument Reference

*   `app_id` - (Required) The UUID of the Phase application.
*   `env` - (Required) The name of the environment.
*   `path` - (Optional) The path to filter secrets by. If omitted or empty, secrets from the root path (`/`) are fetched by default (behavior might depend on API specifics, explicitly use `/` for root). **Note:** The API endpoint used might primarily fetch based on `key` if provided, potentially ignoring `path`. For guaranteed path-based fetching without a specific key, ensure `key` is omitted. For fetching *all* secrets regardless of path, this might require multiple data source calls or future provider enhancements if the API requires path specification.
*   `key` - (Optional) The key of a *specific* secret to fetch. If provided, only the secret matching this key (within the specified `app_id` and `env`, considering `path` behavior mentioned above) will be returned.
*   `tags` - (Optional) A list of strings (tags) to filter secrets by. Secrets matching *any* of the provided tags will be included (OR logic).

#### Attribute Reference

*   `secrets` - (Computed, Sensitive) A map where keys are the secret keys (e.g., `DATABASE_URL`) and values are their corresponding secret values. If a Personal Secret Override is active for the authenticated user, the override value will be returned here.
*   `id` - A unique identifier constructed by the provider for this data source instance based on the input arguments (`app_id`, `env`, `path`, `key`, `tags`).

## Importing

Existing secrets managed outside of Terraform can be imported into your Terraform state.

Use the `terraform import` command with the following ID format:

```bash
terraform import phase_secret.<resource_name_in_tf> "{app_id}:{env}:{path}:{key}"
```

**Components:**

*   `phase_secret.<resource_name_in_tf>`: The type and name of the resource block in your Terraform configuration (`.tf` file) that corresponds to the secret you want to import.
*   `{app_id}`: The UUID of the application.
*   `{env}`: The name of the environment.
*   `{path}`: The **exact** path where the secret exists in Phase, including leading and trailing slashes if applicable (e.g., `/`, `/database/`, `/folder/path/`).
*   `{key}`: The key of the secret.

**Example:**

```bash
# Assuming a resource block like: resource "phase_secret" "imported_secret" { ... }
terraform import phase_secret.imported_secret "907549ca-1430-4aa0-9998-290525741005:production:/database/:DB_HOST"
```

After importing, run `terraform plan` to see any differences between your configuration and the imported state, and adjust your `.tf` file accordingly.

## Advanced Topics

### Personal Secret Overrides

Personal Secret Overrides allow individual users (authenticating with a User Token/PAT) to temporarily use a different value for a secret without affecting the globally stored value.

*   **Authentication:** Requires a `pss_user:...` token. Service tokens (`pss_service:...`) cannot read or manage overrides.
*   **Provider Interaction:**
    *   **Reading (`data "phase_secrets"`):** If an override is *active* in Phase for the authenticated user, the data source will return the override value.
    *   **Managing (`resource "phase_secret"`):** You can define the `override` block in a `phase_secret` resource to *configure* the override value in Phase. However, **activating** the override must still be done separately through the Phase Console or CLI. The provider essentially sets the stage for the override.
*   **Visibility:** Overrides are personal. Only the user who created and activated the override (and is authenticated with their PAT) will see the overridden value via the provider.

### Working with Tags

Tags provide a way to categorize and filter secrets.

Please note: To be able to assign tags, they must be already created in the Phase Console before hand.

*   **Assigning Tags:** Use the `tags` argument in the `phase_secret` resource:
    ```hcl
    resource "phase_secret" "api_key" {
      # ... other args ...
      key  = "THIRD_PARTY_API_KEY"
      path = "/integrations/"
      tags = ["api", "billing", "external"]
    }
    ```
*   **Filtering by Tags:** Use the `tags` argument in the `phase_secrets` data source. It returns secrets matching *any* of the specified tags (OR logic).
    ```hcl
    # Fetch secrets tagged with 'database' OR 'redis'
    data "phase_secrets" "cache_and_db" {
      app_id = "your-app-id"
      env    = "staging"
      tags   = ["database", "redis"]
    }

    # Fetch 'api' tagged secrets specifically from the '/backend' path
    data "phase_secrets" "backend_api" {
      app_id = "your-app-id"
      env    = "staging"
      path   = "/backend/"
      tags   = ["api"]
    }

    output "api_keys" {
      value     = data.phase_secrets.backend_api.secrets
      sensitive = true
    }
    ```

### Secret Metadata

The `phase_secret` resource exports metadata about the managed secret:

```hcl
resource "phase_secret" "config" {
  app_id = "your-app-id"
  env    = "production"
  key    = "FEATURE_FLAG_X"
  value  = "true"
}

output "config_version" {
  description = "Current version of the feature flag secret."
  value       = phase_secret.config.version
}

output "config_last_updated" {
  description = "Timestamp when the feature flag was last modified."
  value       = phase_secret.config.updated_at
}
```