# Phase Provider Documentation

The Phase Terraform provider allows you to manage applications and secrets in Phase from your Terraform configurations.

## Example Usage

The following example creates a Phase application, manages a secret within that application, and retrieves secrets using the `phase_secrets` data source.

```hcl
terraform {
  required_providers {
    phase = {
      source  = "phasehq/phase"
      version = "0.2.0"
    }

    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

provider "phase" {
  # phase_token can also be provided through the PHASE_TOKEN environment variable.
  # host = "https://your-self-hosted-phase.com"
  # skip_tls_verification = true
}

resource "phase_application" "example" {
  name        = "my-application"
  description = "Application managed by Terraform"
}

resource "random_password" "db_password" {
  length           = 32
  special          = true
  override_special = "_%@"
}

resource "phase_secret" "database_password" {
  app_id = phase_application.example.id
  env    = "production"
  key    = "DATABASE_PASSWORD"
  value  = random_password.db_password.result
  path   = "/database/"
  tags   = ["database", "credentials"]
  comment = "Managed by Terraform"
}

data "phase_secrets" "database" {
  app_id = phase_application.example.id
  env    = "production"
  path   = "/database/"
  tags   = ["database"]
}

output "database_password" {
  value     = data.phase_secrets.database.secrets["DATABASE_PASSWORD"]
  sensitive = true
}

output "application_id" {
  value = phase_application.example.id
}

output "managed_secret_version" {
  value = phase_secret.database_password.version
}

output "managed_secret_updated_at" {
  value = phase_secret.database_password.updated_at
}
```

## Provider Configuration

The following arguments are supported in the `provider "phase"` block.

### `phase_token`

Optional. The Phase authentication token.

The provider supports both:

* Service Tokens (`pss_service:...`)
* Personal Access Tokens (`pss_user:...`)

The token can also be provided through an environment variable. The provider checks the following variables in order:

1. `PHASE_TOKEN`
2. `PHASE_SERVICE_TOKEN`
3. `PHASE_PAT_TOKEN`

A token configured directly in the provider configuration takes precedence over environment variables.

The token is sensitive and should not be committed to source control.

Example:

```hcl
provider "phase" {
  phase_token = var.phase_token
}
```

Or using an environment variable:

```sh
export PHASE_TOKEN="pss_service:v1:..."
```

### `host`

Optional. The base URL for the Phase API.

Default:

```text
https://api.phase.dev
```

For self-hosted Phase instances, provide the appropriate host URL.

The provider automatically appends `/service/public` to custom hosts when required by the Phase API.

Example:

```hcl
provider "phase" {
  host = "https://your-self-hosted-phase.com"
}
```

The host can also be configured using the `PHASE_HOST` environment variable.

### `skip_tls_verification`

Optional. Disables TLS certificate verification when set to `true`.

This can be useful for self-hosted Phase instances using self-signed certificates.

Defaults to `false`.

```hcl
provider "phase" {
  host                 = "https://your-self-hosted-phase.com"
  skip_tls_verification = true
}
```

Use this option with caution.

## Resources

### `phase_application`

Manages an application in Phase.

#### Example Usage

```hcl
resource "phase_application" "example" {
  name        = "my-application"
  description = "Application managed by Terraform"
}
```

#### Argument Reference

* `name` - (Required) The name of the application.
* `description` - (Optional) A description for the application.

#### Attribute Reference

In addition to the arguments above, the following attributes are exported:

* `id` - The unique identifier assigned to the application by Phase.
* `created_at` - The timestamp when the application was created.
* `updated_at` - The timestamp when the application was last updated.

#### Using an Application with Secrets

The application ID can be referenced directly by `phase_secret` resources and `phase_secrets` data sources:

```hcl
resource "phase_application" "example" {
  name = "my-application"
}

resource "phase_secret" "api_key" {
  app_id = phase_application.example.id
  env    = "production"
  key    = "API_KEY"
  value  = "my-secret-value"
}
```

### `phase_secret`

Manages a single secret within a specific application and environment in Phase.

The provider supports create, read, update, and delete operations. If a secret already exists for the specified application, environment, and key, the provider attempts to manage the existing secret instead of failing during creation.

#### Argument Reference

* `app_id` - (Required, ForceNew) The ID of the Phase application where the secret resides. Changing this forces a new resource to be created.
* `env` - (Required, ForceNew) The name of the environment within the application, such as `development` or `production`.
* `key` - (Required) The key of the secret, such as `DATABASE_URL` or `API_KEY`.
* `value` - (Required, Sensitive) The value of the secret.
* `path` - (Optional) The path where the secret is stored. Defaults to `/`.
* `comment` - (Optional) A description or comment for the secret.
* `tags` - (Optional) A list of tags assigned to the secret.
* `override` - (Optional) A block used to configure a Personal Secret Override. This requires authentication with a User Token (PAT).

  * `value` - (Required, Sensitive) The value to use when the override is active.
  * `is_active` - (Required, Boolean) Whether the override is active.

#### Attribute Reference

In addition to the arguments above, the following attributes are exported:

* `id` - The unique identifier assigned to the secret by Phase.
* `version` - The current version of the secret.
* `created_at` - The timestamp when the secret was created.
* `updated_at` - The timestamp when the secret was last updated.

#### Example

```hcl
resource "phase_secret" "database_password" {
  app_id = phase_application.example.id
  env    = "production"
  key    = "DATABASE_PASSWORD"
  value  = "my-secret-password"
  path   = "/database/"
  tags   = ["database", "credentials"]
  comment = "Managed by Terraform"
}
```

## Data Sources

### `phase_secrets`

Fetches multiple secrets from Phase using application, environment, path, key, and tag filters.

#### Argument Reference

* `app_id` - (Required) The ID of the Phase application.
* `env` - (Required) The name of the environment.
* `path` - (Optional) The path used to filter the returned secrets. Defaults to `/`.
* `key` - (Optional) The key of a specific secret to fetch.
* `tags` - (Optional) A list of tags used to filter secrets.

#### Attribute Reference

* `secrets` - (Computed, Sensitive) A map where each key is a secret key and each value is the corresponding secret value.

If a Personal Secret Override is active for the authenticated user, the override value is returned instead of the regular secret value.

* `id` - A unique identifier constructed by the provider from the data source arguments.

#### Example

```hcl
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

### Fetching a Specific Secret

Use the `key` argument when only a specific secret is required:

```hcl
data "phase_secrets" "database_password" {
  app_id = phase_application.example.id
  env    = "production"
  path   = "/database/"
  key    = "DATABASE_PASSWORD"
}

output "database_password" {
  value     = data.phase_secrets.database_password.secrets["DATABASE_PASSWORD"]
  sensitive = true
}
```

### Filtering by Tags

Tags can be supplied to the `phase_secrets` data source:

```hcl
data "phase_secrets" "backend" {
  app_id = phase_application.example.id
  env    = "production"
  path   = "/backend/"
  tags   = ["api", "database"]
}
```

The tags are passed to the Phase API as filters.

## Importing

Existing secrets managed outside Terraform can be imported into Terraform state.

### Importing a Secret

Use the following ID format:

```bash
terraform import phase_secret.<resource_name> "{app_id}:{env}:{path}:{key}"
```

For example:

```bash
terraform import phase_secret.imported_secret "907549ca-1430-4aa0-9998-290525741005:production:/database/:DB_HOST"
```

The components are:

* `app_id` - The ID of the Phase application.
* `env` - The environment containing the secret.
* `path` - The exact path where the secret exists.
* `key` - The key of the secret.

After importing the secret, run:

```bash
terraform plan
```

to review the imported state against your Terraform configuration.

## Personal Secret Overrides

Personal Secret Overrides allow an individual user to use a different value for a secret without changing the globally stored secret.

Overrides require authentication with a Personal Access Token (`pss_user:...`).

### Reading Overrides

When an active override exists for the authenticated user, the `phase_secrets` data source returns the override value.

### Managing Overrides

A `phase_secret` resource can configure an override:

```hcl
resource "phase_secret" "api_key" {
  app_id = phase_application.example.id
  env    = "development"
  key    = "API_KEY"
  value  = "production-api-key"

  override {
    value     = "local-development-api-key"
    is_active = true
  }
}
```

The override is personal to the authenticated user.

## Working with Tags

Tags can be assigned to secrets and used as filters when reading secrets.

Tags must already exist in Phase before they can be assigned to a secret.

### Assigning Tags

```hcl
resource "phase_secret" "api_key" {
  app_id = phase_application.example.id
  env    = "production"
  key    = "THIRD_PARTY_API_KEY"
  value  = "my-api-key"
  path   = "/integrations/"
  tags   = ["api", "billing", "external"]
}
```

### Filtering by Tags

```hcl
data "phase_secrets" "backend" {
  app_id = phase_application.example.id
  env    = "production"
  path   = "/backend/"
  tags   = ["api", "billing"]
}
```

## Secret Metadata

The `phase_secret` resource exports metadata about the managed secret.

```hcl
resource "phase_secret" "config" {
  app_id = phase_application.example.id
  env    = "production"
  key    = "FEATURE_FLAG_X"
  value  = "true"
}

output "config_version" {
  value = phase_secret.config.version
}

output "config_last_updated" {
  value = phase_secret.config.updated_at
}
```

## Development

Run the complete test suite with:

```sh
go test -count=1 ./...
```

To run tests for a specific package:

```sh
go test -count=1 ./internal/resources
```

To run tests with verbose output:

```sh
go test -count=1 -v ./...
```

To generate provider documentation:

```sh
go generate
```
