# Fallback only — used via the `whisper-docker` compose profile, when no
# native whisper-server is available on the host. Docker Desktop's Linux
# VM has no Metal/GPU passthrough, so this is CPU-only and slower than
# the native binary the api service prefers by default (see
# docker-compose.yml).
#
# NOT live-verified during setup (whisper.cpp wasn't built from source in
# this session — the native binary was already installed). Smoke-test a
# real build + `/inference` call before relying on this path.
FROM ubuntu:22.04 AS build
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential cmake git ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN git clone --depth 1 https://github.com/ggml-org/whisper.cpp.git .
RUN cmake -B build -DGGML_CUDA=OFF -DWHISPER_SDL2=OFF \
    && cmake --build build -j --config Release --target whisper-server

FROM ubuntu:22.04 AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /src/build/bin/whisper-server /usr/local/bin/whisper-server
EXPOSE 8090
ENTRYPOINT ["whisper-server"]
CMD ["-m", "/models/ggml-medium.bin", "--host", "0.0.0.0", "--port", "8090", "--convert"]
