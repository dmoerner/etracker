# add_infohash.py
#
# Helper function to calculate the infohash from a torrent file and add it
# to a running copy of etracker through the admin API.

import argparse
import requests
import bencoder
import hashlib


def calculate_infohash(filename):
    with open(filename, "rb") as f:
        bencoded = bencoder.decode(f.read())
        info_dict = bencoded[b"info"]
        return hashlib.sha1(bencoder.encode(info_dict)).hexdigest()


def add_api(address, apikey, infohash, name):
    url = f"{address}/api?action=insert_infohash&info_hash={infohash}&name={name}"
    print(url)
    r = requests.get(url, headers={"Authorization": apikey})
    return r.status_code


def main():
    parser = argparse.ArgumentParser(
        prog="add_infohash.py",
        description="calculate torrent infohash and add to etracker allowlist",
    )

    parser.add_argument("address", help="etracker address")
    parser.add_argument("apikey", help="etracker api key")
    parser.add_argument("torrentfile", help="path to torrent file")
    parser.add_argument(
        "name", help="infohash name, should be base path of data in most cases"
    )

    args = parser.parse_args()

    infohash = calculate_infohash(args.torrentfile)
    print(infohash)

    result = add_api(args.address, args.apikey, infohash, args.name)
    print(result)


if __name__ == "__main__":
    main()
