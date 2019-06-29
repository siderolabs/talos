workflow "on push, run conform" {
  on       = "push"

  resolves = [
    "conform"
  ]
}

action "conform" {
  uses    = "docker://talos-systems/conform:v0.1.0-alpha.12"

  secrets = [
    "GITHUB_TOKEN"
  ]
}
