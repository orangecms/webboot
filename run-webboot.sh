#!/bin/sh

# https://docs.pmem.io/persistent-memory/getting-started-guide/creating-development-environments/virtualization/qemu
export PMEM_SIZE=1G
export RAM_SIZE=2G
export KERNEL=bzImage

qemu-system-x86_64 \
  -m $RAM_SIZE \
  -object rng-random,filename=/dev/urandom,id=rng0 \
  -device virtio-rng-pci,rng=rng0 \
  -netdev user,id=u1 -device e1000,netdev=u1 \
  -kernel $KERNEL \
  -initrd initramfs.linux_amd64_webboot.cpio \
  -serial stdio \
  -append "console=ttyS0 vga=786 memmap=$PMEM_SIZE!$PMEM_SIZE"
