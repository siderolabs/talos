// This file contains the logic for building our CI for Drone. The idea here is
// that we create a pipeline for all of the major tasks we need to perform
// (e.g. builds, E2E testing, conformance testing, releases). Each pipeline
// after the default builds on a previous pipeline.
// Generate with `drone jsonnet --source ./hack/drone.jsonnet --stream --format`
// Sign with `drone sign talos-systems/talos --save`

local build_container = 'autonomy/build-container:latest';
local local_registry = 'registry.dev.talos-systems.io';

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

  outerdockersock: {
    pipeline: {
      name: 'outerdockersock',
      host: {
        path: '/var/ci-docker'
      },
    },
    step: {
      name: $.outerdockersock.pipeline.name,
      path: '/var/outer-run',
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
      temp: {
        'medium': 'memory',
      },
    },
    step: {
      name: $.tmp.pipeline.name,
      path: '/tmp',
    },
  },

  // cache: {
  //   pipeline: {
  //     name: 'cache',
  //     host: {
  //       path: '/tmp',
  //     },
  //   },
  //   step: {
  //     name: $.cache.pipeline.name,
  //     path: '/tmp/cache',
  //   },
  // },

  ForStep(): [
    self.dockersock.step,
    self.outerdockersock.step,
    self.docker.step,
    self.kube.step,
    self.dev.step,
    self.tmp.step,
    // self.cache.step,
  ],

  ForPipeline(): [
    self.dockersock.pipeline,
    self.outerdockersock.pipeline,
    self.docker.pipeline,
    self.kube.pipeline,
    self.dev.pipeline,
    self.tmp.pipeline,
    // self.cache.pipeline,
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
  // Set resource requests to ensure that only one build can be performed at a
  // time. We set it on the service so that we get the scheduling restricitions
  // while still allowing parallel steps.
  resources: {
    requests: {
      cpu: 24000,
      memory: '48GiB',
    },
  },
  volumes: volumes.ForStep(),
};

// Sets up the CI environment
local setup_ci = {
  name: 'setup-ci',
  image: 'autonomy/build-container:latest',
  privileged: true,

  commands: [
    'sleep 5', // Give docker enough time to start.
    'apk add coreutils',
    'docker buildx create --driver docker-container --platform linux/amd64 --buildkitd-flags "--allow-insecure-entitlement security.insecure" --name talosbuilder --use unix:///var/outer-run/docker.sock',
    'docker buildx create --append --name talosbuilder --platform linux/arm64 --buildkitd-flags "--allow-insecure-entitlement security.insecure" tcp://docker-arm64.ci.svc:2376',
    'docker buildx inspect --bootstrap',
    'make ./_out/sonobuoy',
    'make ./_out/kubectl',
    'git fetch --tags',
  ],
  volumes: volumes.ForStep(),
};

// Step standardizes the creation of build steps. The name of the step is used
// as the target when building the make command. For example, if name equals
// "test", the resulting step command will be "make test". This is done to
// encourage alignment between this file and the Makefile, and gives us a
// standardized structure that should make things easier to reason about if we
// know that each step is essentially a Makefile target.
local Step(name, image='', target='', privileged=false, depends_on=[], environment={}, extra_volumes=[], when={}) = {
  local make = if target == '' then std.format('make %s', name) else std.format('make %s', target),

  local common_env_vars = {
    "PLATFORM": "linux/amd64,linux/arm64",
    // "CI_ARGS": "--cache-to=type=local,dest=/tmp/cache --cache-from=type=local,src=/tmp/cache"
  },

  name: name,
  image: if image == '' then build_container else image,
  pull: "always",
  commands: [make],
  privileged: privileged,
  environment: common_env_vars + environment,
  volumes: volumes.ForStep() + extra_volumes,
  depends_on: [x.name for x in depends_on],
  when: when,
};

// Pipeline is a way to standardize the creation of pipelines. It supports
// using and existing pipeline as a base.
local Pipeline(name, steps=[], depends_on=[], with_docker=true, disable_clone=false, type='kubernetes') = {
  kind: 'pipeline',
  type: type,
  name: name,
  [if type == 'digitalocean' then 'token']: {
    from_secret: 'digitalocean_token'
  },
  // See https://slugs.do-api.dev/.
  [if type == 'digitalocean' then 'server']: {
    image: 'ubuntu-20-04-x64',
    size: 'c-32',
    region: 'nyc3',
  },
  [if with_docker then 'services']: [docker],
  [ if disable_clone then 'clone']: {
    disable: true,
  },
  steps: steps,
  volumes: volumes.ForPipeline(),
  depends_on: [x.name for x in depends_on],
};

// Default pipeline.

local docs = Step("docs", depends_on=[setup_ci]);
local generate = Step("generate", depends_on=[setup_ci]);
local check_dirty = Step("check-dirty", depends_on=[docs, generate]);
local talosctl_linux = Step("talosctl-linux", depends_on=[check_dirty]);
local talosctl_darwin = Step("talosctl-darwin", depends_on=[check_dirty]);
local kernel = Step('kernel', depends_on=[check_dirty]);
local initramfs = Step("initramfs", depends_on=[check_dirty]);
local installer = Step("installer", depends_on=[initramfs], environment={"REGISTRY": local_registry, "PUSH": true});
local talos = Step("talos", depends_on=[initramfs], environment={"REGISTRY": local_registry, "PUSH": true});
local golint = Step("lint-go", depends_on=[check_dirty]);
local markdownlint = Step("lint-markdown", depends_on=[check_dirty]);
local protobuflint = Step("lint-protobuf", depends_on=[check_dirty]);
local image_aws = Step("image-aws", depends_on=[installer], environment={"REGISTRY": local_registry});
local image_azure = Step("image-azure", depends_on=[installer], environment={"REGISTRY": local_registry});
local image_digital_ocean = Step("image-digital-ocean", depends_on=[installer], environment={"REGISTRY": local_registry});
local image_gcp = Step("image-gcp", depends_on=[installer], environment={"REGISTRY": local_registry});
local image_vmware = Step("image-vmware", depends_on=[installer], environment={"REGISTRY": local_registry});
local unit_tests = Step("unit-tests", depends_on=[initramfs]);
local unit_tests_race = Step("unit-tests-race", depends_on=[initramfs]);
local e2e_docker = Step("e2e-docker-short", depends_on=[talos, talosctl_linux, unit_tests, unit_tests_race], target="e2e-docker", environment={"SHORT_INTEGRATION_TEST": "yes", "REGISTRY": local_registry});
local e2e_qemu = Step("e2e-qemu-short", privileged=true, target="e2e-qemu", depends_on=[talosctl_linux, initramfs, kernel, installer, unit_tests, unit_tests_race], environment={"REGISTRY": local_registry, "SHORT_INTEGRATION_TEST": "yes"}, when={event: ['pull_request']});

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
  depends_on: [unit_tests.name, unit_tests_race.name],
};

local push = {
  name: 'push',
  image: 'autonomy/build-container:latest',
  pull: 'always',
  environment: {
    GHCR_USERNAME: { from_secret: 'ghcr_username' },
    GHCR_PASSWORD: { from_secret: 'ghcr_token' },
    PLATFORM: "linux/amd64,linux/arm64",
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
  depends_on: [e2e_docker.name, e2e_qemu.name],
};

local push_latest = {
  name: 'push-latest',
  image: 'autonomy/build-container:latest',
  pull: 'always',
  environment: {
    GHCR_USERNAME: { from_secret: 'ghcr_username' },
    GHCR_PASSWORD: { from_secret: 'ghcr_token' },
    PLATFORM: "linux/amd64,linux/arm64",
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
  depends_on: [e2e_docker.name, e2e_qemu.name],
};

local default_steps = [
  setup_ci,
  docs,
  generate,
  check_dirty,
  talosctl_linux,
  talosctl_darwin,
  kernel,
  initramfs,
  installer,
  talos,
  golint,
  markdownlint,
  protobuflint,
  image_aws,
  image_azure,
  image_digital_ocean,
  image_gcp,
  image_vmware,
  unit_tests,
  unit_tests_race,
  coverage,
  e2e_docker,
  e2e_qemu,
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

// Full integration pipeline.

local integration_qemu = Step("e2e-qemu", privileged=true, depends_on=[initramfs, talosctl_linux, kernel, installer, unit_tests, unit_tests_race], environment={"REGISTRY": local_registry});
local integration_provision_tests_prepare = Step("provision-tests-prepare", privileged=true, depends_on=[initramfs, talosctl_linux, kernel, installer, unit_tests, unit_tests_race, e2e_qemu, e2e_docker]);
local integration_provision_tests_track_0 = Step("provision-tests-track-0", privileged=true, depends_on=[integration_provision_tests_prepare], environment={"REGISTRY": local_registry});
local integration_provision_tests_track_1 = Step("provision-tests-track-1", privileged=true, depends_on=[integration_provision_tests_prepare], environment={"REGISTRY": local_registry});
local integration_provision_tests_track_0_cilium = Step("provision-tests-track-0-cilium", target="provision-tests-track-0", privileged=true, depends_on=[integration_provision_tests_track_0], environment={
        "CUSTOM_CNI_URL": "https://raw.githubusercontent.com/cilium/cilium/v1.8.2/install/kubernetes/quick-install.yaml",
        "REGISTRY": local_registry,
});
local integration_cilium = Step("e2e-cilium-1.8.2", target="e2e-qemu", privileged=true, depends_on=[integration_qemu], environment={
        "SHORT_INTEGRATION_TEST": "yes",
        "CUSTOM_CNI_URL": "https://raw.githubusercontent.com/cilium/cilium/v1.8.2/install/kubernetes/quick-install.yaml",
        "REGISTRY": local_registry,
});
local integration_uefi = Step("e2e-uefi", target="e2e-qemu", privileged=true, depends_on=[integration_cilium], environment={
        "SHORT_INTEGRATION_TEST": "yes",
        "WITH_UEFI": "true",
        "REGISTRY": local_registry,
});
local push_edge = {
  name: 'push-edge',
  image: 'autonomy/build-container:latest',
  pull: 'always',
  environment: {
    GHCR_USERNAME: { from_secret: 'ghcr_username' },
    GHCR_PASSWORD: { from_secret: 'ghcr_token' },
  },
  commands: ['make push-edge'],
  volumes: volumes.ForStep(),
  when: {
    cron: [
      'nightly',
    ],
  },
  depends_on: [
     // we skip track_0 dependency since it's a dependency for the cilium test below.
    integration_provision_tests_track_1.name,
    integration_uefi.name,
    integration_provision_tests_track_0_cilium.name,
  ],
};


local integration_steps = default_steps + [
  integration_qemu,
  integration_provision_tests_prepare,
  integration_provision_tests_track_0,
  integration_provision_tests_track_1,
  integration_provision_tests_track_0_cilium,
  integration_cilium,
  integration_uefi,
  push_edge,
];

local integration_trigger = {
  trigger: {
    target: {
      include: ['integration'],
    },
  },
};

local integration_nightly_trigger = {
  trigger: {
    cron: {
      include: ['nightly'],
    },
  },
};


local integration_pipeline = Pipeline('integration', integration_steps) + integration_trigger;
local integration_nightly_pipeline = Pipeline('integration-nightly', integration_steps) + integration_nightly_trigger;

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

local e2e_capi = Step("e2e-capi", depends_on=[e2e_docker], environment=creds_env_vars);
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

local conformance_steps = default_steps + [
  e2e_capi,
  conformance_aws,
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

local boot = Step('boot', depends_on=[e2e_docker, e2e_qemu]);

local release_notes = Step('release-notes', depends_on=[e2e_docker, e2e_qemu]);

// TODO(andrewrynhard): We should run E2E tests on a release.
local release = {
  name: 'release',
  image: 'plugins/github-release',
  settings: {
    api_key: { from_secret: 'github_token' },
    draft: true,
    note: '_out/RELEASE_NOTES.md',
    files: [
      '_out/aws.tar.gz',
      '_out/azure.tar.gz',
      '_out/boot-amd64.tar.gz',
      '_out/boot-arm64.tar.gz',
      '_out/digital-ocean.tar.gz',
      '_out/gcp.tar.gz',
      '_out/initramfs-amd64.xz',
      '_out/initramfs-arm64.xz',
      '_out/talosctl-darwin-amd64',
      '_out/talosctl-linux-amd64',
      '_out/talosctl-linux-arm64',
      '_out/talosctl-linux-armv7',
      '_out/vmware.ova',
      '_out/vmlinuz-amd64',
      '_out/vmlinuz-arm64',
    ],
    checksum: ['sha256', 'sha512'],
  },
  when: {
    event: ['tag'],
  },
  depends_on: [kernel.name, boot.name, image_gcp.name, image_azure.name, image_aws.name, image_vmware.name, image_digital_ocean.name, push.name, release_notes.name]
};

local release_steps = default_steps + [
  boot,
  release_notes,
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
  settings: {
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
  when: {
    status: [
      'success',
      'failure'
    ],
  },
};

local notify_steps = [notify];

local notify_trigger = {
  trigger: {
    status: ['success', 'failure'],
  },
};

local notify_pipeline = Pipeline('notify', notify_steps, [default_pipeline, e2e_pipeline, integration_pipeline, integration_nightly_pipeline, conformance_pipeline, nightly_pipeline, release_pipeline], false, true) + notify_trigger;

// Final configuration file definition.

[
  default_pipeline,
  integration_pipeline,
  integration_nightly_pipeline,
  e2e_pipeline,
  conformance_pipeline,
  nightly_pipeline,
  release_pipeline,
  notify_pipeline,
]
