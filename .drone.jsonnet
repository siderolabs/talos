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

  ForStep(): [
    self.dockersock.step,
    self.outerdockersock.step,
    self.docker.step,
    self.kube.step,
    self.dev.step,
    self.tmp.step,
  ],

  ForPipeline(): [
    self.dockersock.pipeline,
    self.outerdockersock.pipeline,
    self.docker.pipeline,
    self.kube.pipeline,
    self.dev.pipeline,
    self.tmp.pipeline,
  ],
};

// This provides the docker service.
local docker = {
  name: 'docker',
  image: 'ghcr.io/smira/docker:20.10-dind-hacked',
  entrypoint: ['dockerd'],
  privileged: true,
  command: [
    '--dns=8.8.8.8',
    '--dns=8.8.4.4',
    '--mtu=1450',
    '--log-level=error',
  ],
  // Set resource requests to ensure that only three builds can be performed at a
  // time. We set it on the service so that we get the scheduling restricitions
  // while still allowing parallel steps.
  resources: {
    requests: {
      cpu: 12000,
      memory: '18GiB',
    },
  },
  volumes: volumes.ForStep(),
};

// Sets up the CI environment
local setup_ci = {
  name: 'setup-ci',
  image: 'autonomy/build-container:latest',
  pull: "always",
  privileged: true,

  commands: [
    'setup-ci',
    'make ./_out/kubectl',
  ],
  environment: {
    "BUILDKIT_FLAVOR": "cross",
  },
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

local generate = Step("generate", target="generate docs", depends_on=[setup_ci]);
local check_dirty = Step("check-dirty", depends_on=[generate]);
local build = Step("build", target="talosctl-linux talosctl-darwin talosctl-windows kernel initramfs installer imager talos", depends_on=[check_dirty], environment={"IMAGE_REGISTRY": local_registry, "PUSH": true});
local lint = Step("lint", depends_on=[build]);
local talosctl_cni_bundle = Step('talosctl-cni-bundle', depends_on=[build, lint]);
local iso = Step("iso", target="iso", depends_on=[build], environment={"IMAGE_REGISTRY": local_registry});
local images_essential = Step("images-essential", target="images-essential", depends_on=[iso], environment={"IMAGE_REGISTRY": local_registry});
local unit_tests = Step("unit-tests", target="unit-tests unit-tests-race", depends_on=[build, lint]);
local e2e_docker = Step("e2e-docker-short", depends_on=[build, unit_tests], target="e2e-docker", environment={"SHORT_INTEGRATION_TEST": "yes", "IMAGE_REGISTRY": local_registry});
local e2e_qemu = Step("e2e-qemu-short", privileged=true, target="e2e-qemu", depends_on=[build, unit_tests, talosctl_cni_bundle], environment={"IMAGE_REGISTRY": local_registry, "SHORT_INTEGRATION_TEST": "yes"}, when={event: ['pull_request']});
local e2e_iso = Step("e2e-iso", privileged=true, target="e2e-iso", depends_on=[build, unit_tests, iso, talosctl_cni_bundle], when={event: ['pull_request']}, environment={"IMAGE_REGISTRY": local_registry});
local release_notes = Step('release-notes', depends_on=[e2e_docker, e2e_qemu]);

local coverage = {
  name: 'coverage',
  image: 'autonomy/build-container:latest',
  pull: 'always',
  environment: {
    CODECOV_TOKEN: { from_secret: 'codecov_token' },
  },
  commands: [
    '/usr/local/bin/codecov -f _out/coverage.txt -X fix'
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
  depends_on: [push.name],
};

local save_artifacts = {
  name: 'save-artifacts',
  image: 'docker.io/d3fk/s3cmd:latest',
  pull: 'always',
  environment: {
    AWS_ACCESS_KEY_ID: { from_secret: 'rook_access_key_id' },
    AWS_SECRET_ACCESS_KEY: { from_secret: 'rook_secret_access_key' },
  },
  commands: [
    's3cmd --host=rook-ceph-rgw-ci-store.rook-ceph.svc --host-bucket=rook-ceph-rgw-ci-store.rook-ceph.svc --no-ssl mb s3://${CI_COMMIT_SHA}${DRONE_TAG//./-}',
    's3cmd --host=rook-ceph-rgw-ci-store.rook-ceph.svc --host-bucket=rook-ceph-rgw-ci-store.rook-ceph.svc --no-ssl expire s3://${CI_COMMIT_SHA}${DRONE_TAG//./-} --expiry-days=3',
    's3cmd --host=rook-ceph-rgw-ci-store.rook-ceph.svc --host-bucket=rook-ceph-rgw-ci-store.rook-ceph.svc --no-ssl --stats sync _out s3://${CI_COMMIT_SHA}${DRONE_TAG//./-}',
  ],
  volumes: volumes.ForStep(),
  depends_on: [build.name, images_essential.name, iso.name, talosctl_cni_bundle.name],
};

local load_artifacts = {
  name: 'load-artifacts',
  image: 'docker.io/d3fk/s3cmd:latest',
  pull: 'always',
  environment: {
    AWS_ACCESS_KEY_ID: { from_secret: 'rook_access_key_id' },
    AWS_SECRET_ACCESS_KEY: { from_secret: 'rook_secret_access_key' },
  },
  commands: [
    's3cmd --host=rook-ceph-rgw-ci-store.rook-ceph.svc --host-bucket=rook-ceph-rgw-ci-store.rook-ceph.svc --no-ssl --stats sync s3://${CI_COMMIT_SHA}${DRONE_TAG//./-} .',
  ],
  volumes: volumes.ForStep(),
  depends_on: [setup_ci.name],
};

local default_steps = [
  setup_ci,
  generate,
  check_dirty,
  build,
  lint,
  talosctl_cni_bundle,
  iso,
  images_essential,
  unit_tests,
  save_artifacts,
  coverage,
  e2e_iso,
  e2e_qemu,
  e2e_docker,
  release_notes,
  push,
  push_latest,
];

local default_trigger = {
  trigger: {
    event: {
      exclude: [
        'tag',
        'promote',
        'cron',
      ],
    },
  },
};

local cron_trigger(schedules) = {
  trigger: {
    cron: {
      include: schedules,
    },
  },
};

local default_pipeline = Pipeline('default', default_steps) + default_trigger;

local default_cron_pipeline = Pipeline('cron-default', default_steps) + cron_trigger(['thrice-daily', 'nightly']);

// Full integration pipeline.

local default_pipeline_steps = [
  setup_ci,
  load_artifacts,
];

local integration_qemu = Step("e2e-qemu", privileged=true, depends_on=[load_artifacts], environment={"IMAGE_REGISTRY": local_registry});

local build_race = Step("build-race", target="initramfs installer", depends_on=[load_artifacts], environment={"IMAGE_REGISTRY": local_registry, "PUSH": true, "TAG_SUFFIX": "-race", "WITH_RACE": "1", "PLATFORM": "linux/amd64"});
local integration_qemu_race = Step("e2e-qemu-race", target="e2e-qemu", privileged=true, depends_on=[build_race], environment={"IMAGE_REGISTRY": local_registry,  "TAG_SUFFIX": "-race"});

local integration_provision_tests_prepare = Step("provision-tests-prepare", privileged=true, depends_on=[load_artifacts]);
local integration_provision_tests_track_0 = Step("provision-tests-track-0", privileged=true, depends_on=[integration_provision_tests_prepare], environment={"IMAGE_REGISTRY": local_registry});
local integration_provision_tests_track_1 = Step("provision-tests-track-1", privileged=true, depends_on=[integration_provision_tests_prepare], environment={"IMAGE_REGISTRY": local_registry});
local integration_provision_tests_track_2 = Step("provision-tests-track-2", privileged=true, depends_on=[integration_provision_tests_prepare], environment={"IMAGE_REGISTRY": local_registry});

local integration_cilium = Step("e2e-cilium-1.9.10", target="e2e-qemu", privileged=true, depends_on=[load_artifacts], environment={
        "SHORT_INTEGRATION_TEST": "yes",
        "CUSTOM_CNI_URL": "https://raw.githubusercontent.com/cilium/cilium/v1.9.10/install/kubernetes/quick-install.yaml",
        "WITH_CONFIG_PATCH": '[{"op": "replace", "path": "/cluster/network/podSubnets", "value": ["10.0.0.0/8"]}]', # use Pod CIDRs as hardcoded in Cilium's quick-install
        "IMAGE_REGISTRY": local_registry,
});
local integration_uefi = Step("e2e-uefi", target="e2e-qemu", privileged=true, depends_on=[integration_cilium], environment={
        "SHORT_INTEGRATION_TEST": "yes",
        "WITH_UEFI": "true",
        "IMAGE_REGISTRY": local_registry,
});
local integration_disk_image = Step("e2e-disk-image", target="e2e-qemu", privileged=true, depends_on=[integration_uefi], environment={
        "SHORT_INTEGRATION_TEST": "yes",
        "USE_DISK_IMAGE": "true",
        "IMAGE_REGISTRY": local_registry,
        "WITH_DISK_ENCRYPTION": "true",
});
local integration_canal_reset = Step("e2e-canal-disabled-reset", target="e2e-qemu", privileged=true, depends_on=[integration_disk_image], environment={
        "INTEGRATION_TEST_RUN": "TestIntegration/api.ResetSuite/TestResetWithSpec",
        // TODO: re-enable when https://github.com/projectcalico/cni-plugin/issues/1214 is fixed
        // "CUSTOM_CNI_URL": "https://docs.projectcalico.org/manifests/canal.yaml",
        "REGISTRY": local_registry,
});
local integration_no_cluster_discovery = Step("e2e-no-cluster-discovery", target="e2e-qemu", privileged=true, depends_on=[integration_canal_reset], environment={
        "SHORT_INTEGRATION_TEST": "yes",
        "WITH_CLUSTER_DISCOVERY": "false",
        "IMAGE_REGISTRY": local_registry,
});
local integration_kubespan = Step("e2e-kubespan", target="e2e-qemu", privileged=true, depends_on=[integration_no_cluster_discovery], environment={
        "SHORT_INTEGRATION_TEST": "yes",
        "WITH_CLUSTER_DISCOVERY": "false",
        "IMAGE_REGISTRY": local_registry,
});
local integration_qemu_encrypted_vip = Step("e2e-encrypted-vip", target="e2e-qemu", privileged=true, depends_on=[load_artifacts], environment={
        "WITH_DISK_ENCRYPTION": "true",
        "WITH_VIRTUAL_IP": "true",
        "IMAGE_REGISTRY": local_registry,
});

local integration_images = Step("images", target="images", depends_on=[load_artifacts], environment={"IMAGE_REGISTRY": local_registry});
local integration_sbcs = Step("sbcs", target="sbcs", depends_on=[integration_images], environment={"IMAGE_REGISTRY": local_registry});

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
    integration_qemu.name,
  ],
};

local integration_trigger(names) = {
  trigger: {
    target: {
      include: ['integration'] + names,
    },
  },
};

local integration_pipelines = [
  // regular pipelines, triggered on promote events
  Pipeline('integration-qemu', default_pipeline_steps + [integration_qemu, push_edge]) + integration_trigger(['integration-qemu']),
  Pipeline('integration-provision-0', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_0]) + integration_trigger(['integration-provision', 'integration-provision-0']),
  Pipeline('integration-provision-1', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_1]) + integration_trigger(['integration-provision', 'integration-provision-1']),
  Pipeline('integration-provision-2', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_2]) + integration_trigger(['integration-provision', 'integration-provision-2']),
  Pipeline('integration-misc', default_pipeline_steps + [integration_cilium, integration_uefi, integration_disk_image, integration_canal_reset, integration_no_cluster_discovery, integration_kubespan]) + integration_trigger(['integration-misc']),
  Pipeline('integration-qemu-encrypted-vip', default_pipeline_steps + [integration_qemu_encrypted_vip]) + integration_trigger(['integration-qemu-encrypted-vip']),
  Pipeline('integration-qemu-race', default_pipeline_steps + [build_race, integration_qemu_race]) + integration_trigger(['integration-qemu-race']),
  Pipeline('integration-images', default_pipeline_steps + [integration_images, integration_sbcs]) + integration_trigger(['integration-images']),

  // cron pipelines, triggered on schedule events
  Pipeline('cron-integration-qemu', default_pipeline_steps + [integration_qemu, push_edge], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-provision-0', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_0], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-provision-1', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_1], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-provision-2', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_2], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-misc', default_pipeline_steps + [integration_cilium, integration_uefi, integration_disk_image, integration_canal_reset, integration_no_cluster_discovery, integration_kubespan], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-qemu-encrypted-vip', default_pipeline_steps + [integration_qemu_encrypted_vip], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-qemu-race', default_pipeline_steps + [build_race, integration_qemu_race], [default_cron_pipeline]) + cron_trigger(['nightly']),
  Pipeline('cron-integration-images', default_pipeline_steps + [integration_images, integration_sbcs], [default_cron_pipeline]) + cron_trigger(['nightly']),
];


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

local capi_docker = Step("e2e-docker", depends_on=[load_artifacts], target="e2e-docker", environment={
  "IMAGE_REGISTRY": local_registry,
  "SHORT_INTEGRATION_TEST": "yes",
  "INTEGRATION_TEST_RUN": "XXX",
});
local e2e_capi = Step("e2e-capi", depends_on=[capi_docker], environment=creds_env_vars);
local e2e_aws = Step("e2e-aws", depends_on=[e2e_capi], environment=creds_env_vars);
local e2e_azure = Step("e2e-azure", depends_on=[e2e_capi], environment=creds_env_vars);
local e2e_gcp = Step("e2e-gcp", depends_on=[e2e_capi], environment=creds_env_vars);

local e2e_trigger(names) = {
  trigger: {
    target: {
      include: ['e2e'] + names,
    },
  },
};

local e2e_pipelines = [
  // regular pipelines, triggered on promote events
  Pipeline('e2e-aws', default_pipeline_steps + [capi_docker, e2e_capi, e2e_aws]) + e2e_trigger(['e2e-aws']),
  Pipeline('e2e-gcp', default_pipeline_steps + [capi_docker, e2e_capi, e2e_gcp]) + e2e_trigger(['e2e-gcp']),

  // cron pipelines, triggered on schedule events
  Pipeline('cron-e2e-aws', default_pipeline_steps + [capi_docker, e2e_capi, e2e_aws], [default_cron_pipeline]) + cron_trigger(['thrice-daily','nightly']),
  Pipeline('cron-e2e-gcp', default_pipeline_steps + [capi_docker, e2e_capi, e2e_gcp], [default_cron_pipeline]) + cron_trigger(['thrice-daily','nightly']),
];

// Conformance pipeline.

local conformance_k8s_qemu = Step("conformance-k8s-qemu", target="e2e-qemu", privileged=true, depends_on=[load_artifacts], environment={
        "QEMU_WORKERS": "2", // conformance test requires >=2 workers
        "QEMU_CPUS": "4", // conformance test in parallel runs with number of CPUs
        "TEST_MODE": "fast-conformance",
        "IMAGE_REGISTRY": local_registry,
});

local conformance_trigger(names) = {
  trigger: {
    target: {
      include: ['conformance'] + names,
    },
  },
};

local conformance_pipelines = [
  // regular pipelines, triggered on promote events
  Pipeline('conformance-qemu', default_pipeline_steps + [conformance_k8s_qemu]) + conformance_trigger(['conformance-qemu']),

  // cron pipelines, triggered on schedule events
  Pipeline('cron-conformance-qemu', default_pipeline_steps + [conformance_k8s_qemu], [default_cron_pipeline]) + cron_trigger(['nightly']),
];

// Release pipeline.

local cloud_images = Step("cloud-images", depends_on=[e2e_docker, e2e_qemu], environment=creds_env_vars);
local images = Step("images", target="images", depends_on=[iso], environment={"IMAGE_REGISTRY": local_registry});
local sbcs = Step("sbcs", target="sbcs", depends_on=[images], environment={"IMAGE_REGISTRY": local_registry});

// TODO(andrewrynhard): We should run E2E tests on a release.
local release = {
  name: 'release',
  image: 'plugins/github-release',
  settings: {
    api_key: { from_secret: 'github_token' },
    draft: true,
    note: '_out/RELEASE_NOTES.md',
    files: [
      '_out/aws-amd64.tar.gz',
      '_out/aws-arm64.tar.gz',
      '_out/azure-amd64.tar.gz',
      '_out/azure-arm64.tar.gz',
      '_out/cloud-images.json',
      '_out/digital-ocean-amd64.tar.gz',
      '_out/digital-ocean-arm64.tar.gz',
      '_out/gcp-amd64.tar.gz',
      '_out/gcp-arm64.tar.gz',
      '_out/hcloud-amd64.raw.xz',
      '_out/hcloud-arm64.raw.xz',
      '_out/initramfs-amd64.xz',
      '_out/initramfs-arm64.xz',
      '_out/metal-amd64.tar.gz',
      '_out/metal-arm64.tar.gz',
      '_out/metal-rpi_4-arm64.img.xz',
      '_out/metal-rockpi_4-arm64.img.xz',
      '_out/metal-rock64-arm64.img.xz',
      '_out/metal-pine64-arm64.img.xz',
      '_out/metal-bananapi_m64-arm64.img.xz',
      '_out/metal-libretech_all_h3_cc_h5-arm64.img.xz',
      '_out/nocloud-amd64.raw.xz',
      '_out/nocloud-arm64.raw.xz',
      '_out/openstack-amd64.tar.gz',
      '_out/openstack-arm64.tar.gz',
      '_out/scaleway-amd64.raw.xz',
      '_out/scaleway-arm64.raw.xz',
      '_out/talos-amd64.iso',
      '_out/talos-arm64.iso',
      '_out/talosctl-cni-bundle-amd64.tar.gz',
      '_out/talosctl-cni-bundle-arm64.tar.gz',
      '_out/talosctl-darwin-amd64',
      '_out/talosctl-darwin-arm64',
      '_out/talosctl-linux-amd64',
      '_out/talosctl-linux-arm64',
      '_out/talosctl-linux-armv7',
      '_out/talosctl-windows-amd64.exe',
      '_out/upcloud-amd64.raw.xz',
      '_out/upcloud-arm64.raw.xz',
      '_out/vmware-amd64.ova',
      '_out/vmware-arm64.ova',
      '_out/vmlinuz-amd64',
      '_out/vmlinuz-arm64',
      '_out/vultr-amd64.raw.xz',
      '_out/vultr-arm64.raw.xz',
    ],
    checksum: ['sha256', 'sha512'],
  },
  when: {
    event: ['tag'],
  },
  depends_on: [build.name, cloud_images.name, talosctl_cni_bundle.name, images.name, sbcs.name, iso.name, push.name, release_notes.name]
};

local release_steps = default_steps + [
  images,
  sbcs,
  cloud_images,
  release,
];

local release_trigger = {
  trigger: {
    event: [
      'tag',
    ],
    ref: {
      exclude: [
        "refs/tags/pkg/**",
      ],
    },
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

local notify_pipeline = Pipeline('notify', notify_steps, [default_pipeline, release_pipeline] + integration_pipelines + e2e_pipelines + conformance_pipelines, false, true) + notify_trigger;

// Final configuration file definition.

[
  default_pipeline,
  default_cron_pipeline,
  release_pipeline,
] + integration_pipelines + e2e_pipelines + conformance_pipelines + [
  notify_pipeline,
]
