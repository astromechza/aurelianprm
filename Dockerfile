FROM gcr.io/distroless/static-debian12:nonroot
COPY aurelianprm /usr/local/bin/aurelianprm
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["aurelianprm", "--db", "/data/aurelianprm.db"]
