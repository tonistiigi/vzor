# sbox

## Try it out

```
# requires Docker 19.03 + 4.8+ kernel
docker run -it tonistiigi/sboxdemo
```

```
# in container
sbox /bin/uname -a
sbox --tty --hostnet /bin/ash
```

## Building

Run from the root of the repository

```
$ docker build -t sboxdemo -f cmd/sbox/Dockerfile .
```
