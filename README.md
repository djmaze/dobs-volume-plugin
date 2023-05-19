# docker-volume-dobs-driver

Docker volume driver for Digital Ocean block storage. Mounts your Digital Ocean volumes as Docker volumes in your containers.

## Installation

1. Create a **personal access token** in the Digital Ocean backend. It needs to have both `read` and `write` scope.

1. Install the volume driver on your node:

    ```bash
    docker plugin install decentralize/dobs-volume-plugin:0.2.0 TOKEN=<DIGITALOCEAN API TOKEN>
    ```

    Optionally, you can set the `API_BASE_URL` as well. (It defaults to `https://api.digitalocean.com/`.)

## Usage

1. You need to create a Digital Ocean volume manually (via the web interface or API) first. 
   1. Choose `ext4` as a filesystem.
   2. Make sure the volume is not attached to any droplet.
1. Create a docker volume:
    ```bash
    docker volume create -d decentralize/dobs-volume-plugin:0.1.0 -o name=<NAME OF DIGITALOCEAN VOLUME> -o uid=<UID TO CHOWN TO> -o gid=<GID TO CHOWN TO> <NAME OF DOCKER VOLUME>
    ```
    If the `name` option is missing, the Digital Ocean volume name is supposed to be the same as the Docker volume name.
    
    The data directory will be chown´ed to `uid`:`gid`. Both default to `0` (i.e. `root:root`).

1. Example usage:
    ```bash
    docker run -it --rm -v <NAME OF DOCKER VOLUME>:/mnt alpine
    ```

## Caveats

* The volume needs to exist and be formatted with `ext4`.
* The mount will fail if the volume is already attached to a different droplet. (This limitation is probably going to be removed, see **TODO**.)

## TODO

- [ ] Allow forcing detachment (needs testing!)
- [ ] Include fields in logging (current volume etc.)
- [ ] Make timeouts configurable
