#!/bin/bash
# 06_cleanup.sh: Final Cleanup

if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

cleanup_all
