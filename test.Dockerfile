FROM us-docker.pkg.dev/cordialsys/containers/build-base:latest
WORKDIR /data

ENV PROJECT=crosschain
ENV CARGO_TARGET_DIR=/tmp/target

RUN --mount=type=cache,target=/root/go/pkg \
    --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/root/target \
    --mount=type=cache,target=/root/.cargo/registry \
    --mount=type=bind,source=.,target=.,readonly \
    make -C $PROJECT lint

RUN --mount=type=cache,target=/root/go/pkg \
    --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/root/target \
    --mount=type=cache,target=/root/.cargo/registry \
    --mount=type=bind,source=.,target=.,readonly \
    make -C $PROJECT test