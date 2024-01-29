FROM debian:jessie

RUN apt-get update && apt-get install -y \
  cloc \
  --no-install-recommends

ENV LANG en_US.utf8
ENTRYPOINT ["/usr/bin/cloc"]
