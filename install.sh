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
      if $(uname -a | grep -c armv) = 1
      then
        echo arm
      fi
      if $(uname -a | grep -c aarch64) = 1
      then
        echo arm64
      fi
      ;;
  esac
}

echo "Arch: $(getArch)"

if [ "$(uname -o)" = "GNU/Linux" ]
then

  # curl -o- https://github.com/ramzes642/mini-deployer/releases/download/0.1.1/mini-deployer-0.1.1-linux-$(getArch).tar.gz
  cd /tmp
  curl -s https://api.github.com/repos/ramzes642/mini-deployer/releases/latest \
| grep "mini-deployer.*$(getArch).tar.gz" \
| cut -d : -f 2,3 \
| tr -d \" \
| wget -qi -
  tar -xzvf mini-deployer*.tar.gz

else
  echo Unsupported $(uname -a)
  echo Report issue to https://github.com/ramzes642/mini-deployer/issues
fi
