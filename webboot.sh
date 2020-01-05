#!/bin/sh

# just a workaround for now :D
echo "#!/bin/sh" > /bin/wifi
chmod 777 /bin/wifi

dhclient -ipv6=false eth0
wget http://10.0.2.2:8080/bzImage
webboot -interface eth0 Tinycore
