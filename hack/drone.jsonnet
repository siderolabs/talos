local build_container = "autonomy/build-container:latest";
local dind_image = "docker:19.03-dind";
local default_buildkit_endpoint = "tcp://buildkitd.ci.svc:1234";
local bin_dir = "/usr/local/bin";
local build_env_vars = {
    BUILDKIT_HOST: std.format("${BUILDKIT_HOST=%s}", default_buildkit_endpoint),
    BINDIR: bin_dir,
};
local creds_env_vars = {
    AZURE_SVC_ACCT: {from_secret: "azure_svc_acct"},
    GCE_SVC_ACCT: {from_secret: "gce_svc_acct"},
    PACKET_AUTH_TOKEN: {from_secret: "packet_auth_token"},
};
local aws_env_vars = {
    AWS_ACCESS_KEY_ID: {from_secret: "aws_access_key_id"},
    AWS_SECRET_ACCESS_KEY: {from_secret: "aws_secret_access_key"},
    AWS_DEFAULT_REGION: "us-west-2",
    AWS_PUBLISH_REGIONS: "us-west-2,us-east-1,us-east-2,us-west-1,eu-central-1",
};
local node = {"node-role.kubernetes.io/ci": ""};
local volume_dockersock = {
  name: "dockersock",
  temp: {},
};
local volume_tmp = {
  name: "tmp",
  temp: {},
};
local volume_dev = {
  name: "dev",
  host: {
    path: "/dev"
  },
};
local pipeline_volumes = [
  volume_dockersock,
  volume_tmp,
  volume_dev,
];
local step_volumes = [
  {
    name: volume_dockersock.name,
    path: "/var/run",
  },
  {
    name: volume_tmp.name,
    path: "/tmp",
  },
  {
    name: volume_dev.name,
    path: "/dev",
  },
];

// Common steps

local clone = {
  name: "clone",
  image: build_container,
  commands: [
    "git init",
    "git remote add origin ${DRONE_REMOTE_URL}",
    "git fetch origin +refs/heads/${DRONE_COMMIT_BRANCH}:",
    "git checkout ${DRONE_COMMIT_BRANCH}",
    "git fetch origin ${DRONE_COMMIT_REF}:",
    "git merge ${DRONE_COMMIT_SHA}",
    "git fetch --tags",
  ],
  when: {
    event: {
      exclude: [""],
    },
  },
};

local docker = {
  name: "docker",
  image: dind_image,
  entrypoint: ["dockerd"],
  privileged: true,
  command: [
    "--dns=8.8.8.8",
    "--dns=8.8.4.4",
    "--mtu=1440",
    "--log-level=error",
  ],
  volumes: step_volumes,
};

local buildkit = {
  name: "buildkit",
  image: "moby/buildkit:v0.6.0",
  privileged: true,
  detach: true,
  commands: ["buildkitd --addr tcp://0.0.0.0:1234 --allow-insecure-entitlement security.insecure"],
  when: {
    event: {
      include: [""],
    },
  },
};

local notify = {
  name: "slack",
  image: "plugins/slack",
  settings:
    {
     webhook: {from_secret: "slack_webhook"},
     channel: "proj-talos-maint",
    },
};

// Functions

local step(name, depends_on=[], environment={}, target="") = {
  name: std.format("%s", name),
  image: build_container,
  commands: [if target == "" then std.format("make %s", name) else std.format("make %s", target)],
  environment: build_env_vars+environment,
  volumes: step_volumes,
  depends_on: [x.name for x in depends_on],
};


local pipeline(name, steps=[]) = {
  kind: "pipeline",
  name: name,
  node: node,
  services: if name != "notify" then [docker] else [],
  volumes: if name != "notify" then pipeline_volumes else [],
  clone: {
    disable: true,
  },
  steps: if name != "notify" then [clone, buildkit] + steps else steps,
};

// Apps

local machined = step("machined", [clone]);
local osd = step("osd", [clone]);
local trustd = step("trustd", [clone]);
local proxyd = step("proxyd", [clone]);
local ntpd = step("ntpd", [clone]);
local osctl_linux = step("osctl-linux", [clone]);
local osctl_darwin = step("osctl-darwin", [clone]);

// Artifacts

local rootfs =  step("rootfs", [machined, osd, trustd, proxyd, ntpd]);
local initramfs = step("initramfs", [rootfs]);
local installer = step("installer", [rootfs]);
local container = step("container", [rootfs]);
local image_azure = step("image-azure", [installer]);
local image_gce = step("image-gce", [installer]);
local kernel = step("kernel", [clone]);
local iso = step("iso", [installer]);

// CAPI.

local push_image_azure = step("push-image-azure", [image_azure], creds_env_vars);
local push_image_gce = step("push-image-gce", [image_gce], creds_env_vars);

// Tests

local lint = step("lint", [clone]);
local unit_tests = step("unit-tests", [rootfs]);
local unit_tests_race = step("unit-tests-race", [rootfs]);
local basic_integration = step("basic-integration", [container, osctl_linux]);
local capi = step("capi", [basic_integration], creds_env_vars);
local e2e_integration_azure = step("e2e-integration-azure", [capi, push_image_azure], {PLATFORM: "azure"}, "e2e-integration");
local e2e_integration_gce = step("e2e-integration-gce", [capi, push_image_gce], {PLATFORM: "gce"}, "e2e-integration");
local conformance_azure = step("conformance-azure", [capi, push_image_azure], {PLATFORM: "azure", CONFORMANCE: "run"}, "e2e-integration");
local conformance_gce = step("conformance-gce", [capi, push_image_gce], {PLATFORM: "gce", CONFORMANCE: "run"}, "e2e-integration");


local coverage = {
  name: "coverage",
  image: "plugins/codecov",
  settings:{
    token: {from_secret: "codecov_token"},
    files: ["coverage.txt"],
  },
  trigger: {
    event: ["pull_request"],
  },
  depends_on: [unit_tests.name],
};

local push = {
  name: "push",
  image: "autonomy/build-container:latest",
  pull: "always",
  environment:{
    DOCKER_USERNAME: {from_secret: "docker_username"},
    DOCKER_PASSWORD: {from_secret: "docker_password"},
  },
  commands: ["make gitmeta", "make login", "make push"],
  volumes: step_volumes,
  when: {
    event: ["push"],
  },
  depends_on: [container.name],
};

local image_aws = step("image-aws", [push], aws_env_vars);

// Steps

local apps = [
    machined,
    osd,
    trustd,
    proxyd,
    ntpd,
    osctl_linux,
    osctl_darwin,
];

local artifacts = [
  rootfs,
  initramfs,
  installer,
  container,
];

local tests = [
  lint,
  unit_tests,
  unit_tests_race,
  coverage,
  basic_integration,
];

local e2e_setup = apps + artifacts + [
  image_azure,
  image_gce,
  push_image_azure,
  push_image_gce,
  basic_integration,
  capi,
];

local e2e = e2e_setup + [
  e2e_integration_azure,
  e2e_integration_gce,
];

local conformance = e2e_setup + [
  conformance_azure,
  conformance_gce,
];


local release_step ={
  name: "release",
  image: "plugins/github-release",
  settings:{
    api_key: {from_secret: "github_token"},
    draft: true,
    files: ["build/*"],
    checksum: ["sha256", "sha512"],
  },
  depends_on: [kernel.name, iso.name, image_gce.name, image_azure.name, image_aws.name, push.name]
};

local release = apps + artifacts + [
  kernel,
  image_azure,
  image_gce,
  iso,
  push,
  image_aws,
  release_step,
];

// Triggers

local default_trigger = {
  trigger: {
    cron: {
      exclude: ["nightly"]
    },
    target: {
      exclude: ["e2e", "conformance", "release"]
    },
  },
};

local nightly_trigger = {
  trigger: {
    cron: {
      include: ["nightly"]
    },
  },
};

local e2e_trigger = {
  trigger: {
    target: {
      include: ["e2e"]
    },
  },
};

local conformance_trigger = {
  trigger: {
    target: {
      include: ["conformance"]
    },
  },
};

local release_trigger = {
  trigger: {
    event: ["tag"],
  },
};

local notify_trigger = {
  trigger: {
    status: ["success", "failure"],
  },
};

local notify_depends_on = {
  depends_on: ["default", "e2e", "conformance", "nightly", "release"],
};

local default_pipeline = pipeline("default", [ x for x in apps + artifacts + tests ] + [push]) + default_trigger;
local e2e_pipeline = pipeline("e2e", e2e) + e2e_trigger;
local conformance_pipeline = pipeline("conformance", conformance) + conformance_trigger;
local nightly_pipeline = pipeline("nightly", conformance) + nightly_trigger;
local release_pipeline = pipeline("release", release) + release_trigger;
local notify_pipeline = pipeline("notify", [notify]) + notify_trigger + notify_depends_on;

[
  default_pipeline,
  e2e_pipeline,
  conformance_pipeline,
  nightly_pipeline,
  release_pipeline,
  notify_pipeline,
]
