# very simple docker image config
FROM golang:1.20-alpine

COPY aaa-synthetic-exporter /bin/aaa-synthetic-exporter
RUN chmod 755 /bin/aaa-synthetic-exporter

# create an empty config file
COPY empty_config.yaml /etc/aaa-synthetic-exporter/config.yaml

EXPOSE      9115
ENTRYPOINT  [ "/bin/aaa-synthetic-exporter" ]
CMD         [ "--config.file=/etc/aaa-synthetic-exporter/config.yaml" ]

# if you want to test then set CMD as follows
#CMD sleep 3650d
