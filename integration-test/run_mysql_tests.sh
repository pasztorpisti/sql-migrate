#!/bin/bash
set -euo pipefail
cd "$( dirname "$0" )"

PORT=5406
DSN="test:test@tcp(localhost:${PORT})/test"

CONTAINER=sql-migrate_test_mysql
CONTAINER_CREATED=0

main() {
	cleanup() {
		if [ "${CONTAINER_CREATED}" -eq 1 ]; then
			docker rm -f "${CONTAINER}" &> /dev/null || true
		fi
	}
	trap cleanup EXIT

	create_container
	wait_for_service_ready mysql 60 1 docker exec -it "${CONTAINER}" mysql -u test -ptest -D test -e exit

	export SQL_MIGRATE_BINARY=./mysql-migrate
	go build -o "${SQL_MIGRATE_BINARY}" -tags="custom mysql" github.com/pasztorpisti/sql-migrate
	MYSQL_DSN="${DSN}" go test -tags="custom mysql integration" -v github.com/pasztorpisti/sql-migrate/...
}

create_container() {
	echo "Creating mysql container ..."

	docker rm -f "${CONTAINER}" &> /dev/null || true
	docker run -d \
		--name "${CONTAINER}" \
		-p ${PORT}:3306 \
		-e MYSQL_DATABASE=test \
		-e MYSQL_USER=test \
		-e MYSQL_PASSWORD=test \
		-e MYSQL_ALLOW_EMPTY_PASSWORD=yes \
			mysql:5.7.20 > /dev/null

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
