workflow "Conform" {
  on       = "pull_request"

  resolves = [
    "conform"
  ]
}

action "conform" {
  uses = "docker://autonomy/conform:v0.1.0-alpha.12"
}

workflow "auto-label" {
  on       = "pull_request"

  resolves = [
    "Auto label"
  ]
}

action "Auto label" {
  uses    = "banyan/auto-label@master"

  secrets = [
    "GITHUB_TOKEN"
  ]
}
