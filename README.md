# kindle-mdns-rcvr

**Connect to your kindle with just `kindle.local` No IPs, no usbNet!**

An mDNS responder for a jailbroken Kindle, written in Go with no dependencies
outside the standard library.

To ssh into a jailbroken kindle, we need its IP or usbNet. But the IP can change. We need a way to connect our kindle with a domain such as `kindle.local`. This receiver solves that problem.  

You have to look up the IP on your router every time you want to
`ssh` in. You can set a static IP on your router but not everyone has access to their router. Besides where is the fun in that! 

Kindles don't run the standard mdns libraries such as Avahi or Bonjour. This binary runs on the Kindle, listens on the mDNS multicast group,
and answers any query for `kindle.local` with the device's current IPv4 address.
After that, `ssh root@kindle.local` just works.

## Building

You need a Go toolchain on your dev env, not on the Kindle. Cross
compile a static ARM binary:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5 go build -o kindle-mdns-rcvr .
```

- `CGO_ENABLED=0` produces a fully static binary. Without it the Go runtime
  links against the host's libc for DNS and user lookups, and the Kindle's
  glibc is far too old to satisfy it.
- `GOARM=5` targets soft-float ARMv5, the safe floor across Kindle models. Look
  up your own model and adjust (run `uname -m` on your kindle to find out): a binary built for a higher ARM version will
  fail to start on older hardware.

If you manage Go through gvm (because the newer go versions do not support the kindle's linux versions), point at that toolchain explicitly:

```sh
GOROOT=~/.gvm/go CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5 \
  ~/.gvm/go/bin/go build -o kindle-mdns-rcvr .
```
I recommend Go16 for Kindle pw4. You can choose a version by looking up the kindle's linux version and checking if your Go version supports that version.

Then copy it over and run it (you'll need the IP the first time and every time you reboot):

##Running
```sh
scp kindle-mdns-rcvr root@<kindle-ip>:/mnt/us/
ssh root@<kindle-ip>  

cd /mnt/us/
chmod +x kindle-mdns-rcvr
./kindle-mdns-rcvr
```
Voila! you are ready to ssh or find your kindle's IP with: 
```
ssh root@kindle.local 
ping kindle.local
```



##Make it a daemon
1. make your kindle writable - `mntroot rw`
2. move binary - `mv /mnt/usr/kindle-mdns-rcvr /usr/local/bin`
3. Make it a daemon with kindle's start-stop 
```
/mnt/us/usbnet/bin/busybox start-stop-daemon -S -b -m \

  -p /var/run/kindle-mdns.pid \

  -x /usr/local/bin/kindle-mdns-rcvr
```
4. To kill, ps ax. Find pid. then run `kill pid`

##Release
[Here's](https://github.com/sindhusp/kindle-mdns-rcvr/blob/master/kindle-mdns-rcvr) a binary built for my jailbroken kindle PW4 running 4.1.4 firmware. If you are on the same device, download it and move it over to your kindle. Follow the instructions [here](#Running)

## Gotchas


- **The interface name is hardcoded to `wlan0`.** Run `ifconfig` on your Kindle
  first; if the wifi interface is called wlan0, you can run this code as is. Otherwise, if the WiFi interface has a different name, you'll need to edit
  `main.go` and rebuild.
- **The name is hardcoded to `kindle.local`.**
- Unicast queries are ignored for now. Also this is an mdns receiver, it doesn't announce itself unless queried.

## Extra
- A note on integration: It's tricky for android browsers to connect to this domain `kindle.local` if you are running a server on your kindle as .local mdns resolution was recently introduced natively to android. It's straightforward on ios, go to your Safari browser and type http://kindle.local:<port>/ to connect

