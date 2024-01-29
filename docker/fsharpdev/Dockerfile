FROM ubuntu:20.04

ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update \
 && apt-get -y install wget gnupg apt-transport-https software-properties-common \
 && wget -q https://packages.microsoft.com/config/ubuntu/20.04/packages-microsoft-prod.deb -O packages-microsoft-prod.deb  \
 && dpkg -i packages-microsoft-prod.deb \
 && apt-get update \
 && add-apt-repository universe \
 && apt-get update \
 && apt-get -y install dotnet-sdk-5.0 \
 && rm -rf /var/lib/apt/lists/*

# Create a warmup project to populate package cache
RUN mkdir warmup \
 && cd warmup \
 && dotnet new xunit -lang "F#" \
 # Add required packages here that should end up in the cache:
 && dotnet add package Microsoft.NET.Test.Sdk --version 16.11.0 \
 && dotnet add package MSTest.TestAdapter --version 2.1.2 \
 && dotnet add package MSTest.TestFramework --version 2.1.2 \
 && dotnet add package FsCheck --version 2.16.0 \
 && dotnet add package Unquote --version 6.1.0 \
 && dotnet restore \
 && cd - \
 && rm -rf warmup /tmp/NuGetScratch
