#!/bin/bash
set -euo pipefail
cd "$( dirname "$0" )"

echo "+----------+"
echo "| postgres |"
echo "+----------+"
./run_postgres_tests.sh

echo
echo "+-------+"
echo "| mysql |"
echo "+-------+"
./run_mysql_tests.sh
