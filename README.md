# Deployment mini-service

This mini web-server is made to deploy your code without yaml-files headache.

If you just need to update your code somewhere after commit - use this tool.
You can just define simple shell-commands to be done after git http-hook is requested.
This mini-service is so simple and secure, it is only about 100 lines long, that you can check security
by yourself.
Daemon is using only standard library with no external dependencies.
It was built using the latest stable go 1.6 release, can do auto-reload of config files and 
serve you in your small CI/CD deployment tasks.

Just define command in config and place a web-hook in your git to roll out your code changes in production.

### Sample usage:

#### Modify config (/etc/micro-deployer.json)
add inside "commands" key a new "micro" hook and a secret:
```json
"commands": {
    "micro": "cd /var/www/micro && git pull"
},
"gitlab_token": "123456"
```
#### Add a gilab webhook:
* Go to Settings -> Webhooks
* Enter URL: http://my.server:7654/micro
* Enter Secret token: 123456
* Put the checkbox on trigger: **Push events**
* Click "Add webhook"

That's it! You are done, you may click test - to check that your command works as expected.

You can use either gitlab_token as a secret token or ip whitelist.

### Install latest version: 

Systemd linux installer:
```
curl -s -o- https://raw.githubusercontent.com/ramzes642/mini-deployer/main/install.sh | sudo bash
```
Now only x86, amd64, arm, arm64 systemd linuxes are supported

### Binary manual install
Download [prebuilt binary archive](https://github.com/ramzes642/mini-deployer/releases)
Extract files from archive, copy files as follows and enable autorun:
```shell
  cp config.sample.json /etc/mini-deployer.json
  cp deployer.service /etc/systemd/system/mini-deployer.service
  cp mini-deployer /usr/bin/mini-deployer
  
  # Enable autostart systemd service
  systemctl enable mini-deployer.service
  systemctl start mini-deployer.service
```
Make sure that's installation is ok in **journalctl -fu deployer** or in log file
Edit config **/etc/mini-deployer.json** as you need

### Configuration sample:
```json5
{
  "cert": "",
  "key": "",
  "commands": {
    "micro": "cd /var/www/micro && git pull"
  },
  "whitelist": [
    "127.0.0.1",
    "::1/128",
    "172.17.0.1/24"
  ],
  "log": "",
  "disable_autoreload": false,
  "gitlab_token": "",
  "timeout": 120
}
```
* cert/key - path to crt & key pem files to enable https
* commands - paths doing deploing jobs
* whitelist - list of ip/subnets to allow access
* log - path to logfile (if you leave it empty, as described in service file - logs will be in syslog)
* disable_autoreload - disable autoreload feature (use curl localhost:7654/reload to do it manually) 
* gitlab_token - Instead of using whitelist ips you may bypass it using gitlab_token config flag equal to "Secret token" from gitlab webhook configuration
* timeout - how many seconds to wait until process kill (default 10 seconds) 


### Binary run flags:
```shell
Usage of ./deployer:
  -config string
        config file location (default "config.json")
  -listen string
        addr port (default ":7654")
```

### Config manual reload

curl http://localhost:7654/reload

_Works on same machine only if 127.0.0.1 is whitelisted in config (default)_