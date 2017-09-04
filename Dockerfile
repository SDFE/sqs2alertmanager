FROM prom/alertmanager:latest

RUN mkdir -p /etc/alertmanager/templates/

EXPOSE     9093
VOLUME     [ "/alertmanager", "/etc/alertmanager" ]

WORKDIR    /alertmanager

ENTRYPOINT [ "/bin/alertmanager" ]
CMD        [ "-config.file=/etc/alertmanager/config.yml", \
             "-storage.path=/alertmanager", \
             "-log.level=debug" ]
