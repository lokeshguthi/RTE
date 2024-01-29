FROM ubuntu:16.04

RUN apt-get update && apt-get install -y locales clang llvm && rm -rf /var/lib/apt/lists/* \
    && localedef -i en_US -c -f UTF-8 -A /usr/share/locale/locale.alias en_US.UTF-8
ENV LANG en_US.utf8
ENV ASAN_SYMBOLIZER_PATH /usr/lib/llvm-3.8/bin/llvm-symbolizer