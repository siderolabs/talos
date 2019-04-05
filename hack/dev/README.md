# Run a local talos cluster using docker

```bash
# standup HA cluster
make

# standup single-node k8s
make SERVICES=master-1

# use a specific image tag
make TAG=<image_tag>


# connect using ../../build/osctl-linux-amd64
## master-1
./osctl ps
## master-2
./osctl -t master-2 ps


# use kubectl
make kubeconfig

## apply PSP & CNI
make manifests

## get nodes
./kubectl.sh get nodes

# read init logs  (container stdout equiv. to /dev/kmsg)
docker-compose logs -f

# export all logs directly from docker  (useful if osd is down or init is broken)
sudo docker cp master-1:/var/log master-1-logs
sudo chown -R $USER master-1-logs
chmod -R +rw master-1-logs


# cleanup
make clean
```
