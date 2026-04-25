FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY aurelianprm /usr/local/bin/aurelianprm
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["aurelianprm", "--db", "/data/aurelianprm.db"]
