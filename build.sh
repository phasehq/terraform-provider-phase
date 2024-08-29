go build -o terraform-provider-phase

rm -rf  ~/.terraform.d/plugins/registry.terraform.io/phasehq/phase/1.0.0/lin
ux_amd64/

mv terraform-provider-phase ~/.terraform.d/plugins/registry.terraform.io/phasehq/phase/0.1.0/$(go env GOOS)_$(go env GOARCH)

rm .terraform.lock.hcl

terraform init

terraform plan