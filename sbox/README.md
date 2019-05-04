```
# requires Docker 19.03 + 4.8+ kernel
docker run -it tonistiigi/sboxdemo
```

```
# in container
sbox /bin/uname -a
sbox --tty --hostnet /bin/ash
```