#!/bin/bash

function getArch() {
  case $(uname -i) in
    x86_64|amd64)
      echo amd64;;
    i?86)
      echo "386";;
    arm*)
      echo arm;;
    powerpc|ppc64)
      echo PowerPC;;
    aarch64)
      echo arm64;;
    unknown)
      if [ $(uname -a | grep -c armv) = 1 ]
      then
        echo arm
      fi
      if [ $(uname -a | grep -c aarch64) = 1 ]
      then
        echo arm64
      fi
      ;;
  esac
}

if [ $(ps -p 1 | grep -c systemd) != 1 ]
then
  echo "ERROR: Only systemd linux systems are supported, sorry"
  exit
fi

if [ "$(uname -o)" = "GNU/Linux" ]
then

  mkdir /tmp/install-deployer
  cd /tmp/install-deployer || exit

  curl -s https://api.github.com/repos/ramzes642/mini-deployer/releases/latest \
| grep "mini-deployer.*$(getArch).tar.gz" \
| cut -d : -f 2,3 \
| tr -d \" \
| wget -qi -
  tar -xzf mini-deployer*.tar.gz

  # shellcheck disable=SC2181
  if [ $? != 0 ]
  then
    echo "ERROR: Download failed, it seems that your arch '$(getArch)' is not supported"
    echo "If you think that is an error, please leave an issue with your uname -a:"
    echo "uname -a: " $(uname -a)
    echo "uname -i: " $(uname -i)
    rm -rf /tmp/install-deployer
    exit
  fi

  mv /tmp/install-deployer/config.sample.json /etc/mini-deployer.json
  mv /tmp/install-deployer/deployer.service /etc/systemd/system/mini-deployer.service
  if [ -x /etc/systemd/system/mini-deployer.service ]
  then
    systemctl stop mini-deployer.service
  fi
  mv /tmp/install-deployer/mini-deployer /usr/bin/mini-deployer

  systemctl enable mini-deployer.service
  systemctl start mini-deployer.service

  rm -rf /tmp/install-deployer

  echo Mini-deployer succecefully installed
  echo Run \# journalctl -fu mini-deployer to inspect logs
  echo Edit /etc/mini-deployer.json to modify deployment hooks


else
  echo Unsupported $(uname -a)
  echo Report issue to https://github.com/ramzes642/mini-deployer/issues
fi
