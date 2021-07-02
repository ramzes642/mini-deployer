# Deployment mini-service

### Latest version: 

[download linux binary](/deployer/-/jobs/artifacts/master/raw/deployer?job=compile)

## Installation

TODO

Check for run errors in **journalctl -fu deployer** or in log file

## Run flags:
```shell
Usage of ./deployer:
  -config string
        config file location (default "config.json")
  -listen string
        addr port (default ":7654")
```

### Config hot reload

curl http://localhost:7654/reload

## Config sample:
```json5
{
  "cert": "",
  "key": "",
  "commands": {
    "micro": "cd /var/www/micro && git pull"
  },
  "whitelist": [
    "127.0.0.1",
    "172.17.0.1/24",
    "::1/128"
  ],
  "log": "",
  "disable_autoreload": false,
  "gitlab_token": ""
}
```
* cert/key - path to crt & key pem files to enable https
* commands - paths doing deploing jobs
* whitelist - list of ip/subnets to allow access
* log - path to logfile (if you leave it empty, as described in service file - logs will be in syslog)
* disable_autoreload - disable autoreload feature (use curl localhost:7654/reload to do it manually) 
* gitlab_token - Instead of using whitelist ips you may bypass it using gitlab_token config flag equal to "Secret token" from gitlab webhook configuration
