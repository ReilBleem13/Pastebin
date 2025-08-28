FROM fluentd:v1.18-debian-1

USER root

RUN apt-get update
RUN apt-get install -y build-essential libgeoip-dev libmaxminddb-dev

RUN apt-get update && apt-get install -y netcat-openbsd

RUN gem install excon -v 0.109.0
RUN gem install faraday -v 2.8.1 --no-document
RUN gem install faraday-net_http -v 3.0.2 --no-document
RUN gem install fluent-plugin-elasticsearch --no-document
RUN gem install fluent-plugin-opensearch --no-document
RUN gem install fluent-plugin-beats --no-document
RUN gem install fluent-plugin-docker --no-document

USER fluent