// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gcp_test

import "testing"

func TestEmpty(t *testing.T) {
	// added for accurate coverage estimation
	//
	// please remove it once any unit-test is added
	// for this package
}

// TODO use this mock data for tests
/*
brad@instance-1:~$ curl -H "Metadata-Flavor: Google" 'http://169.254.169.254/computeMetadata/v1/instance/?recursive=true' | jq '.'
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100  1825  100  1825    0     0   285k      0 --:--:-- --:--:-- --:--:--  297k
{
  "attributes": {
    "ssh-keys": ""
  },
  "cpuPlatform": "Intel Haswell",
  "description": "",
  "disks": [
    {
      "deviceName": "instance-1",
      "index": 0,
      "mode": "READ_WRITE",
      "type": "PERSISTENT"
    }
  ],
  "guestAttributes": {},
  "hostname": "instance-1.c.talos-testbed.internal",
  "id": 7413733082653629000,
  "image": "projects/debian-cloud/global/images/debian-9-stretch-v20190916",
  "licenses": [
    {
      "id": "1000205"
    }
  ],
  "machineType": "projects/381598048798/machineTypes/f1-micro",
  "maintenanceEvent": "NONE",
  "name": "instance-1",
  "networkInterfaces": [
    {
      "accessConfigs": [
        {
          "externalIp": "35.239.151.17",
          "type": "ONE_TO_ONE_NAT"
        }
      ],
      "dnsServers": [
        "169.254.169.254"
      ],
      "forwardedIps": [],
      "gateway": "10.128.0.1",
      "ip": "10.128.15.237",
      "ipAliases": [],
      "mac": "42:01:0a:80:0f:ed",
      "mtu": 1460,
      "network": "projects/381598048798/networks/default",
      "subnetmask": "255.255.240.0",
      "targetInstanceIps": []
    }
  ],
  "preempted": "FALSE",
  "remainingCpuTime": -1,
  "scheduling": {
    "automaticRestart": "TRUE",
    "onHostMaintenance": "MIGRATE",
    "preemptible": "FALSE"
  },
  "serviceAccounts": {},
  "tags": [],
  "virtualClock": {
    "driftToken": "0"
  },
  "zone": "projects/381598048798/zones/us-central1-f"
}
*/
