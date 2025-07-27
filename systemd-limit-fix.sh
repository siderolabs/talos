#!/bin/bash
# Permanently increase file descriptor limits via systemd

# Create a new systemd configuration file
cat > /etc/systemd/system.conf.d/limits.conf << EOF
[Manager]
DefaultLimitNOFILE=65536:65536
EOF

# Create directory if it doesn't exist
mkdir -p /etc/systemd/system.conf.d/

# Reload systemd manager configuration
systemctl daemon-reload

echo "Systemd default limits updated. A reboot is recommended for changes to take full effect."
echo "After reboot, verify with: systemctl show --property DefaultLimitNOFILE"
