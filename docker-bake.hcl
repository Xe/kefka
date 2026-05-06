variable "ALPINE_VERSION" { default = "3.23" }
variable "GO_VERSION" { default = "1.26" }

group "default" {
  targets = [
    "sophia",
  ]
}

target "sophia" {
  args = {
    ALPINE_VERSION = null
    GO_VERSION = null
  }
  context = "."
  dockerfile = "./docker/sophia.Dockerfile"
  platforms = [ "linux/amd64", "linux/arm64" ]
  pull = true
  tags = [
    "ghcr.io/xe/kefka/sophia:main"
  ]
}
