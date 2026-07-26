FROM almir/webhook:2.8.3@sha256:f77cc91c91d1527b48052280af38b50791e842ad8dd291e9b360a0c13c9ca991

# The base image runs as the unprivileged "webhook" user by default. redeploy.sh needs root:
# it writes into the bind-mounted /opt/zaas checkout (owned by mismatched host UIDs) and uses
# the mounted docker socket, which grants host-root-equivalent access regardless of container
# user, so there is no isolation benefit to dropping privileges here.
USER root

# redeploy.sh needs git and the docker CLI, neither of which the base image ships.
RUN apk add --no-cache git docker-cli docker-cli-compose
