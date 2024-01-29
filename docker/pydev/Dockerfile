FROM ubuntu:20.04

RUN apt-get update \
    && apt-get install -y locales python3-pip  \
    && rm -rf /var/lib/apt/lists/* \
    && localedef -i en_US -c -f UTF-8 -A /usr/share/locale/locale.alias en_US.UTF-8 \
    && pip3 install pytest scipy numpy pandas matplotlib ipytest testbook ipykernel nbconvert
ENV LANG en_US.utf8
