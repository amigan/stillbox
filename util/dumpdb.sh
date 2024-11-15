#!/bin/sh

config=config.yaml
pgformat="-Fc"
ext=pgdump

while getopts ":p" arg; do
  case $arg in
  c)
    config=$OPTARG
    ;;
  p)
    pgformat="-Fp"
    ext=sql
    ;;
  esac
done

filen=`date "+backups/%Y%m%d_%H%M%S.${ext}"`

mkdir -p backups/
dbstring=`yq -r .db.connect "${config}"`
pg_dump "${pgformat}" -f "${filen}" -T calls "${dbstring}"
echo "backed up to ${filen}"
