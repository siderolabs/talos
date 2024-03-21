// This file contains the logic for building our CI for Drone. The idea here is
// that we create a pipeline for all of the major tasks we need to perform
// (e.g. builds, E2E testing, conformance testing, releases). Each pipeline
// after the default builds on a previous pipeline.
// Generate with `drone jsonnet --source ./hack/drone.jsonnet --stream --format`
// Sign with `drone sign siderolabs/talos --save`

local build_container = 'autonomy/build-container:latest';
local downstream_image = 'ghcr.io/siderolabs/drone-downstream:v1.2.0-33-g2306176';
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
        path: '/var/ci-docker',
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
        medium: 'memory',
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

// Step standardizes the creation of build steps. The name of the step is used
// as the target when building the make command unless with_make is set to false. For example, if name equals
// "test", the resulting step command will be "make test". This is done to
// encourage alignment between this file and the Makefile, and gives us a
// standardized structure that should make things easier to reason about if we
// know that each step is essentially a Makefile target.
local Step(name, image='', target='', privileged=false, depends_on=[], environment={}, extra_volumes=[], with_make=true, entrypoint=null, extra_commands=[], resources={}, when={}) = {
  local make = if target == '' then std.format('make %s', name) else std.format('make %s', target),

  local commands = if with_make then [make] + extra_commands else extra_commands,

  local common_env_vars = {
    PLATFORM: 'linux/amd64,linux/arm64',
  },

  name: name,
  image: if image == '' then build_container else image,
  pull: 'always',
  entrypoint: entrypoint,
  commands: commands,
  resources: resources,
  privileged: privileged,
  environment: common_env_vars + environment,
  volumes: volumes.ForStep() + extra_volumes,
  depends_on: [x.name for x in depends_on],
  when: when,
};

// TriggerDownstream is a helper function for creating a step that triggers a
// downstream pipeline. It is used to standardize the creation of these steps.
local TriggerDownstream(name, target, repositories, image='', params=[], depends_on=[], when={}) = {
  name: name,
  image: if image == '' then downstream_image else image,
  settings: {
    server: 'https://ci.dev.talos-systems.io/',
    token: {
      from_secret: 'drone_token',
    },
    repositories: repositories,
    last_successful: true,
    block: true,
    params: params,
    deploy: target,
  },
  depends_on: [x.name for x in depends_on],
  when: when,
};

// This provides the docker service.
local docker = {
  name: 'docker',
  image: 'docker:25.0.2-dind',
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

// Pipeline is a way to standardize the creation of pipelines. It supports
// using and existing pipeline as a base.
local Pipeline(name, steps=[], depends_on=[], with_docker=true, disable_clone=false, type='kubernetes') = {
  kind: 'pipeline',
  type: type,
  name: name,
  [if type == 'digitalocean' then 'token']: {
    from_secret: 'digitalocean_token',
  },
  // See https://slugs.do-api.dev/.
  [if type == 'digitalocean' then 'server']: {
    image: 'ubuntu-20-04-x64',
    size: 'c-32',
    region: 'nyc3',
  },
  [if with_docker then 'services']: [docker],
  [if disable_clone then 'clone']: {
    disable: true,
  },
  steps: steps,
  volumes: volumes.ForPipeline(),
  depends_on: [x.name for x in depends_on],
};

local creds_env_vars = {
  // AWS creds
  AWS_ACCESS_KEY_ID: { from_secret: 'aws_access_key_id' },
  AWS_SECRET_ACCESS_KEY: { from_secret: 'aws_secret_access_key' },
  AWS_SVC_ACCT: { from_secret: 'aws_svc_acct' },
  // Azure creds
  AZURE_SUBSCRIPTION_ID: { from_secret: 'azure_subscription_id' },
  AZURE_STORAGE_ACCOUNT: { from_secret: 'az_storage_account' },
  AZURE_CLIENT_ID: { from_secret: 'az_storage_user' },  // using old variable name not to break existing release branch pipelines
  AZURE_CLIENT_SECRET: { from_secret: 'az_storage_pass' },  // using old variable name not to break existing release branch pipelines
  AZURE_TENANT_ID: { from_secret: 'azure_tenant_id' },
  // TODO(andrewrynhard): Rename this to the GCP convention.
  GCE_SVC_ACCT: { from_secret: 'gce_svc_acct' },
  PACKET_AUTH_TOKEN: { from_secret: 'packet_auth_token' },
  GITHUB_TOKEN: { from_secret: 'ghcr_token' },  // Use GitHub API token to avoid rate limiting on CAPI -> GitHub calls.
};

// Sets up the CI environment
local setup_ci = Step(
  'setup-ci',
  with_make=false,
  privileged=true,
  extra_commands=[
    'setup-ci',
  ],
  environment={
    BUILDKIT_FLAVOR: 'cross',
  },
);

// Default pipeline.
local external_artifacts = Step('external-artifacts', depends_on=[setup_ci]);
local generate = Step('generate', target='generate docs', depends_on=[setup_ci]);
local uki_certs = Step('uki-certs', depends_on=[generate], environment={ PLATFORM: 'linux/amd64' });
local check_dirty = Step('check-dirty', depends_on=[generate, external_artifacts, uki_certs]);
local build = Step('build', target='talosctl-all kernel sd-boot sd-stub initramfs installer imager talos _out/integration-test-linux-amd64', depends_on=[check_dirty], environment={ IMAGE_REGISTRY: local_registry, PUSH: true });
local lint = Step('lint', depends_on=[build]);
local talosctl_cni_bundle = Step('talosctl-cni-bundle', depends_on=[build, lint]);
local iso = Step('iso', target='iso secureboot-iso', depends_on=[build], environment={ IMAGE_REGISTRY: local_registry });
local images_essential = Step('images-essential', target='images-essential', depends_on=[iso], environment={ IMAGE_REGISTRY: local_registry });
local unit_tests = Step('unit-tests', target='unit-tests unit-tests-race', depends_on=[build, lint]);
local e2e_docker = Step('e2e-docker-short', depends_on=[build, unit_tests], target='e2e-docker', environment={ SHORT_INTEGRATION_TEST: 'yes', IMAGE_REGISTRY: local_registry });
local e2e_qemu = Step('e2e-qemu-short', privileged=true, target='e2e-qemu', depends_on=[build, unit_tests, talosctl_cni_bundle], environment={ IMAGE_REGISTRY: local_registry, SHORT_INTEGRATION_TEST: 'yes' }, when={ event: ['pull_request'] });
local e2e_iso = Step('e2e-iso', privileged=true, target='e2e-iso', depends_on=[build, unit_tests, iso, talosctl_cni_bundle], when={ event: ['pull_request'] }, environment={ IMAGE_REGISTRY: local_registry });
local release_notes = Step('release-notes', depends_on=[e2e_docker, e2e_qemu]);

local coverage = Step(
  'coverage',
  with_make=false,
  environment={
    CODECOV_TOKEN: { from_secret: 'codecov_token' },
  },
  extra_commands=[
    '/usr/local/bin/codecov -f _out/coverage.txt -X fix',
  ],
  when={
    event: ['pull_request'],
  },
  depends_on=[unit_tests],
);

local push = Step(
  'push',
  environment={
    GHCR_USERNAME: { from_secret: 'ghcr_username' },
    GHCR_PASSWORD: { from_secret: 'ghcr_token' },
    PLATFORM: 'linux/amd64,linux/arm64',
  },
  depends_on=[
    e2e_docker,
    e2e_qemu,
  ],
  when={
    event: {
      exclude: [
        'pull_request',
        'promote',
        'cron',
      ],
    },
  }
);

local push_latest = Step(
  'push-latest',
  environment={
    GHCR_USERNAME: { from_secret: 'ghcr_username' },
    GHCR_PASSWORD: { from_secret: 'ghcr_token' },
    PLATFORM: 'linux/amd64,linux/arm64',
  },
  depends_on=[
    push,
  ],
  when={
    branch: [
      'main',
    ],
    event: [
      'push',
    ],
  },
);

local save_artifacts = Step(
  'save-artifacts',
  with_make=false,
  environment=creds_env_vars,
  depends_on=[
    build,
    images_essential,
    iso,
    talosctl_cni_bundle,
  ],
  extra_commands=[
    'az login --service-principal -u "$${AZURE_CLIENT_ID}" -p "$${AZURE_CLIENT_SECRET}" --tenant "$${AZURE_TENANT_ID}"',
    'az storage container create --metadata ci=true -n ${CI_COMMIT_SHA}${DRONE_TAG//./-}',
    'az storage blob upload-batch --overwrite -s _out -d  ${CI_COMMIT_SHA}${DRONE_TAG//./-}',
  ]
);

local load_artifacts = Step(
  'load-artifacts',
  with_make=false,
  environment=creds_env_vars,
  depends_on=[
    setup_ci,
  ],
  extra_commands=[
    'az login --service-principal -u "$${AZURE_CLIENT_ID}" -p "$${AZURE_CLIENT_SECRET}" --tenant "$${AZURE_TENANT_ID}"',
    'mkdir -p _out/',
    'az storage blob download-batch --overwrite -d _out -s ${CI_COMMIT_SHA}${DRONE_TAG//./-}',
    'chmod +x _out/clusterctl _out/integration-test-linux-amd64 _out/module-sig-verify-linux-amd64 _out/kubectl _out/kubestr _out/helm _out/cilium _out/talosctl*',
  ]
);

// ExtensionsStep is a helper function for creating a step that builds the
// extensions and runs the e2e tests for the extensions which can be set to be skipped.
local ExtensionsStep(with_e2e=true) =
  // builds the extensions
  local extensions_build = TriggerDownstream(
    'extensions-build',
    'e2e-talos',
    ['siderolabs/extensions@main'],
    params=[
      std.format('REGISTRY=%s', local_registry),
      'PLATFORM=linux/amd64',
      'BUCKET_PATH=${CI_COMMIT_SHA}${DRONE_TAG//./-}',
      '_out/talos-metadata',  // params passed from file with KEY=VALUE format
    ],
    depends_on=[load_artifacts],
  );

  // here we need to wait for the extensions build to finish
  local extensions_artifacts = Step(
    'extensions-artifacts',
    with_make=false,
    environment=creds_env_vars,
    depends_on=[
      setup_ci,
      extensions_build,
    ],
    extra_commands=[
      'az login --service-principal -u "$${AZURE_CLIENT_ID}" -p "$${AZURE_CLIENT_SECRET}" --tenant "$${AZURE_TENANT_ID}"',
      'az storage blob download -f _out/extensions-metadata -n extensions-metadata -c ${CI_COMMIT_SHA}${DRONE_TAG//./-}',
    ]
  );

  // generates the extension list patch manifest
  local extensions_patch_manifest = Step(
    'extensions-patch-manifest',
    with_make=false,
    environment=creds_env_vars,
    depends_on=[
      extensions_artifacts,
    ],
    extra_commands=[
      // create a patch file to pass to the downstream build
      // ignore nvidia extensions, testing nvidia extensions needs a machine with nvidia graphics card
      'jq -R < _out/extensions-metadata | jq -s -f hack/test/extensions/extension-patch-filter.jq | yq eval ".[] | split_doc" -P > _out/extensions-patch.yaml',
      'cat _out/extensions-patch.yaml',
    ]
  );

  local e2e_extensions = Step('e2e-extensions', target='e2e-qemu', privileged=true, depends_on=[extensions_patch_manifest], environment={
    QEMU_MEMORY_WORKERS: '4096',
    WITH_CONFIG_PATCH_WORKER: '@_out/extensions-patch.yaml',
    IMAGE_REGISTRY: local_registry,
    QEMU_EXTRA_DISKS: '3',
    SHORT_INTEGRATION_TEST: 'yes',
    EXTRA_TEST_ARGS: '-talos.extensions.qemu',
  });

  local step_targets = [extensions_build, extensions_artifacts, extensions_patch_manifest, e2e_extensions];
  local targets = if with_e2e then step_targets else [extensions_build, extensions_artifacts];

  targets;


local default_steps = [
  setup_ci,
  external_artifacts,
  generate,
  uki_certs,
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
    branch: {
      exclude: [
        'renovate/*',
        'dependabot/*',
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

local integration_qemu = Step('e2e-qemu', privileged=true, depends_on=[load_artifacts], environment={ IMAGE_REGISTRY: local_registry });

local integration_extensions = [step for step in ExtensionsStep()];

local integration_qemu_trusted_boot = Step('e2e-qemu-trusted-boot', target='e2e-qemu', privileged=true, depends_on=[load_artifacts], environment={
  IMAGE_REGISTRY: local_registry,
  VIA_MAINTENANCE_MODE: 'true',
  WITH_TRUSTED_BOOT_ISO: 'true',
  EXTRA_TEST_ARGS: '-talos.trustedboot',
});

local integration_factory_16_iso = Step('factory-1.6-iso', target='e2e-image-factory', privileged=true, depends_on=[load_artifacts], environment={
  FACTORY_BOOT_METHOD: 'iso',
  FACTORY_VERSION: 'v1.6.0',
  FACTORY_SCHEMATIC: '376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba',
  KUBERNETES_VERSION: '1.29.0',
  FACTORY_UPGRADE: 'true',
  FACTORY_UPGRADE_SCHEMATIC: 'cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f',
  FACTORY_UPGRADE_VERSION: 'v1.6.1',
});

local integration_factory_16_image = Step('factory-1.6-image', depends_on=[integration_factory_16_iso], target='e2e-image-factory', privileged=true, environment={
  FACTORY_BOOT_METHOD: 'disk-image',
  FACTORY_VERSION: 'v1.6.0',
  FACTORY_SCHEMATIC: '376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba',
  KUBERNETES_VERSION: '1.29.0',
  FACTORY_UPGRADE: 'true',
  FACTORY_UPGRADE_SCHEMATIC: 'cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f',
  FACTORY_UPGRADE_VERSION: 'v1.6.1',
});

local integration_factory_16_pxe = Step('factory-1.6-pxe', depends_on=[integration_factory_16_image], target='e2e-image-factory', privileged=true, environment={
  FACTORY_BOOT_METHOD: 'pxe',
  FACTORY_VERSION: 'v1.6.1',
  FACTORY_SCHEMATIC: '376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba',
  KUBERNETES_VERSION: '1.29.0',
});

local integration_factory_16_secureboot = Step('factory-1.6-secureboot', depends_on=[integration_factory_16_pxe], target='e2e-image-factory', privileged=true, environment={
  FACTORY_BOOT_METHOD: 'secureboot-iso',
  FACTORY_VERSION: 'v1.6.0',
  FACTORY_SCHEMATIC: 'cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f',
  KUBERNETES_VERSION: '1.29.0',
  FACTORY_UPGRADE: 'true',
  FACTORY_UPGRADE_SCHEMATIC: '376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba',
  FACTORY_UPGRADE_VERSION: 'v1.6.1',
});

local integration_factory_15_iso = Step('factory-1.5-iso', depends_on=[integration_factory_16_secureboot], target='e2e-image-factory', privileged=true, environment={
  FACTORY_BOOT_METHOD: 'iso',
  FACTORY_VERSION: 'v1.5.5',
  FACTORY_SCHEMATIC: '376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba',
  KUBERNETES_VERSION: '1.28.5',
  FACTORY_UPGRADE: 'true',
  FACTORY_UPGRADE_SCHEMATIC: 'cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f',
  FACTORY_UPGRADE_VERSION: 'v1.5.5',
});

local integration_factory_13_iso = Step('factory-1.3-iso', depends_on=[integration_factory_15_iso], target='e2e-image-factory', privileged=true, environment={
  FACTORY_BOOT_METHOD: 'iso',
  FACTORY_VERSION: 'v1.3.7',
  FACTORY_SCHEMATIC: '376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba',
  KUBERNETES_VERSION: '1.26.5',
  FACTORY_UPGRADE: 'true',
  FACTORY_UPGRADE_SCHEMATIC: 'cf9b7aab9ed7c365d5384509b4d31c02fdaa06d2b3ac6cc0bc806f28130eff1f',
  FACTORY_UPGRADE_VERSION: 'v1.3.7',
});

local integration_factory_13_image = Step('factory-1.3-image', depends_on=[integration_factory_13_iso], target='e2e-image-factory', privileged=true, environment={
  FACTORY_BOOT_METHOD: 'disk-image',
  FACTORY_VERSION: 'v1.3.7',
  FACTORY_SCHEMATIC: '376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba',
  KUBERNETES_VERSION: '1.26.5',
});


local build_race = Step('build-race', target='initramfs installer', depends_on=[load_artifacts], environment={ IMAGE_REGISTRY: local_registry, PUSH: true, TAG_SUFFIX: '-race', WITH_RACE: '1', PLATFORM: 'linux/amd64' });
local integration_qemu_race = Step('e2e-qemu-race', target='e2e-qemu', privileged=true, depends_on=[build_race], environment={ IMAGE_REGISTRY: local_registry, TAG_SUFFIX: '-race' });

local integration_provision_tests_prepare = Step('provision-tests-prepare', privileged=true, depends_on=[load_artifacts]);
local integration_provision_tests_track_0 = Step('provision-tests-track-0', privileged=true, depends_on=[integration_provision_tests_prepare], environment={ IMAGE_REGISTRY: local_registry });
local integration_provision_tests_track_1 = Step('provision-tests-track-1', privileged=true, depends_on=[integration_provision_tests_prepare], environment={ IMAGE_REGISTRY: local_registry });
local integration_provision_tests_track_2 = Step('provision-tests-track-2', privileged=true, depends_on=[integration_provision_tests_prepare], environment={ IMAGE_REGISTRY: local_registry });

local integration_cilium = Step('e2e-cilium', target='e2e-qemu', privileged=true, depends_on=[load_artifacts], environment={
  SHORT_INTEGRATION_TEST: 'yes',
  WITH_SKIP_BOOT_PHASE_FINISHED_CHECK: 'yes',
  WITH_CUSTOM_CNI: 'cilium',
  WITH_FIREWALL: 'accept',
  QEMU_WORKERS: '2',
  WITH_CONFIG_PATCH: '[{"op": "add", "path": "/cluster/network", "value": {"cni": {"name": "none"}}}]',
  IMAGE_REGISTRY: local_registry,
});
local integration_cilium_strict = Step('e2e-cilium-strict', target='e2e-qemu', privileged=true, depends_on=[integration_cilium], environment={
  SHORT_INTEGRATION_TEST: 'yes',
  WITH_SKIP_BOOT_PHASE_FINISHED_CHECK: 'yes',
  WITH_CUSTOM_CNI: 'cilium',
  WITH_FIREWALL: 'accept',
  QEMU_WORKERS: '2',
  CILIUM_INSTALL_TYPE: 'strict',
  WITH_CONFIG_PATCH: '[{"op": "add", "path": "/cluster/network", "value": {"cni": {"name": "none"}}}, {"op": "add", "path": "/cluster/proxy", "value": {"disabled": true}}]',
  IMAGE_REGISTRY: local_registry,
});
local integration_cilium_strict_kubespan = Step('e2e-cilium-strict-kubespan', target='e2e-qemu', privileged=true, depends_on=[integration_cilium_strict], environment={
  SHORT_INTEGRATION_TEST: 'yes',
  WITH_SKIP_BOOT_PHASE_FINISHED_CHECK: 'yes',
  WITH_CUSTOM_CNI: 'cilium',
  WITH_FIREWALL: 'accept',
  WITH_KUBESPAN: 'true',
  QEMU_WORKERS: '2',
  CILIUM_INSTALL_TYPE: 'strict',
  WITH_CONFIG_PATCH: '[{"op": "add", "path": "/cluster/network", "value": {"cni": {"name": "none"}}}, {"op": "add", "path": "/cluster/proxy", "value": {"disabled": true}}]',
  IMAGE_REGISTRY: local_registry,
});

local integration_firewall = Step('e2e-firewall', target='e2e-qemu', privileged=true, depends_on=[load_artifacts], environment={
  SHORT_INTEGRATION_TEST: 'yes',
  WITH_FIREWALL: 'block',
  REGISTRY: local_registry,
});
local integration_network_chaos = Step('e2e-network-chaos', target='e2e-qemu', privileged=true, depends_on=[integration_firewall], environment={
  SHORT_INTEGRATION_TEST: 'yes',
  WITH_NETWORK_CHAOS: 'true',
  REGISTRY: local_registry,
});
local integration_canal_reset = Step('e2e-canal-reset', target='e2e-qemu', privileged=true, depends_on=[integration_network_chaos], environment={
  INTEGRATION_TEST_RUN: 'TestIntegration/api.ResetSuite/TestResetWithSpec',
  CUSTOM_CNI_URL: 'https://raw.githubusercontent.com/projectcalico/calico/v3.25.0/manifests/canal.yaml',
  REGISTRY: local_registry,
});
local integration_bios_cgroupsv1 = Step('e2e-bios-cgroupsv1', target='e2e-qemu', privileged=true, depends_on=[integration_canal_reset], environment={
  SHORT_INTEGRATION_TEST: 'yes',
  WITH_UEFI: 'false',
  IMAGE_REGISTRY: local_registry,
  WITH_CONFIG_PATCH: '[{"op": "add", "path": "/machine/install/extraKernelArgs/-", "value": "talos.unified_cgroup_hierarchy=0"}]',  // use cgroupsv1
});
local integration_disk_image = Step('e2e-disk-image', target='e2e-qemu', privileged=true, depends_on=[integration_bios_cgroupsv1], environment={
  SHORT_INTEGRATION_TEST: 'yes',
  USE_DISK_IMAGE: 'true',
  VIA_MAINTENANCE_MODE: 'true',
  IMAGE_REGISTRY: local_registry,
  WITH_DISK_ENCRYPTION: 'true',
});
local integration_control_plane_port = Step('e2e-cp-port', target='e2e-qemu', privileged=true, depends_on=[integration_disk_image], environment={
  SHORT_INTEGRATION_TEST: 'yes',
  REGISTRY: local_registry,
  WITH_CONTROL_PLANE_PORT: '443',
});
local integration_no_cluster_discovery = Step('e2e-no-cluster-discovery', target='e2e-qemu', privileged=true, depends_on=[integration_control_plane_port], environment={
  SHORT_INTEGRATION_TEST: 'yes',
  WITH_CLUSTER_DISCOVERY: 'false',
  IMAGE_REGISTRY: local_registry,
});
local integration_kubespan = Step('e2e-kubespan', target='e2e-qemu', privileged=true, depends_on=[integration_no_cluster_discovery], environment={
  SHORT_INTEGRATION_TEST: 'yes',
  WITH_CLUSTER_DISCOVERY: 'true',
  WITH_KUBESPAN: 'true',
  IMAGE_REGISTRY: local_registry,
});
local integration_default_hostname = Step('e2e-default-hostname', target='e2e-qemu', privileged=true, depends_on=[integration_kubespan], environment={
  // regression test: make sure Talos works in maintenance mode when no hostname is set
  SHORT_INTEGRATION_TEST: 'yes',
  IMAGE_REGISTRY: local_registry,
  VIA_MAINTENANCE_MODE: 'true',
  DISABLE_DHCP_HOSTNAME: 'true',
});

local integration_qemu_encrypted_vip = Step('e2e-encrypted-kubespan-vip', target='e2e-qemu', privileged=true, depends_on=[load_artifacts], environment={
  WITH_DISK_ENCRYPTION: 'true',
  WITH_VIRTUAL_IP: 'true',
  WITH_KUBESPAN: 'true',
  IMAGE_REGISTRY: local_registry,
});

local integration_qemu_csi = Step('e2e-csi', target='e2e-qemu', privileged=true, depends_on=[load_artifacts], environment={
  IMAGE_REGISTRY: local_registry,
  SHORT_INTEGRATION_TEST: 'yes',
  QEMU_WORKERS: '3',
  QEMU_CPUS_WORKERS: '4',
  QEMU_MEMORY_WORKERS: '5120',
  QEMU_EXTRA_DISKS: '1',
  QEMU_EXTRA_DISKS_SIZE: '12288',
  WITH_TEST: 'run_csi_tests',
});

local integration_images = Step('images', target='images', depends_on=[load_artifacts], environment={ IMAGE_REGISTRY: local_registry });
local integration_cloud_images = Step('cloud-images', depends_on=[integration_images], environment=creds_env_vars);

local integration_reproducibility_test = Step('reproducibility-test', target='reproducibility-test', depends_on=[load_artifacts], environment={ IMAGE_REGISTRY: local_registry });

local integration_siderolink = Step('e2e-siderolink', target='e2e-qemu', privileged=true, depends_on=[integration_default_hostname], environment={
  SHORT_INTEGRATION_TEST: 'yes',
  WITH_SIDEROLINK_AGENT: 'true',
  VIA_MAINTENANCE_MODE: 'true',
  REGISTRY: local_registry,
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
    integration_qemu.name,
  ],
};

local literal_trigger(names) = {
  trigger: {
    target: {
      include: names,
    },
  },
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
  Pipeline('integration-trusted-boot', default_pipeline_steps + [integration_qemu_trusted_boot]) + integration_trigger(['integration-trusted-boot']),
  Pipeline('integration-provision-0', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_0]) + integration_trigger(['integration-provision', 'integration-provision-0']),
  Pipeline('integration-provision-1', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_1]) + integration_trigger(['integration-provision', 'integration-provision-1']),
  Pipeline('integration-provision-2', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_2]) + integration_trigger(['integration-provision', 'integration-provision-2']),
  Pipeline('integration-misc', default_pipeline_steps + [
    integration_firewall,
    integration_network_chaos,
    integration_canal_reset,
    integration_bios_cgroupsv1,
    integration_disk_image,
    integration_control_plane_port,
    integration_no_cluster_discovery,
    integration_kubespan,
    integration_default_hostname,
    integration_siderolink,
  ]) + integration_trigger(['integration-misc']),
  Pipeline('integration-extensions', default_pipeline_steps + integration_extensions) + integration_trigger(['integration-extensions']),
  Pipeline('integration-cilium', default_pipeline_steps + [integration_cilium, integration_cilium_strict, integration_cilium_strict_kubespan]) + integration_trigger(['integration-cilium']),
  Pipeline('integration-qemu-encrypted-vip', default_pipeline_steps + [integration_qemu_encrypted_vip]) + integration_trigger(['integration-qemu-encrypted-vip']),
  Pipeline('integration-qemu-race', default_pipeline_steps + [build_race, integration_qemu_race]) + integration_trigger(['integration-qemu-race']),
  Pipeline('integration-qemu-csi', default_pipeline_steps + [integration_qemu_csi]) + integration_trigger(['integration-qemu-csi']),
  Pipeline('integration-images', default_pipeline_steps + [integration_images]) + integration_trigger(['integration-images']),
  Pipeline('integration-reproducibility-test', default_pipeline_steps + [integration_reproducibility_test]) + integration_trigger(['integration-reproducibility']),
  Pipeline('integration-cloud-images', default_pipeline_steps + [integration_images, integration_cloud_images]) + literal_trigger(['integration-cloud-images']),
  Pipeline('image-factory', default_pipeline_steps + [
    integration_factory_16_iso,
    integration_factory_16_image,
    integration_factory_16_pxe,
    integration_factory_16_secureboot,
    integration_factory_15_iso,
    integration_factory_13_iso,
    integration_factory_13_image,
  ]) + literal_trigger(['image-factory']),

  // cron pipelines, triggered on schedule events
  Pipeline('cron-integration-qemu', default_pipeline_steps + [integration_qemu, push_edge], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-provision-0', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_0], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-provision-1', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_1], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-provision-2', default_pipeline_steps + [integration_provision_tests_prepare, integration_provision_tests_track_2], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-misc', default_pipeline_steps + [
    integration_firewall,
    integration_network_chaos,
    integration_canal_reset,
    integration_bios_cgroupsv1,
    integration_disk_image,
    integration_control_plane_port,
    integration_no_cluster_discovery,
    integration_kubespan,
    integration_default_hostname,
  ], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-extensions', default_pipeline_steps + integration_extensions, [default_cron_pipeline]) + cron_trigger(['nightly']),
  Pipeline('cron-integration-cilium', default_pipeline_steps + [integration_cilium, integration_cilium_strict, integration_cilium_strict_kubespan], [default_cron_pipeline]) + cron_trigger(['nightly']),
  Pipeline('cron-integration-qemu-encrypted-vip', default_pipeline_steps + [integration_qemu_encrypted_vip], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-integration-qemu-race', default_pipeline_steps + [build_race, integration_qemu_race], [default_cron_pipeline]) + cron_trigger(['nightly']),
  Pipeline('cron-integration-qemu-csi', default_pipeline_steps + [integration_qemu_csi], [default_cron_pipeline]) + cron_trigger(['nightly']),
  Pipeline('cron-integration-images', default_pipeline_steps + [integration_images], [default_cron_pipeline]) + cron_trigger(['nightly']),
  Pipeline('cron-integration-reproducibility-test', default_pipeline_steps + [integration_reproducibility_test], [default_cron_pipeline]) + cron_trigger(['nightly']),
  Pipeline('cron-image-factory',
           default_pipeline_steps + [
             integration_factory_16_iso,
             integration_factory_16_image,
             integration_factory_16_pxe,
             integration_factory_16_secureboot,
             integration_factory_15_iso,
             integration_factory_13_iso,
             integration_factory_13_image,
           ],
           [default_cron_pipeline]) + cron_trigger(['nightly']),
];


// E2E pipeline.
local capi_docker = Step('e2e-docker', depends_on=[load_artifacts], target='e2e-docker', environment={
  IMAGE_REGISTRY: local_registry,
  SHORT_INTEGRATION_TEST: 'yes',
  INTEGRATION_TEST_RUN: 'XXX',
});
local e2e_capi = Step('e2e-capi', depends_on=[capi_docker], environment=creds_env_vars);

local E2EAWS(target) =
  local extensions_artifacts = [step for step in ExtensionsStep(with_e2e=false)];
  local depends_on = if std.startsWith(target, 'nvidia') then [load_artifacts] + extensions_artifacts else [load_artifacts];
  local test_num_nodes = if std.startsWith(target, 'nvidia') then 4 else 6;
  local extra_test_args = if std.startsWith(target, 'nvidia') then '-talos.extensions.nvidia' else '';

  local e2e_aws_prepare = Step(
    'e2e-aws-prepare',
    depends_on=depends_on,
    environment=creds_env_vars {
      IMAGE_REGISTRY: local_registry,
      E2E_AWS_TARGET: target,
    },
    extra_commands=[
      'az login --service-principal -u "$${AZURE_CLIENT_ID}" -p "$${AZURE_CLIENT_SECRET}" --tenant "$${AZURE_TENANT_ID}"',
      'az storage blob upload-batch --overwrite -s _out --pattern "e2e-aws-generated/*" -d "${CI_COMMIT_SHA}${DRONE_TAG//./-}"',
    ]
  );

  local tf_apply = TriggerDownstream(
    'tf-apply',
    'e2e-talos-tf-apply',
    ['siderolabs/contrib@main'],
    params=[
      'BUCKET_PATH=${CI_COMMIT_SHA}${DRONE_TAG//./-}',
      'TYPE=aws',
      'AWS_DEFAULT_REGION=us-east-1',
    ],
    depends_on=[e2e_aws_prepare],
  );

  local e2e_aws_tf_apply_post = Step(
    'e2e-aws-download-artifacts',
    with_make=false,
    environment=creds_env_vars,
    extra_commands=[
      'az login --service-principal -u "$${AZURE_CLIENT_ID}" -p "$${AZURE_CLIENT_SECRET}" --tenant "$${AZURE_TENANT_ID}"',
      'az storage blob download -f _out/e2e-aws-talosconfig -n e2e-aws-talosconfig -c ${CI_COMMIT_SHA}${DRONE_TAG//./-}',
      'az storage blob download -f _out/e2e-aws-kubeconfig -n e2e-aws-kubeconfig -c ${CI_COMMIT_SHA}${DRONE_TAG//./-}',
    ],
    depends_on=[tf_apply],
  );

  local e2e_aws = Step(
    'e2e-aws',
    depends_on=[e2e_aws_tf_apply_post],
    environment=creds_env_vars {
      TEST_NUM_NODES: test_num_nodes,
      EXTRA_TEST_ARGS: extra_test_args,
    }
  );

  local tf_destroy = TriggerDownstream(
    'tf-destroy',
    'e2e-talos-tf-destroy',
    ['siderolabs/contrib@main'],
    params=[
      'BUCKET_PATH=${CI_COMMIT_SHA}${DRONE_TAG//./-}',
      'TYPE=aws',
      'AWS_DEFAULT_REGION=us-east-1',
    ],
    depends_on=[e2e_aws],
    when={
      status: [
        'failure',
        'success',
      ],
    },
  );

  local step_targets = [e2e_aws_prepare, tf_apply, e2e_aws_tf_apply_post, e2e_aws, tf_destroy];
  local targets = if std.startsWith(target, 'nvidia') then extensions_artifacts + step_targets else step_targets;

  targets;

local E2EAzure() =
  local depends_on = [load_artifacts];

  local e2e_azure_prepare = Step(
    'e2e-azure-prepare',
    depends_on=depends_on,
    environment=creds_env_vars {
      IMAGE_REGISTRY: local_registry,
    },
    extra_commands=[
      'az login --service-principal -u "$${AZURE_CLIENT_ID}" -p "$${AZURE_CLIENT_SECRET}" --tenant "$${AZURE_TENANT_ID}"',
      'az storage blob upload-batch --overwrite -s _out --pattern "e2e-azure-generated/*" -d "${CI_COMMIT_SHA}${DRONE_TAG//./-}"',
    ]
  );

  local tf_apply = TriggerDownstream(
    'tf-apply',
    'e2e-talos-tf-apply',
    ['siderolabs/contrib@main'],
    params=[
      'BUCKET_PATH=${CI_COMMIT_SHA}${DRONE_TAG//./-}',
      'TYPE=azure',
    ],
    depends_on=[e2e_azure_prepare],
  );

  local e2e_azure_tf_apply_post = Step(
    'e2e-azure-download-artifacts',
    with_make=false,
    environment=creds_env_vars,
    extra_commands=[
      'az login --service-principal -u "$${AZURE_CLIENT_ID}" -p "$${AZURE_CLIENT_SECRET}" --tenant "$${AZURE_TENANT_ID}"',
      'az storage blob download -f _out/e2e-azure-talosconfig -n e2e-azure-talosconfig -c ${CI_COMMIT_SHA}${DRONE_TAG//./-}',
      'az storage blob download -f _out/e2e-azure-kubeconfig -n e2e-azure-kubeconfig -c ${CI_COMMIT_SHA}${DRONE_TAG//./-}',
    ],
    depends_on=[tf_apply],
  );

  local e2e_azure = Step(
    'e2e-azure',
    depends_on=[e2e_azure_tf_apply_post],
    environment=creds_env_vars {}
  );

  local tf_destroy = TriggerDownstream(
    'tf-destroy',
    'e2e-talos-tf-destroy',
    ['siderolabs/contrib@main'],
    params=[
      'BUCKET_PATH=${CI_COMMIT_SHA}${DRONE_TAG//./-}',
      'TYPE=azure',
    ],
    depends_on=[e2e_azure],
    when={
      status: [
        'failure',
        'success',
      ],
    },
  );

  local targets = [e2e_azure_prepare, tf_apply, e2e_azure_tf_apply_post, e2e_azure, tf_destroy];

  targets;


local e2e_aws = [step for step in E2EAWS('default')];
local e2e_aws_nvidia_oss = [step for step in E2EAWS('nvidia-oss')];
local e2e_azure = [step for step in E2EAzure()];
local e2e_gcp = Step('e2e-gcp', depends_on=[e2e_capi], environment=creds_env_vars);

local e2e_trigger(names) = {
  trigger: {
    target: {
      include: ['e2e'] + names,
    },
  },
};

local e2e_pipelines = [
  // regular pipelines, triggered on promote events
  Pipeline('e2e-aws', default_pipeline_steps + e2e_aws) + e2e_trigger(['e2e-aws']),
  Pipeline('e2e-aws-nvidia-oss', default_pipeline_steps + e2e_aws_nvidia_oss) + e2e_trigger(['e2e-aws-nvidia-oss']),
  Pipeline('e2e-azure', default_pipeline_steps + e2e_azure) + e2e_trigger(['e2e-azure']),
  Pipeline('e2e-gcp', default_pipeline_steps + [capi_docker, e2e_capi, e2e_gcp]) + e2e_trigger(['e2e-gcp']),

  // cron pipelines, triggered on schedule events
  Pipeline('cron-e2e-aws', default_pipeline_steps + e2e_aws, [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-e2e-azure', default_pipeline_steps + e2e_azure, [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
  Pipeline('cron-e2e-gcp', default_pipeline_steps + [capi_docker, e2e_capi, e2e_gcp], [default_cron_pipeline]) + cron_trigger(['thrice-daily', 'nightly']),
];

// Conformance pipeline.

local conformance_k8s_qemu = Step('conformance-k8s-qemu', target='e2e-qemu', privileged=true, depends_on=[load_artifacts], environment={
  QEMU_WORKERS: '2',  // conformance test requires >=2 workers
  QEMU_CPUS: '4',  // conformance test in parallel runs with number of CPUs
  TEST_MODE: 'fast-conformance',
  IMAGE_REGISTRY: local_registry,
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

local cloud_images = Step('cloud-images', depends_on=[e2e_docker, e2e_qemu], environment=creds_env_vars);
local images = Step('images', target='images', depends_on=[iso, images_essential, save_artifacts], environment={ IMAGE_REGISTRY: local_registry });

// TODO(andrewrynhard): We should run E2E tests on a release.
local release = {
  name: 'release',
  image: 'plugins/github-release',
  settings: {
    api_key: { from_secret: 'github_token' },
    draft: true,
    note: '_out/RELEASE_NOTES.md',
    files: [
      '_out/aws-amd64.raw.xz',
      '_out/aws-arm64.raw.xz',
      '_out/azure-amd64.vhd.xz',
      '_out/azure-arm64.vhd.xz',
      '_out/cloud-images.json',
      '_out/digital-ocean-amd64.raw.gz',
      '_out/digital-ocean-arm64.raw.gz',
      '_out/exoscale-amd64.qcow2.xz',
      '_out/exoscale-arm64.qcow2.xz',
      '_out/gcp-amd64.raw.tar.gz',
      '_out/gcp-arm64.raw.tar.gz',
      '_out/hcloud-amd64.raw.xz',
      '_out/hcloud-arm64.raw.xz',
      '_out/initramfs-amd64.xz',
      '_out/initramfs-arm64.xz',
      '_out/metal-amd64.iso',
      '_out/metal-arm64.iso',
      '_out/metal-amd64.raw.xz',
      '_out/metal-arm64.raw.xz',
      '_out/nocloud-amd64.raw.xz',
      '_out/nocloud-arm64.raw.xz',
      '_out/opennebula-amd64.raw.xz',
      '_out/opennebula-arm64.raw.xz',
      '_out/openstack-amd64.raw.xz',
      '_out/openstack-arm64.raw.xz',
      '_out/oracle-amd64.qcow2.xz',
      '_out/oracle-arm64.qcow2.xz',
      '_out/scaleway-amd64.raw.xz',
      '_out/scaleway-arm64.raw.xz',
      '_out/sd-boot-amd64.efi',
      '_out/sd-boot-arm64.efi',
      '_out/sd-stub-amd64.efi',
      '_out/sd-stub-arm64.efi',
      '_out/talosctl-cni-bundle-amd64.tar.gz',
      '_out/talosctl-cni-bundle-arm64.tar.gz',
      '_out/talosctl-darwin-amd64',
      '_out/talosctl-darwin-arm64',
      '_out/talosctl-freebsd-amd64',
      '_out/talosctl-freebsd-arm64',
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
  depends_on: [
    build.name,
    cloud_images.name,
    talosctl_cni_bundle.name,
    images.name,
    iso.name,
    push.name,
    release_notes.name,
  ],
};

local release_steps = default_steps + [
  images,
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
        'refs/tags/pkg/**',
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
    template: '{{#if build.pull }}\n*{{#success build.status}}✓ Success{{else}}✕ Fail{{/success}}*: {{ repo.owner }}/{{ repo.name }} - <https://github.com/{{ repo.owner }}/{{ repo.name }}/pull/{{ build.pull }}|Pull Request #{{ build.pull }}>\n{{else}}\n*{{#success build.status}}✓ Success{{else}}✕ Fail{{/success}}: {{ repo.owner }}/{{ repo.name }} - Build #{{ build.number }}* (type: `{{ build.event }}`)\n{{/if}}\nCommit: <https://github.com/{{ repo.owner }}/{{ repo.name }}/commit/{{ build.commit }}|{{ truncate build.commit 8 }}>\nBranch: <https://github.com/{{ repo.owner }}/{{ repo.name }}/commits/{{ build.branch }}|{{ build.branch }}>\nAuthor: {{ build.author }}\n<{{ build.link }}|Visit build page>',
  },
  when: {
    status: [
      'success',
      'failure',
    ],
  },
};

local notify_steps = [notify];

local notify_trigger = {
  trigger: {
    status: ['success', 'failure'],
    branch: {
      exclude: [
        'renovate/*',
        'dependabot/*',
      ],
    },
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
