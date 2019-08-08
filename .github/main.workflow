workflow "Conform Pull Request" {
  on       = "pull_request"

  resolves = [
    "conform"
  ]
}

action "conform" {
  uses    = "docker://autonomy/conform:latest"

  secrets = [
    "GITHUB_TOKEN"
  ]
}
