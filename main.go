// This code is available on the terms of the project LICENSE.md file,
// also available online at https://blueoakcouncil.org/license/1.0.0.

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
)

func installurl() string {
	url := "https://cdn.openbsd.org/pub/OpenBSD"
	if f, err := os.Open("/etc/installurl"); err == nil {
		url, err = bufio.NewReader(f).ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		url = url[:len(url)-1] // remove newline
	}
	return url
}

func machine() string {
	cmd := exec.Command("uname", "-m")
	output, err := cmd.Output()
	if err != nil {
		return "amd64"
	}
	return string(output[:len(output)-1]) // remove newline
}

var (
	mirror = flag.String("mirror", installurl(), "snapshot mirror")
	arch   = flag.String("arch", machine(), "CPU architecture")
	rel    = flag.Int("release", 66, "OpenBSD release")
	dir    = flag.String("d", "/home/_sysupgrade", "download directory")
	pubkey = flag.String("p", "", "signify pubkey file")
)

var fetch = []string{
	"SHA256.sig",
	"INSTALL.ARCH",
	"baseXX.tgz",
	"bsd",
	"bsd.mp",
	"bsd.rd",
	"compXX.tgz",
	"gameXX.tgz",
	"manXX.tgz",
	"xbaseXX.tgz",
	"xfontXX.tgz",
	"xservXX.tgz",
	"xshareXX.tgz",
}

var verify = fetch[1:]

func main() {
	flag.Parse()

	rel := strconv.Itoa(*rel)
	arch := *arch
	for i := range fetch {
		fetch[i] = strings.Replace(fetch[i], "XX", rel, 1)
		fetch[i] = strings.Replace(fetch[i], "ARCH", arch, 1)
	}

	mirror := fmt.Sprintf("%s/snapshots/%s/", *mirror, arch)

	log.Printf("Downloading latest snapshot from %v to %v", mirror, *dir)
	ctx := context.Background()
	g, ctx := errgroup.WithContext(ctx)
	for i := range fetch {
		file := fetch[i]
		url := mirror + file
		out, err := os.Create(filepath.Join(*dir, file))
		if err != nil {
			log.Fatal(err)
		}
		r, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Fatal(err)
		}
		r = r.WithContext(ctx)
		g.Go(func() error {
			resp, err := http.DefaultClient.Do(r)
			if resp != nil {
				defer log.Printf("GET %s (%d)", file, resp.StatusCode)
			}
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			_, err = io.Copy(out, resp.Body)
			return err
		})
	}
	err := g.Wait()
	if err != nil {
		log.Fatal(err)
	}

	pubkey := *pubkey
	if pubkey == "" {
		pubkey = "/etc/signify/openbsd-" + rel + "-base.pub"
	}
	args := []string{"-C", "-p", pubkey, "-x", "SHA256.sig"}
	args = append(args, verify...)
	log.Println("Verfiying")
	cmd := exec.Command("signify", args...)
	cmd.Dir = *dir
	output, err := cmd.CombinedOutput()
	os.Stderr.Write(output)
	if err != nil {
		log.Fatal(err)
	}
}
