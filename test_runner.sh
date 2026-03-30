#!/bin/bash
# test_runner.sh: Orchestrator for API testing

set -e

# Source common functions and config
if [ -f "$(dirname "$0")/tests/common.sh" ]; then
    source "$(dirname "$0")/tests/common.sh"
else
    echo "Error: tests/common.sh not found"
    exit 1
fi

log_info "Starting Test Runner..."
check_server

# Initial Cleanup
log_info "Running Initial Cleanup..."
cleanup_all

# Run Tests
TEST_FILES=(
    "tests/01_basics.sh"
    "tests/02_food_mgmt.sh"
    "tests/03_logging.sh"
    "tests/04_stats_reads.sh"
    "tests/05_input_validation.sh"
    "tests/07_account_passkeys.sh"
    "tests/08_account_profile.sh"
)

for TEST_FILE in "${TEST_FILES[@]}"; do
    if [ -f "$TEST_FILE" ]; then
        log_info "Running $TEST_FILE..."
        # Sourcing ensures variables (IDs) are shared
        source "$TEST_FILE"
    else
        log_err "Test file not found: $TEST_FILE"
        exit 1
    fi
done

# Final Cleanup (optional, currently separate script)
# source "tests/06_cleanup.sh"

log_info "🎉 All Tests Passed Successfully!"
