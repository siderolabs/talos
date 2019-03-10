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
TAG= docker-compose run --rm osctl ps
## master-2
TAG= docker-compose run --rm osctl -t 10.5.0.7 ps


# use kubectl
make kubeconfig

## apply PSP
TAG= docker-compose run --rm kubectl apply -f ./manifests/psp.yaml
## apply CNI
TAG= docker-compose run --rm kubectl apply -f ./manifests/cni.yaml


# read init logs  (container stdout equiv. to /dev/kmsg)
docker-compose logs -f

# export all logs directly from docker  (useful if osd is down or init is broken)
sudo docker cp master-1:/var/log master-1-logs
sudo chown -R $USER master-1-logs
chmod -R +rw master-1-logs


# cleanup
make clean
```