#!/bin/bash
# Increase file descriptor limits for current session

# Set higher limits for current shell
ulimit -n 65536

# Check new limits
echo "Current file descriptor limits:"
ulimit -n

echo "If you're running tests in a specific user context, run this script"
echo "in the same shell where you'll be running the tests."
