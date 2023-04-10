[
    {
        "op": "add",
        "path": "/machine/install/extensions",
        "value": [
            {
                "image": map(select(. | contains("nvidia-container-toolkit") or contains("nvidia-fabricmanager") | not)) | .[]
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
                    "name": "drbd"
                },
                {
                    "name": "gasket"
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
            }
        ]
    }
]
