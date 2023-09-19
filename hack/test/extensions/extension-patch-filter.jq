[
    {
        "op": "add",
        "path": "/machine/install/extensions",
        "value": [
            {
                "image": map(select(. | contains("nvidia") or contains("tailscale") or contains("xe-guest-utilities") | not)) | .[]
            }
        ]
    },
    {
        "op": "add",
        "path": "/machine/sysctls",
        "value": {
            "user.max_user_namespaces": "11255"
        }
    },
    {
        "op": "add",
        "path": "/machine/kernel",
        "value": {
            "modules": [
                {
                    "name": "asix"
                },
                {
                    "name": "ax88179_178a"
                },
                {
                    "name": "ax88796b"
                },
                {
                    "name": "binfmt_misc"
                },
                {
                    "name": "btrfs"
                },
                {
                    "name": "cdc_ether"
                },
                {
                    "name": "cdc_mbim"
                },
                {
                    "name": "cdc_ncm"
                },
                {
                    "name": "cdc_subset"
                },
                {
                    "name": "cdc_wdm"
                },
                {
                    "name": "cxgb"
                },
                {
                    "name": "cxgb3"
                },
                {
                    "name": "cxgb4"
                },
                {
                    "name": "cxgb4vf"
                },
                {
                    "name": "drbd"
                },
                {
                    "name": "gasket"
                },
                {
                    "name": "net1080"
                },
                {
                    "name": "option"
                },
                {
                    "name": "qmi_wwan"
                },
                {
                    "name": "r8153_ecm"
                },
                {
                    "name": "thunderbolt"
                },
                {
                    "name": "thunderbolt_net"
                },
                {
                    "name": "usb_wwan"
                },
                {
                    "name": "usbnet"
                },
                {
                    "name": "usbserial"
                },
                {
                    "name": "zaurus"
                },
                {
                    "name": "zfs"
                }
            ]
        }
    },
    {
        "op": "add",
        "path": "/machine/files",
        "value": [
            {
                "path": "/var/etc/nut/upsmon.conf",
                "permissions": 600,
                "op": "create",
                "content": "MONITOR ${upsmonHost} 1 remote ${upsmonPasswd} slave\nSHUTDOWNCMD \"/sbin/poweroff\""
            },
            {
                "path": "/var/etc/tailscale/auth.env",
                "permissions": 600,
                "op": "create",
                "content": ""
            }
        ]
    }
]
