# Terraform Provider for Phase

This Terraform provider allows you to manage secrets in Phase from your Terraform configurations.

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

## Using the provider

To use the latest version of the provider in your Terraform configuration, add the following terraform block:

```hcl
terraform {
  required_providers {
    phase = {
      source = "phasehq/phase"
    }
  }
}

provider "phase" {
  # Configuration options
}
```

See the [Phase Provider documentation](docs/index.md) for all the available options.

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```sh
$ make testacc
```