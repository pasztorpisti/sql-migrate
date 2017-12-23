#!/bin/bash
set -euo pipefail
cd "$( dirname "$0" )"

PORT=5407
DSN="postgres://test:test@localhost:${PORT}/test?sslmode=disable"

CONTAINER=sql-migrate_test_postgres
CONTAINER_CREATED=0

main() {
	cleanup() {
		if [ "${CONTAINER_CREATED}" -eq 1 ]; then
			docker rm -f "${CONTAINER}" &> /dev/null || true
		fi
	}
	trap cleanup EXIT

	create_container
	wait_for_service_ready postgres 60 1 docker exec -it "${CONTAINER}" psql -U test -c '\q'

	export SQL_MIGRATE_BINARY=./postgres-migrate
	go build -o "${SQL_MIGRATE_BINARY}" -tags="custom postgres" github.com/pasztorpisti/sql-migrate
	POSTGRES_DSN="${DSN}" go test -tags="custom postgres integration" -v github.com/pasztorpisti/sql-migrate/...
}

create_container() {
	echo "Creating postgres container ..."

	docker rm -f "${CONTAINER}" &> /dev/null || true
	docker run -d \
		--name "${CONTAINER}" \
		-p ${PORT}:5432 \
		-e POSTGRES_DB=test \
		-e POSTGRES_USER=test \
		-e POSTGRES_PASSWORD=test \
			postgres:10.1-alpine > /dev/null

	CONTAINER_CREATED=1
}

wait_for_service_ready() {
	local NAME=$1
	local ATTEMPTS=$2
	local DELAY_SECS=$3
	shift 3

	echo -n "Waiting for ${NAME} to become ready ..."
	for I in `seq 0 ${ATTEMPTS}`; do
		if [ ${I} -eq ${ATTEMPTS} ]; then
			>&2 echo " Error waiting for ${NAME} to become ready."
			exit 1
		fi
		"$@" &> /dev/null && break
		sleep ${DELAY_SECS}
		echo -n "."
	done
	echo " OK"
}

main
