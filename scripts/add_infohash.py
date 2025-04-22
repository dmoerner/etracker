# add_infohash.py
#
# Helper function to calculate the infohash from a torrent file and add it
# to a running copy of etracker through the admin API.

import argparse
import base64
import bencoder
import hashlib
import os
import requests


def parse_torrent(filename):
    with open(filename, "rb") as f:
        bencoded = bencoder.decode(f.read())
        if not isinstance(bencoded, dict):
            raise ValueError("invalid torrent file")
        info_dict = bencoded[b"info"]
        info_hash = base64.b64encode(
            hashlib.sha1(bencoder.encode(info_dict)).digest()
        ).decode("utf-8")
        name = info_dict[b"name"].decode("utf-8")
        return info_hash, name


def post_infohash(hostname, apikey, info_hash, name):
    url = f"{hostname}/api/infohash"
    body = {"info_hash": info_hash, "name": name}
    verify = True
    if "localhost" in hostname or "127.0.0.1" in hostname:
        verify = False
    r = requests.post(url, headers={"Authorization": apikey}, json=body, verify=verify)
    return r

def post_torrent(hostname, apikey, filename):
    headers={"Authorization": apikey}
    url = f"{hostname}/api/torrentfile"
    with open(filename, "rb") as f:
        files = {
            'file': (os.path.basename(filename), f, 'application/x-bittorrent')
        }
        verify = True
        if "localhost" in hostname or "127.0.0.1" in hostname:
            verify = False
        r = requests.post(url, headers=headers, files=files, verify=verify)
        return r


def main():
    parser = argparse.ArgumentParser(
        prog="add_infohash.py",
        description="calculate torrent infohash and add to etracker allowlist",
    )

    parser.add_argument("hostname", help="etracker hostname")
    parser.add_argument("apikey", help="etracker api key")
    parser.add_argument("torrentfile", help="path to torrent file")

    args = parser.parse_args()

    # info_hash, name = parse_torrent(args.torrentfile)

    # result = post_infohash(args.hostname, args.apikey, info_hash, name)
    result = post_torrent(args.hostname, args.apikey, args.torrentfile)
    print(f"{result.status_code}, {result.json()}")


if __name__ == "__main__":
    main()
