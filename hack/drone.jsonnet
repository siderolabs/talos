// This file contains the logic for building our CI for Drone. The idea here is
// that we create a pipeline for all of the major tasks we need to perform
// (e.g. builds, E2E testing, conformance testing, releases). Each pipeline
// after the default builds on a previous pipeline.
// Generate with `drone jsonnet --source ./hack/drone.jsonnet --stream --format`

local build_container = 'autonomy/build-container:latest';

local secret = {
  kind: 'secret',
  name: 'kubeconfig',
  get: {
    path: 'buildx',
    name: 'kubeconfig'
  },
};

local volumes = {
  dockersock: {
    pipeline: {
      name: 'dockersock',
      temp: {},
    },
    step: {
      name: $.dockersock.pipeline.name,
      path: '/var/run',
    },
  },

  docker: {
    pipeline: {
      name: 'docker',
      temp: {},
    },
    step: {
      name: $.docker.pipeline.name,
      path: '/root/.docker/buildx',
    },
  },

  kube: {
    pipeline: {
      name: 'kube',
      temp: {},
    },
    step: {
      name: $.kube.pipeline.name,
      path: '/root/.kube',
    },
  },

  dev: {
    pipeline: {
      name: 'dev',
      host: {
        path: '/dev',
      },
    },
    step: {
      name: $.dev.pipeline.name,
      path: '/dev',
    },
  },

  tmp: {
    pipeline: {
      name: 'tmp',
      temp: {},
    },
    step: {
      name: $.tmp.pipeline.name,
      path: '/tmp',
    },
  },

  ForStep(): [
    self.dockersock.step,
    self.docker.step,
    self.kube.step,
    self.dev.step,
    self.tmp.step,
  ],

  ForPipeline(): [
    self.dockersock.pipeline,
    self.docker.pipeline,
    self.kube.pipeline,
    self.dev.pipeline,
    self.tmp.pipeline,
  ],
};

// This provides the docker service.
local docker = {
  name: 'docker',
  image: 'docker:19.03-dind',
  entrypoint: ['dockerd'],
  privileged: true,
  command: [
    '--dns=8.8.8.8',
    '--dns=8.8.4.4',
    '--mtu=1500',
    '--log-level=error',
  ],
  ports: [
    6443,
    50000,
  ],
  volumes: volumes.ForStep(),
};

// Sets up the CI environment
local setup_ci = {
  name: 'setup-ci',
  image: 'autonomy/build-container:latest',
  privileged: true,
  environment: {
    BUILDX_KUBECONFIG: { from_secret: secret.name },
  },
  commands: [
    'git fetch --tags',
    'apk add coreutils',
    'echo -e "$BUILDX_KUBECONFIG" > /root/.kube/config',
    'docker buildx create --driver kubernetes --driver-opt replicas=2 --driver-opt namespace=ci --driver-opt image=moby/buildkit:v0.6.2 --name ci --buildkitd-flags="--allow-insecure-entitlement security.insecure" --use',
    'docker buildx inspect --bootstrap',
    'make ./_out/sonobuoy',
    'make ./_out/kubectl',
  ],
  volumes: volumes.ForStep(),
};

// Step standardizes the creation of build steps. The name of the step is used
// as the target when building the make command. For example, if name equals
// "test", the resulting step command will be "make test". This is done to
// encourage alignment between this file and the Makefile, and gives us a
// standardized structure that should make things easier to reason about if we
// know that each step is essentially a Makefile target.
local Step(name, image='', target='', privileged=false, depends_on=[], environment={}) = {
  local make = if target == '' then std.format('make %s', name) else std.format('make %s', target),

  local common_env_vars = {},

  name: name,
  image: if image == '' then build_container else image,
  pull: "always",
  commands: [make],
  privileged: privileged,
  environment: common_env_vars + environment,
  volumes: volumes.ForStep(),
  depends_on: [x.name for x in depends_on],
};

// Pipeline is a way to standardize the creation of pipelines. It supports
// using and existing pipeline as a base.
local Pipeline(name, steps=[], depends_on=[], with_docker=true, disable_clone=false) = {
  local node = { 'node-role.kubernetes.io/ci': '' },

  kind: 'pipeline',
  name: name,
  node: node,
  services: [
    if with_docker then docker,
  ],
  [ if disable_clone then 'clone']: {
    disable: true,
  },
  steps: steps,
  volumes: volumes.ForPipeline(),
  depends_on: [x.name for x in depends_on],
};

// Default pipeline.

local machined = Step("machined", depends_on=[setup_ci]);
local osd = Step("osd", depends_on=[setup_ci]);
local trustd = Step("trustd", depends_on=[setup_ci]);
local ntpd = Step("ntpd", depends_on=[setup_ci]);
local networkd = Step("networkd", depends_on=[setup_ci]);
local apid = Step("apid", depends_on=[setup_ci]);
local osctl_linux = Step("osctl-linux", depends_on=[setup_ci]);
local osctl_darwin = Step("osctl-darwin", depends_on=[setup_ci]);
local docs = Step("docs", depends_on=[osctl_linux]);
local kernel = Step('kernel', depends_on=[setup_ci]);
local initramfs = Step("initramfs", depends_on=[apid, machined, networkd, ntpd, osd, trustd]);
local installer = Step("installer", depends_on=[initramfs]);
local talos = Step("talos", depends_on=[installer]);
local golint = Step("lint-go", depends_on=[setup_ci]);
local protobuflint = Step("lint-protobuf", depends_on=[setup_ci]);
local markdownlint = Step("lint-markdown", depends_on=[setup_ci]);
local image_aws = Step("image-aws", depends_on=[installer]);
local image_azure = Step("image-azure", depends_on=[installer]);
local image_digital_ocean = Step("image-digital-ocean", depends_on=[installer]);
local image_gcp = Step("image-gcp", depends_on=[installer]);
local image_vmware = Step("image-vmware", depends_on=[installer]);
local unit_tests = Step("unit-tests", depends_on=[talos]);
local unit_tests_race = Step("unit-tests-race", depends_on=[golint]);
local e2e_docker = Step("e2e-docker", depends_on=[unit_tests, talos, osctl_linux]);
local e2e_firecracker = Step("e2e-firecracker", privileged=true, depends_on=[unit_tests, osctl_linux, kernel]);

local coverage = {
  name: 'coverage',
  image: 'alpine:3.10',
  environment: {
    CODECOV_TOKEN: { from_secret: 'codecov_token' },
  },
  commands: [
    'apk --no-cache add bash curl git',
    'bash -c "bash <(curl -s https://codecov.io/bash) -f _out/coverage.txt -X fix"'
  ],
  when: {
    event: ['pull_request'],
  },
  depends_on: [unit_tests.name],
};

local push = {
  name: 'push',
  image: 'autonomy/build-container:latest',
  pull: 'always',
  environment: {
    DOCKER_USERNAME: { from_secret: 'docker_username' },
    DOCKER_PASSWORD: { from_secret: 'docker_password' },
  },
  commands: ['make push'],
  volumes: volumes.ForStep(),
  when: {
    event: {
      exclude: [
        'pull_request',
        'promote',
        'cron',
      ],
    },
  },
  depends_on: [e2e_docker.name, e2e_firecracker.name],
};

local push_latest = {
  name: 'push-latest',
  image: 'autonomy/build-container:latest',
  pull: 'always',
  environment: {
    DOCKER_USERNAME: { from_secret: 'docker_username' },
    DOCKER_PASSWORD: { from_secret: 'docker_password' },
  },
  commands: ['make push-latest'],
  volumes: volumes.ForStep(),
  when: {
    branch: [
      'master',
    ],
    event: [
      'push',
    ],
  },
  depends_on: [e2e_docker.name, e2e_firecracker.name],
};

local default_steps = [
  setup_ci,
  machined,
  osd,
  apid,
  trustd,
  ntpd,
  networkd,
  osctl_linux,
  osctl_darwin,
  docs,
  kernel,
  initramfs,
  installer,
  talos,
  golint,
  protobuflint,
  markdownlint,
  image_aws,
  image_azure,
  image_digital_ocean,
  image_gcp,
  image_vmware,
  unit_tests,
  unit_tests_race,
  coverage,
  e2e_docker,
  e2e_firecracker,
  push,
  push_latest,
];

local default_trigger = {
  trigger: {
    cron: {
      exclude: ['nightly'],
    },
    event: {
      exclude: [
        'tag',
        'promote',
      ],
    },
  },
};

local default_pipeline = Pipeline('default', default_steps) + default_trigger;

// E2E pipeline.

local creds_env_vars = {
  AWS_ACCESS_KEY_ID: { from_secret: 'aws_access_key_id' },
  AWS_SECRET_ACCESS_KEY: { from_secret: 'aws_secret_access_key' },
  AWS_SVC_ACCT: {from_secret: "aws_svc_acct"},
  AZURE_SVC_ACCT: {from_secret: "azure_svc_acct"},
  // TODO(andrewrynhard): Rename this to the GCP convention.
  GCE_SVC_ACCT: {from_secret: "gce_svc_acct"},
  PACKET_AUTH_TOKEN: {from_secret: "packet_auth_token"},
};

local e2e_capi = Step("e2e-capi", depends_on=[e2e_docker, e2e_firecracker], environment=creds_env_vars);
local e2e_aws = Step("e2e-aws", depends_on=[e2e_capi], environment=creds_env_vars);
local e2e_azure = Step("e2e-azure", depends_on=[e2e_capi], environment=creds_env_vars);
local e2e_gcp = Step("e2e-gcp", depends_on=[e2e_capi], environment=creds_env_vars);

local e2e_steps = default_steps + [
  e2e_capi,
  e2e_aws,
  e2e_gcp,
];

local e2e_trigger = {
  trigger: {
    target: {
      include: ['e2e'],
    },
  },
};

local e2e_pipeline = Pipeline('e2e', e2e_steps) + e2e_trigger;

// Conformance pipeline.

local conformance_aws = Step("e2e-aws", depends_on=[e2e_capi], environment=creds_env_vars+{SONOBUOY_MODE: "certified-conformance"});
local conformance_azure = Step("e2e-azure", depends_on=[e2e_capi], environment=creds_env_vars+{SONOBUOY_MODE: "certified-conformance"});
local conformance_gcp = Step("e2e-gcp", depends_on=[e2e_capi], environment=creds_env_vars+{SONOBUOY_MODE: "certified-conformance"});

local push_edge = {
  name: 'push-edge',
  image: 'autonomy/build-container:latest',
  pull: 'always',
  environment: {
    DOCKER_USERNAME: { from_secret: 'docker_username' },
    DOCKER_PASSWORD: { from_secret: 'docker_password' },
  },
  commands: ['make push-edge'],
  volumes: volumes.ForStep(),
  when: {
    cron: [
      'nightly',
    ],
  },
  depends_on: [conformance_aws.name, conformance_gcp.name],
};

local conformance_steps = default_steps + [
  e2e_capi,
  conformance_aws,
  conformance_gcp,
  push_edge,
];

local conformance_trigger = {
  trigger: {
    target: {
      include: ['conformance'],
    },
  },
};

local conformance_pipeline = Pipeline('conformance', conformance_steps) + conformance_trigger;

// Nightly pipeline.

local nightly_trigger = {
  trigger: {
    cron: {
      include: ['nightly'],
    },
  },
};

local nightly_pipeline = Pipeline('nightly', conformance_steps) + nightly_trigger;

// Release pipeline.

local iso = Step('iso', depends_on=[e2e_docker, e2e_firecracker]);
local boot = Step('boot', depends_on=[e2e_docker, e2e_firecracker]);

// TODO(andrewrynhard): We should run E2E tests on a release.
local release = {
  name: 'release',
  image: 'plugins/github-release',
  settings: {
    api_key: { from_secret: 'github_token' },
    draft: true,
    files: [
      '_out/aws.tar.gz',
      '_out/azure.tar.gz',
      '_out/boot.tar.gz',
      '_out/container.tar',
      '_out/digital-ocean.tar.gz',
      '_out/gcp.tar.gz',
      '_out/initramfs.xz',
      '_out/installer.tar',
      '_out/osctl-darwin-amd64',
      '_out/osctl-linux-amd64',
      '_out/vmware.ova',
      '_out/vmlinux',
      '_out/vmlinuz',
    ],
    checksum: ['sha256', 'sha512'],
  },
  when: {
    event: ['tag'],
  },
  depends_on: [kernel.name, iso.name, boot.name, image_gcp.name, image_azure.name, image_aws.name, push.name]
};

local release_steps = default_steps + [
  iso,
  boot,
  release,
];

local release_trigger = {
  trigger: {
    event: [
      'tag',
    ],
  },
};

local release_pipeline = Pipeline('release', release_steps) + release_trigger;

// Notify pipeline.

local notify = {
  name: 'slack',
  image: 'plugins/slack',
  settings:
    {
      webhook: { from_secret: 'slack_webhook' },
      channel: 'proj-talos-maintainers',
      link_names: true,
      template: '{{#if build.pull }}
*{{#success build.status}}✓ Success{{else}}✕ Fail{{/success}}*: {{ repo.owner }}/{{ repo.name }} - <https://github.com/{{ repo.owner }}/{{ repo.name }}/pull/{{ build.pull }}|Pull Request #{{ build.pull }}>
{{else}}
*{{#success build.status}}✓ Success{{else}}✕ Fail{{/success}}: {{ repo.owner }}/{{ repo.name }} - Build #{{ build.number }}* (type: `{{ build.event }}`)
{{/if}}
Commit: <https://github.com/{{ repo.owner }}/{{ repo.name }}/commit/{{ build.commit }}|{{ truncate build.commit 8 }}>
Branch: <https://github.com/{{ repo.owner }}/{{ repo.name }}/commits/{{ build.branch }}|{{ build.branch }}>
Author: {{ build.author }}
<{{ build.link }}|Visit build page>'
    },
};

local notify_steps = [notify];

local notify_trigger = {
  trigger: {
    status: ['success', 'failure'],
  },
};

local notify_depends_on = {
  depends_on: [
    default_pipeline.name,
    e2e_pipeline.name,
    conformance_pipeline.name,
    nightly_pipeline.name,
    release_pipeline.name,
  ],
};

local notify_pipeline = Pipeline('notify', notify_steps, [default_pipeline, e2e_pipeline, conformance_pipeline, nightly_pipeline, release_pipeline], false, true) + notify_trigger;

// Final configuration file definition.

[
  secret,
  default_pipeline,
  e2e_pipeline,
  conformance_pipeline,
  nightly_pipeline,
  release_pipeline,
  notify_pipeline,
]
