#!/bin/bash
set -eo pipefail

echo "INFO: E2E_TEST_SUITE is '$E2E_TEST_SUITE'"

TARGET_BINARY=""

if [ "$E2E_TEST_SUITE" == "secretmanager" ]; then
  TARGET_BINARY="/bin/e2e_sm.test"
elif [ "$E2E_TEST_SUITE" == "parametermanager" ]; then
  TARGET_BINARY="/bin/e2e_pm.test"
elif [ -z "$E2E_TEST_SUITE" ] || [ "$E2E_TEST_SUITE" == "all" ]; then
  TARGET_BINARY="/bin/e2e_all.test"
  echo "INFO: E2E_TEST_SUITE is empty or 'all', selecting all tests binary."
else
  echo "ERROR: Unknown E2E_TEST_SUITE specified: '$E2E_TEST_SUITE'."
  exit 1
fi

if [ ! -f "$TARGET_BINARY" ]; then
    echo "ERROR: Test binary for suite '$E2E_TEST_SUITE' not found at '$TARGET_BINARY'."
    exit 1
fi

echo "INFO: Executing E2E test binary: $TARGET_BINARY"
# The compiled test binary accepts standard Go test flags.
# -test.v: verbose output
# -test.count=1: disable test caching
# -test.parallel=1: disable parallel execution of tests within this suite (suites run in parallel via K8s Jobs)
# -test.timeout: overall timeout for the test suite
exec "$TARGET_BINARY" -test.v -test.count=1 -test.parallel=1 -test.timeout=90m