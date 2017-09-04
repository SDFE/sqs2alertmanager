# install alertmanager and run

quick manual local (linux) install of alertmanger for testing ... preferably use the Docker container / command provided in README.md

## steps

mkdir -p /opt/alertmanager/{conf.d,templates,bin,data}/

- download alertmanager tgz
- untar
- move bins into /opt/alertmanager/bin/
- move conf.d/config.yml into /opt/alertmanager/conf.d/
- there is no .tmpl example by default

## dir structure

the dir structure used/tested

```
    /opt/alertmanager/
        ... conf.d/simple.yml
        ... templates/*.tmpl
        ... bin/
```

## run alertmanager

```
$ /opt/alertmanager/bin/alertmanager -config.file=/opt/alertmanager/conf.d/simple.yml -log.level=debug -storage.path=/opt/alertmanager/data/ -web.listen-address ":80"
```

## send an alert

```
curl -X POST -d '[
  {
    "labels": {
      "service": "service-one",
      "severity": "critical"
    },
    "annotations": {
      "msg": "first test message"
    },
    "generatorURL": "http://link.to/runbook"
  }
]' localhost:80/api/v1/alerts
```

Returns
```
{"status":"success"}
```

## check in browser

http://localhost/

