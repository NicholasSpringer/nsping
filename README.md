# nsping
nsping is a simple ping application written in Go. The repository 
includes both a CLI application and a Go package that can be used
to collect network information using ICMP echo requests.

CLI usage (must be run as super-user): `sudo ns-ping [-i interval (ms)] [-m ttl] host`