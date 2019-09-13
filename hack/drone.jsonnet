// This file contains the logic for building our CI for Drone. The idea here is
// that we create a pipeline for all of the major tasks we need to perform
// (e.g. builds, E2E testing, conformance testing, releases). Each pipeline
// after the default builds on a previous pipeline.
// Generate with `drone jsonnet --source ./hack/drone.jsonnet --stream --format`

local build_container = 'autonomy/build-container:latest';

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
    self.dev.step,
    self.tmp.step,
  ],

  ForPipeline(): [
    self.dockersock.pipeline,
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
    '--mtu=1440',
    '--log-level=error',
  ],
  volumes: volumes.ForStep(),
};

// This step is used only when `drone exec` is executed.
local buildkit = {
  name: 'buildkit',
  image: 'moby/buildkit:v0.6.0',
  privileged: true,
  detach: true,
  commands: ['buildkitd --addr tcp://0.0.0.0:1234 --allow-insecure-entitlement security.insecure'],
  when: {
    event: {
      include: [''],
    },
  },
};

// Step standardizes the creation of build steps. The name of the step is used
// as the target when building the make command. For example, if name equals
// "test", the resulting step command will be "make test". This is done to
// encourage alignment between this file and the Makefile, and gives us a
// standardized structure that should make things easier to reason about if we
// know that each step is essentially a Makefile target.
local Step(name, target='', depends_on=[], environment={}) = {
  local make = if target == '' then std.format('make %s', name) else std.format('make %s', target),
  local common_env_vars = {
    BUILDKIT_HOST: '${BUILDKIT_HOST=tcp://buildkitd.ci.svc:1234}',
    BINDIR: '/usr/local/bin',
  },

  name: name,
  image: build_container,
  pull: "always",
  commands: [make],
  environment: common_env_vars + environment,
  volumes: volumes.ForStep(),
  depends_on: [x.name for x in depends_on],
};

// Pipeline is a way to standardize the creation of pipelines. It supports
// using and existing pipeline as a base.
local Pipeline(name, steps=[], depends_on=[], with_buildkit=false, with_docker=true) = {
  local node = { 'node-role.kubernetes.io/ci': '' },

  kind: 'pipeline',
  name: name,
  node: node,
  services: [
    if with_docker then docker,
    if with_buildkit then buildkit,
  ],
  steps: steps,
  volumes: volumes.ForPipeline(),
  depends_on: [x.name for x in depends_on],
};

// Default pipeline.
local fetchtags = {
  name: 'fetch-tags',
  image: 'docker:git',
  commands: [
    'git fetch --tags',
  ],
};

local machined = Step("machined", depends_on=[fetchtags]);
local osd = Step("osd", depends_on=[fetchtags]);
local trustd = Step("trustd", depends_on=[fetchtags]);
local ntpd = Step("ntpd", depends_on=[fetchtags]);
local networkd = Step("networkd", depends_on=[fetchtags]);
local osctl_linux = Step("osctl-linux", depends_on=[fetchtags]);
local osctl_darwin = Step("osctl-darwin", depends_on=[fetchtags]);
local apid = Step("apid", depends_on=[fetchtags]);
local rootfs =  Step("rootfs", depends_on=[machined, osd, trustd, ntpd, networkd, apid]);
local initramfs = Step("initramfs", depends_on=[rootfs]);
local installer = Step("installer", depends_on=[rootfs]);
local container = Step("container", depends_on=[rootfs]);
local lint = Step("lint");
local protolint = Step("protolint");
local markdownlint = Step("markdownlint");
local image_test = Step("image-test", depends_on=[installer]);
local unit_tests = Step("unit-tests", depends_on=[rootfs]);
local unit_tests_race = Step("unit-tests-race", depends_on=[lint]);
local basic_integration = Step("basic-integration", depends_on=[container, osctl_linux]);

local coverage = {
  name: 'coverage',
  image: 'plugins/codecov',
  settings: {
    token: { from_secret: 'codecov_token' },
    files: ['coverage.txt'],
  },
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
  commands: ['make gitmeta', 'make login', 'make push'],
  volumes: volumes.ForStep(),
  when: {
    event: {
      exclude: [
        'pull_request',
        'promote',
      ],
    },
  },
  depends_on: [basic_integration.name],
};

local default_steps = [
  fetchtags,
  machined,
  osd,
  apid,
  trustd,
  ntpd,
  networkd,
  osctl_linux,
  osctl_darwin,
  rootfs,
  initramfs,
  installer,
  container,
  lint,
  protolint,
  markdownlint,
  image_test,
  unit_tests,
  unit_tests_race,
  coverage,
  basic_integration,
  push,
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
    AZURE_SVC_ACCT: {from_secret: "azure_svc_acct"},
    // TODO(andrewrynhard): Rename this to the GCP convention.
    GCE_SVC_ACCT: {from_secret: "gce_svc_acct"},
    PACKET_AUTH_TOKEN: {from_secret: "packet_auth_token"},
    AWS_SVC_ACCT: {from_secret: "aws_svc_acct"},
};

local image_aws = Step("image-aws", depends_on=[installer]);
local image_azure = Step("image-azure", depends_on=[installer]);
local image_digital_ocean = Step("image-digital-ocean", depends_on=[installer]);
local image_gcp = Step("image-gcp", depends_on=[installer]);
local image_vmware = Step("image-vmware", depends_on=[installer]);
local capi = Step("capi", depends_on=[basic_integration], environment=creds_env_vars);
local push_image_aws = Step("push-image-aws", depends_on=[image_aws], environment=creds_env_vars);
local push_image_azure = Step("push-image-azure", depends_on=[image_azure], environment=creds_env_vars);
local push_image_gcp = Step("push-image-gcp", depends_on=[image_gcp], environment=creds_env_vars);
local e2e_integration_aws = Step("e2e-integration-aws", "e2e-integration", depends_on=[capi, push_image_aws], environment={PLATFORM: "aws"});
local e2e_integration_azure = Step("e2e-integration-azure", "e2e-integration", depends_on=[capi, push_image_azure], environment={PLATFORM: "azure"});
local e2e_integration_gcp = Step("e2e-integration-gcp", "e2e-integration", depends_on=[capi, push_image_gcp], environment={PLATFORM: "gcp"});

local e2e_steps = default_steps + [
  capi,
  image_aws,
  image_azure,
  image_gcp,
  push_image_aws,
  push_image_azure,
  push_image_gcp,
  e2e_integration_aws,
  e2e_integration_azure,
  e2e_integration_gcp,
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

local conformance_aws = Step("conformance-aws", "e2e-integration", depends_on=[capi, push_image_aws], environment={PLATFORM: "aws", CONFORMANCE: "run"});
local conformance_azure = Step("conformance-azure", "e2e-integration", depends_on=[capi, push_image_azure], environment={PLATFORM: "azure", CONFORMANCE: "run"});
local conformance_gcp = Step("conformance-gcp", "e2e-integration", depends_on=[capi, push_image_gcp], environment={PLATFORM: "gcp", CONFORMANCE: "run"});

local conformance_steps = default_steps + [
  capi,
  image_aws,
  image_azure,
  image_gcp,
  push_image_aws,
  push_image_azure,
  push_image_gcp,
  conformance_aws,
  conformance_azure,
  conformance_gcp,
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

local aws_env_vars = {
  AWS_ACCESS_KEY_ID: { from_secret: 'aws_access_key_id' },
  AWS_SECRET_ACCESS_KEY: { from_secret: 'aws_secret_access_key' },
  AWS_DEFAULT_REGION: 'us-west-2',
  AWS_PUBLISH_REGIONS: 'us-west-2,us-east-1,us-east-2,us-west-1,eu-central-1',
};

local ami_trigger = {
  when: {
    event: ['tag'],
  },
};

local kernel = Step('kernel');
local iso = Step('iso', depends_on=[installer]);

// TODO(andrewrynhard): We should run E2E tests on a release.
local release = {
  name: 'release',
  image: 'plugins/github-release',
  settings: {
    api_key: { from_secret: 'github_token' },
    draft: true,
    files: ['build/*'],
    checksum: ['sha256', 'sha512'],
  },
  when: {
    event: ['tag'],
  },
  depends_on: [kernel.name, iso.name, image_gcp.name, image_azure.name, image_aws.name, push.name]
};

local release_steps = default_steps + [
  kernel,
  image_aws,
  image_azure,
  image_digital_ocean,
  image_gcp,
  image_vmware,
  iso,
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

local notify_pipeline = Pipeline('notify', notify_steps, [default_pipeline, e2e_pipeline, conformance_pipeline, nightly_pipeline, release_pipeline], false, false) + notify_trigger;

// Final configuration file definition.

[
  default_pipeline,
  e2e_pipeline,
  conformance_pipeline,
  nightly_pipeline,
  release_pipeline,
  notify_pipeline,
]
