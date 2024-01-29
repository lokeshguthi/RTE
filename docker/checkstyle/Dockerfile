FROM openjdk:8-jre-alpine

RUN  mkdir /usr/local/checkstyle && cd /usr/local/checkstyle \
    && wget https://github.com/checkstyle/checkstyle/releases/download/checkstyle-8.19/checkstyle-8.19-all.jar

ENV LANG en_US.utf8
ENTRYPOINT ["java", "-jar", "/usr/local/checkstyle/checkstyle-8.19-all.jar"]