# Phase Provider Documentation

The Phase Terraform provider lets you manage secrets, apps, environments, service accounts and invites in Phase directly from your Terraform configurations.

Secrets are encrypted and decrypted client-side by the provider using the [Phase Go SDK](https://github.com/phasehq/golang-sdk): the keys embedded in your token unwrap the environment key locally, so secret values never reach the server in plaintext. Management resources (apps, environments, service accounts, invites) use the Phase REST API.

## Example Usage

```hcl
terraform {
  required_providers {
    phase = {
      source  = "phasehq/phase"
      version = ">= 0.3.0" // Use the latest appropriate version
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
  app_id  = "your-app-id"              # Replace with your actual App ID
  env     = "production"               # Specify the environment
  key     = "DATABASE_PASSWORD"        # The key for the secret
  value   = random_password.db_password.result
  path    = "/database"                # Optional: specify a path (defaults to "/")
  tags    = ["database", "credentials"] # Optional: tags are created if they don't exist yet
  comment = "Managed by Terraform"     # Optional: add a comment
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

*   `phase_token` - (Optional, **Required** if env var not set) The Phase authentication token. This can be a Service Account Token (`pss_service:v2:...`) or a Personal Access Token (`pss_user:v1:...`). The full token is required: the provider uses the key material it contains to encrypt and decrypt secrets locally.
    *   **Environment Variables:** This value can be provided via `PHASE_TOKEN`, `PHASE_SERVICE_TOKEN`, or `PHASE_PAT_TOKEN` environment variables (checked in that order). Providing it in the configuration block takes precedence.
    *   **Sensitive:** This value is sensitive.
*   `host` - (Optional) The base URL of your Phase Console, e.g. `https://phase.example.com`.
    *   **Default:** `https://api.phase.dev` (Phase Cloud). Both `https://api.phase.dev` and `https://console.phase.dev` select Phase Cloud.
    *   **Environment Variable:** Can be set using `PHASE_HOST`.
*   `skip_tls_verification` - (Optional) Set to `true` to disable SSL/TLS certificate validation for the `host`. Useful for self-hosted instances with self-signed certificates. **Use with caution.** The setting is process-wide: it applies to every Phase provider configuration in the same Terraform run. Defaults to `false`.

## Resources

### `phase_secret`

Manages a single secret within a specific application and environment in Phase. Works with end-to-end encrypted (E2EE) and server-side encrypted (SSE) apps alike.

The provider handles create, read, update, and delete operations. If a `phase_secret` resource is defined for a secret that already exists (based on `app_id`, `env`, `path`, `key`), the provider will manage the existing secret and update it if necessary, rather than failing.

#### Argument Reference

*   `app_id` - (Required, ForceNew) The UUID of the Phase application where the secret resides. Changing this forces a new resource to be created.
*   `env` - (Required, ForceNew) The name of the environment within the application (e.g., `development`, `production`). Changing this forces a new resource to be created.
*   `key` - (Required) The key (name) of the secret (e.g., `DATABASE_URL`, `API_KEY`). Stored exactly as written.
*   `value` - (Required, Sensitive) The value of the secret (an empty string is allowed). Secret references such as `${OTHER_KEY}` are stored verbatim; write them as `$${OTHER_KEY}` in HCL.
*   `path` - (Optional) The path where the secret is stored within the environment. Defaults to `/` (root). Example: `/database/credentials`.
*   `comment` - (Optional) A description or comment for the secret.
*   `tags` - (Optional) A list of tag names. Tags that do not exist yet are created automatically.
*   `type` - (Optional, Computed) The secret type: `secret` (default), `config` or `sealed`. Sealed secrets cannot be unsealed, and changing the value of a sealed secret re-creates it.
*   `override` - (Optional) A block to configure a **Personal Secret Override**. This requires authenticating with a User Token (PAT).
    *   `value` - (Required, Sensitive) The value to use when this override is active for the authenticated user.
    *   `is_active` - (Required, Boolean) Must be set to `true`: an override defined here is created active. Deactivating an existing override is done in the Phase Console or CLI. While an override is active, `value` keeps the configured shared value and is not compared against the server.

#### Attribute Reference

In addition to the arguments above, the following attributes are exported:

*   `id` - The unique UUID assigned to the secret by Phase upon creation.
*   `version` - The current version number of the secret. Incremented on each update.
*   `created_at` - The timestamp (UTC RFC3339 format) when the secret was first created.
*   `updated_at` - The timestamp (UTC RFC3339 format) when the secret was last updated.

### `phase_app`

Manages a Phase application. Apps created through the provider use server-side encryption (SSE), which the management resources below require.

#### Argument Reference

*   `name` - (Required) The name of the app.
*   `description` - (Optional) A description of the app.
*   `environments` - (Optional) Names of custom environments to create with the app (paid plans). Defaults to `Development`, `Staging` and `Production`. Only applied at creation and ignored afterwards; use `phase_environment` to manage environments.

#### Attribute Reference

*   `id` - The UUID of the app.
*   `sse_enabled` - Whether server-side encryption is enabled.
*   `created_at`, `updated_at` - Timestamps.

Import with `terraform import phase_app.<name> "<app_id>"`.

**Destroying an app permanently deletes its environments and every secret in them.** Consider `lifecycle { prevent_destroy = true }` on apps that hold production secrets.

### `phase_environment`

Manages an environment within an SSE-enabled app. Creating custom environments requires a paid plan; default environments can be imported and renamed on any plan.

#### Argument Reference

*   `app_id` - (Required, ForceNew) The UUID of the app.
*   `name` - (Required) The environment name (letters, digits, `-` and `_`; max 64 characters).

#### Attribute Reference

*   `id` - The UUID of the environment.
*   `env_type` - `dev`, `staging`, `prod` or `custom`.
*   `index` - The display order of the environment.
*   `created_at`, `updated_at` - Timestamps.

Import with `terraform import phase_environment.<name> "<app_id>:<environment_id>"`.

### `phase_service_account`

Manages a service account with server-side key management, its initial token and its app access.

```hcl
data "phase_role" "service" {
  name = "Service"
}

resource "phase_service_account" "ci" {
  name    = "ci"
  role_id = data.phase_role.service.id

  access {
    app_id       = phase_app.backend.id
    environments = ["Development", "Production"]
  }
}

output "ci_token" {
  value     = phase_service_account.ci.token
  sensitive = true
}
```

#### Argument Reference

*   `name` - (Required) The name of the service account.
*   `role_id` - (Required) The ID of the role to assign. Use the `phase_role` data source to look it up by name.
*   `token_name` - (Optional) The name of the token created with the service account. Defaults to `Default`. Only applied at creation and ignored afterwards; imported service accounts keep whatever tokens they have.
*   `access` - (Optional) One block per app the service account may access. Declarative: apps not listed lose access when the block is set. Only SSE-enabled apps can be granted; access to end-to-end encrypted apps granted in the Console is left untouched and not shown.
    *   `app_id` - (Required) The UUID of the app.
    *   `environments` - (Required) Environment names within the app, as shown in the Console (e.g. `Development`).

#### Attribute Reference

*   `id` - The UUID of the service account.
*   `token` - (Sensitive) The full initial token (`pss_service:v2:...`). Only available at creation; it is stored in state.
*   `bearer_token` - (Sensitive) The initial token's REST API bearer value (`ServiceAccount <token>`).
*   `token_id` - The ID of the initial token.
*   `created_at`, `updated_at` - Timestamps.

Import with `terraform import phase_service_account.<name> "<service_account_id>"` (token attributes are not recoverable).

### `phase_service_account_token`

Creates an additional token for a service account. Tokens cannot be modified; any change forces a new token. A token deleted outside Terraform is re-created on the next apply.

#### Argument Reference

*   `service_account_id` - (Required, ForceNew) The UUID of the service account.
*   `name` - (Required, ForceNew) The token name.
*   `expires_in` - (Optional, ForceNew) Seconds until the token expires. Conflicts with `expires_at`.
*   `expires_at` - (Optional, Computed, ForceNew) RFC 3339 expiry timestamp with a UTC offset, e.g. `2027-01-01T00:00:00Z`. Without `expires_in` or `expires_at` the token never expires.

#### Attribute Reference

*   `id` - The UUID of the token.
*   `token` - (Sensitive) The full token (`pss_service:v2:...`), only available at creation.
*   `bearer_token` - (Sensitive) The REST API bearer value (`ServiceAccount <token>`).
*   `created_at` - Timestamp.

### `phase_invite`

Invites a user to the organisation by email. Once the invite is accepted the resource is kept in state; if it expires or is cancelled outside Terraform it is re-created on the next apply. Invites cannot be assigned roles with global access or roles that can create service account tokens.

#### Argument Reference

*   `email` - (Required, ForceNew) The email address to invite.
*   `role_id` - (Required, ForceNew) The ID of the role to assign. Use the `phase_role` data source to look it up by name.

#### Attribute Reference

*   `id` - The UUID of the invite.
*   `expires_at`, `created_at` - Timestamps. Invites expire after 14 days.

## Data Sources

### `phase_secrets`

Fetches multiple secrets from Phase based on specified filters. Secret references such as `${OTHER_KEY}` are resolved.

#### Argument Reference

*   `app_id` - (Required) The UUID of the Phase application.
*   `env` - (Required) The name of the environment.
*   `path` - (Optional) The path to fetch secrets from. Defaults to `/` (root). Use an empty string (`""`) to fetch secrets from all paths.
*   `key` - (Optional) The key of a *specific* secret to fetch.
*   `tags` - (Optional) A list of tag names to filter secrets by. Secrets matching *any* of the provided tags will be included (OR logic).

#### Attribute Reference

*   `secrets` - (Computed, Sensitive) A map where keys are the secret keys (e.g., `DATABASE_URL`) and values are their corresponding secret values. If a Personal Secret Override is active for the authenticated user, the override value will be returned here. The map is empty when nothing matches.
*   `id` - A unique identifier constructed by the provider for this data source instance based on the input arguments (`app_id`, `env`, `path`, `key`, `tags`).

### `phase_role`

Looks up an organisation role by name, e.g. to reference its ID from `phase_invite` or `phase_service_account`.

#### Argument Reference

*   `name` - (Required) The role name (case-insensitive), e.g. `Developer` or `Service`.

#### Attribute Reference

*   `id` - The UUID of the role.
*   `description` - The role description.
*   `is_default` - Whether the role is one of the built-in roles.

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
*   `{path}`: The path where the secret exists in Phase, e.g. `/`, `/database` or `/folder/path` (a trailing slash is accepted).
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
    *   **Managing (`resource "phase_secret"`):** An `override` block in a `phase_secret` resource creates the override *active* for the authenticated user (or updates its value). The provider does not deactivate an existing override; do that in the Phase Console or CLI.
*   **Visibility:** Overrides are personal. Only the user who created and activated the override (and is authenticated with their PAT) will see the overridden value via the provider.

### Working with Tags

Tags provide a way to categorize and filter secrets. Tags referenced from Terraform are created automatically if they do not exist yet.

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

### Provisioning an app end to end

```hcl
resource "phase_app" "backend" {
  name        = "backend"
  description = "Managed by Terraform"
}

resource "phase_secret" "db_url" {
  app_id = phase_app.backend.id
  env    = "production"
  key    = "DATABASE_URL"
  value  = "postgres://..."
}

data "phase_role" "service" {
  name = "Service"
}

resource "phase_service_account" "deploy" {
  name    = "deploy"
  role_id = data.phase_role.service.id

  access {
    app_id       = phase_app.backend.id
    environments = ["Production"]
  }
}

resource "phase_service_account_token" "ci" {
  service_account_id = phase_service_account.deploy.id
  name               = "ci"
  expires_in         = 60 * 60 * 24 * 90
}

data "phase_role" "developer" {
  name = "Developer"
}

resource "phase_invite" "alice" {
  email   = "alice@example.com"
  role_id = data.phase_role.developer.id
}
```
