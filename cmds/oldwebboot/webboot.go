// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Synopsis:
//     webboot [OPTIONS...] name of bookmark
//
// Options:
//	-cmd: Command line parameters to the second kernel
//	-ifName: Name of the interface
//	-timeout: Lease timeout in seconds
//	-retry: Number of DHCP renewals before exiting
//	-verbose:  Verbose output
//	-ipv4: Use IPV4
//	-ipv6: Use IPV6
//	-dryrun: Do not do the kexec
//	-wifi: [essid [WPA [password]]]
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/u-root/u-root/pkg/boot"
	"github.com/u-root/u-root/pkg/boot/kexec"
	"github.com/u-root/u-root/pkg/mount"
	"github.com/u-root/u-root/pkg/uio"
	"github.com/u-root/webboot/pkg/dhclient"
	"golang.org/x/sys/unix"
)

const (
	wbtcpURL    = "https://github.com/u-root/webboot-distro/raw/master/iso/tinycore/10.x/x86_64/release/TinyCorePure64.iso"
	wbcpURL     = "https://github.com/u-root/webboot-distro/raw/master/iso/tinycore/10.x/x86_64/release/CorePure64.iso"
	tcURL       = "http://tinycorelinux.net/10.x/x86_64/release/TinyCorePure64-10.1.iso"
	coreURL     = "http://tinycorelinux.net/10.x/x86/release/CorePlus-current.iso"
	ubuURL      = "http://releases.ubuntu.com/18.04/ubuntu-18.04.3-desktop-amd64.iso"
	fedoraURL   = "https://download.fedoraproject.org/pub/fedora/linux/releases/32/Spins/x86_64/iso/Fedora-KDE-Live-x86_64-32-1.6.iso"
	oSUSEURL    = "http://orangetux:8080/openSUSE-Leap-15.2-KDE-Live-x86_64-Build31.2-Media.iso"
	archURL     = "http://10.0.2.2:8080/archlinux.iso"
	srcdURL     = "http://10.0.2.2:8080/systemrescuecd.iso"
	manjURL     = "http://10.0.2.2:8080/manjaro.iso"
	debURL      = "http://10.0.2.2:8080/debian.iso"
	tcCmdLine   = "cde"
	slitazURL   = "http://10.0.2.2:8080/slitaz-rolling-core-5in1.iso"
	slitaz64URL = "http://10.0.2.2:8080/slitaz-rolling-core64.iso"
	// slitazURL = "http://mirror.slitaz.org/iso/rolling/slitaz-rolling-core64.iso"
)

//Distro defines an operating system distribution
type Distro struct {
	Kernel  string
	Initrd  string
	Cmdline string
	URL     string
}

var (
	cmd      = flag.String("cmd", "", "Command line parameters to the second kernel")
	ifName   = flag.String("interface", "^[we].*", "Name of the interface")
	timeout  = flag.Int("timeout", 15, "Lease timeout in seconds")
	retry    = flag.Int("retry", 5, "Max number of attempts for DHCP clients to send requests. -1 means infinity")
	verbose  = flag.Bool("verbose", false, "Verbose output")
	ipv4     = flag.Bool("ipv4", true, "use IPV4")
	ipv6     = flag.Bool("ipv6", true, "use IPV6")
	dryrun   = flag.Bool("dryrun", false, "Do not do the kexec")
	wifi     = flag.String("wifi", "", "[essid [WPA [password]]]")
	bookmark = map[string]*Distro{
		// TODO: Fix webboot to process the tinycore's kernel and initrd to boot from instead of using our customized kernel
		"webboot-tinycorepure": &Distro{
			"boot/vmlinuz64",
			"boot/corepure64.gz",
			tcCmdLine,
			wbtcpURL,
		},
		"webboot-corepure": &Distro{
			"boot/vmlinuz64",
			"boot/corepure64.gz",
			tcCmdLine,
			wbcpURL,
		},
		"tinycore": &Distro{
			"boot/vmlinuz64",
			"boot/corepure64.gz",
			tcCmdLine,
			tcURL,
		},
		"Tinycore": &Distro{
			"/bzImage", // our own custom kernel, which has to be in the initramfs
			"boot/corepure64.gz",
			tcCmdLine,
			tcURL,
		},
		"slitaz": &Distro{
			"boot/bzImage64",
			"boot/rootfs5.gz,boot/rootfs4.gz,boot/rootfs3.gz,boot/rootfs2.gz,boot/rootfs1.gz",
			// "boot/rootfs1.gz64",
			// "video=-32 autologin",
			// "rw root=0x100 autologin initrd=/boot/rootfs5.gz initrd=/boot/rootfs4.gz initrd=/boot/rootfs3.gz initrd=/boot/rootfs2.gz initrd=/boot/rootfs1.gz64",
			"rw root=0x100 autologin initrd=/boot/rootfs5.gz,/boot/rootfs4.gz,/boot/rootfs3.gz,/boot/rootfs2.gz,/boot/rootfs1.gz",
			slitazURL,
		},
		"slitaz-base": &Distro{
			"boot/bzImage64",
			"boot/rootfs5.gz,boot/rootfs1.gz",
			"autologin initrd=/boot/rootfs5.gz initrd=/boot/rootfs1.gz",
			slitazURL,
		},
		"slitaz-zero": &Distro{
			"boot/bzImage64",
			"boot/rootfs5.gz",
			"autologin", // initrd=/boot/rootfs5.gz",
			slitazURL,
		},
		"slitaz64": &Distro{
			"boot/bzImage64",
			"boot/rootfs.gz",
			"autologin",
			slitaz64URL,
		},
		"arch": &Distro{
			"arch/boot/x86_64/vmlinuz",
			"arch/boot/intel_ucode.img,arch/boot/x86_64/archiso.img",
			// TODO: find an alternative for maintaining the label?
			"earlyprintk=ttyS0,115200 console=tty0 loglevel=3 archisobasedir=arch archisolabel=ARCH_202005",
			archURL,
		},
		"sysresccd": &Distro{
			"sysresccd/boot/x86_64/vmlinuz",
			"sysresccd/boot/intel_ucode.img,sysresccd/boot/x86_64/sysresccd.img",
			"earlyprintk=ttyS0,115200 console=tty0 loglevel=3 archisobasedir=sysresccd archisolabel=SYSRESCCD_WEBBOOT",
			srcdURL,
		},
		"manjaro": &Distro{
			"boot/vmlinuz-x86_64",
			"boot/initramfs-x86_64.img",
			"earlyprintk=ttyS0,115200 console=tty0 loglevel=3 misobasedir=manjaro misolabel=MANJARO_WEBBOOT",
			manjURL,
		},
		"fedora": &Distro{
			"isolinux/vmlinuz",
			"isolinux/initrd.img",
			"root=live:CDLABEL=Fedora-KDE-Live-32-1-6 rd.live.image",
			fedoraURL,
		},
		"opensuse": &Distro{
			"boot/x86_64/loader/linux",
			"boot/x86_64/loader/initrd",
			"root=live:CDLABEL=openSUSE_Leap_15.2_KDE_Live rd.live.image rd.live.overlay.persistent rd.live.overlay.cowfs=ext4",
			oSUSEURL,
		},
		"debian": &Distro{
			"live/vmlinuz-4.19.0-9-amd64",
			"live/initrd.img-4.19.0-9-amd64",
			"boot=live components", // splash quiet",
			debURL,
		},
		"ubuntu": &Distro{
			"casper/vmlinuz",
			"casper/initrd",
			"earlyprintk=ttyS0,115200 console=tty0 root=/dev/pmem0 loglevel=3 boot=casper file=/cdrom/preseed/ubuntu.seed",
			ubuURL,
		},
		"local": &Distro{
			"/bzImage",
			"boot/corepure64.gz",
			tcCmdLine,
			"file:///iso", // NOTE: three / is REQUIRED
		},
		"core": &Distro{
			"boot/vmlinuz",
			"boot/core.gz",
			tcCmdLine,
			coreURL,
		},
	}
)

// linkOpen returns an io.ReadCloser that holds the content of the URL
func linkOpen(URL string) (io.ReadCloser, error) {
	switch {
	case strings.HasPrefix(URL, "file://"):
		return os.Open(URL[7:])
	case strings.HasPrefix(URL, "http://"), strings.HasPrefix(URL, "https://"):
		resp, err := http.Get(URL)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP Get failed: %v", resp.StatusCode)
		}
		return resp.Body, nil
	}
	return nil, fmt.Errorf("%q: linkopen only supports file://, https://, and http:// schemes", URL)
}

// setupWIFI enables connection to a specified wifi network
// wifi can be an open or closed network
func setupWIFI(wifi string) error {
	if wifi == "" {
		return nil
	}

	c := exec.Command("wifi", strings.Split(wifi, " ")...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	// wifi and its children can run a long time. The bigger problem is
	// knowing when the net is ready, but one problem at a time.
	if err := c.Start(); err != nil {
		return fmt.Errorf("Error starting wifi(%v):%v", wifi, err)
	}
	return nil
}

func usage() {
	log.Printf("Usage: %s [flags] URL or name of bookmark\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(1)
}

func wbpath(dir, file string) string {
	if filepath.IsAbs(file) {
		return file
	}

	return filepath.Join(dir, file)
}

func main() {
	flag.Parse()

	cl, err := ioutil.ReadFile("/proc/cmdline")

	if err != nil {
		log.Fatal(err)
	}

	if flag.NArg() != 1 {
		usage()
	}
	b, ok := bookmark[flag.Arg(0)]
	if !ok {
		var s string
		for os := range bookmark {
			s += os + " "
		}
		log.Fatalf("%v, valid names: %q", err, s)
	}

	// TODO: Find a persistent memory device large enough to store the content. If no blocks are available, error the user.
	pmem, err := os.OpenFile("/dev/pmem0", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}

	if *wifi != "" {
		if err := setupWIFI(*wifi); err != nil {
			log.Fatal(err)
		}
	}
	//the Request function sets up a DHCP confifuration for all interfaces,
	//such as eth0, which is an ethernet interface.
	if *ipv4 || *ipv6 {
		//ifname uses the regular expression ^[we].* to check for an interface starting with w or e such as
		//wlan0/1, enx453243, or eth0/1
		dhclient.Request(*ifName, *timeout, *retry, *verbose, *ipv4, *ipv6)

	}

	// Processes the URL to receive an io.ReadCloser, which holds the content of the downloaded file
	log.Println("Retrieving the file...")
	iso, err := linkOpen(b.URL)
	if err != nil {
		log.Fatal(err)
	}
	defer iso.Close()

	if _, err := io.Copy(pmem, iso); err != nil {
		log.Fatalf("Error copying to persistent memory device: %v", err)
	}
	if err = pmem.Close(); err != nil {
		log.Fatalf("Error closing /dev/pmem0: %v", err)
	}

	tmp, err := ioutil.TempDir("", "mnt")
	if err != nil {
		log.Fatal(err)
	}

	if _, err := mount.Mount("/dev/pmem0", tmp, "iso9660", "", unix.MS_RDONLY|unix.MS_NOATIME); err != nil {
		log.Fatal(err)

	}
	if *dryrun == false {
		k := wbpath(tmp, b.Kernel)
		initrds := strings.Split(b.Initrd, ",")
		var initrdReaders []io.ReaderAt
		for _, relPath := range initrds {
			file, err := os.Open(wbpath(tmp, relPath))
			if err != nil {
				log.Fatal(err)
			}
			initrdReaders = append(initrdReaders, file)
		}
		// initrd := PadInitrds(initrdReaders)
		initrd := boot.CatInitrds(initrdReaders...)

		cmdline := strings.TrimSuffix(string(cl), "\n") + " " + b.Cmdline
		image := &boot.LinuxImage{
			Kernel:  uio.NewLazyFile(k),
			Initrd:  initrd,
			Cmdline: cmdline,
		}
		if err := image.Load(true); err != nil {
			log.Fatalf("error failed to kexec into new kernel:%v", err)
		}
		if err := kexec.Reboot(); err != nil {
			log.Fatalf("error failed to Reboot into new kernel:%v", err)
		}
	}

	fmt.Printf("URL: %v\n Distro: %v\nmount point: %v\n", b.URL, b, tmp)
}
