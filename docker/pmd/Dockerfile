FROM openjdk:8-jre-alpine

RUN  mkdir /usr/local/pmd && cd /usr/local/pmd \
    && wget https://github.com/pmd/pmd/releases/download/pmd_releases%2F6.13.0/pmd-bin-6.13.0.zip \
    && unzip pmd-bin-6.13.0.zip && rm pmd-bin-6.13.0.zip

ENV LANG en_US.utf8
ENTRYPOINT ["sh", "/usr/local/pmd/pmd-bin-6.13.0/bin/run.sh"]