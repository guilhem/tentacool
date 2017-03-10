// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

// Read system DNS config from /etc/resolv.conf

package dnsconfig

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const ResolvPath = "/etc/resolv.conf"

type DnsConfig struct {
	Servers  []string `json:"servers"` // servers to use
	Search   []string `json:"search"`  // suffixes to append to local name
	Ndots    int      `json:"ndots"`   // number of dots in name to trigger absolute lookup
	Timeout  int      `json:"timeout"` // seconds before giving up on packet
	Attempts int      `json:"attemps"` // lost packets before giving up on server
	Rotate   bool     `json:"rotate"`  // round robin among servers
}

func DnsReadConfig(filename string) (*DnsConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	conf := new(DnsConfig)

	for scanner.Scan() {
		line := scanner.Text()
		scannerLine := bufio.NewScanner(strings.NewReader(line))
		scannerLine.Split(bufio.ScanWords)
		var lineArr []string
		for scannerLine.Scan() {
			lineArr = append(lineArr, scannerLine.Text())
		}

		//empty line
		if len(lineArr) == 0 {
			continue
		}
		switch lineArr[0] {
		case "nameserver": // add one name server
			if len(lineArr) > 1 {
				conf.Servers = append(conf.Servers, lineArr[1])
			}

		case "domain": // set search path to just this domain
			if len(lineArr) > 1 {
				conf.Search = make([]string, 1)
				conf.Search[0] = lineArr[1]
			} else {
				conf.Search = make([]string, 0)
			}

		case "search": // set search path to given servers
			conf.Search = make([]string, len(lineArr)-1)
			for i := 0; i < len(conf.Search); i++ {
				conf.Search[i] = lineArr[i+1]
			}

		case "options": // magic options
			for i := 1; i < len(lineArr); i++ {
				s := lineArr[i]
				switch {
				case strings.HasPrefix(s, "ndots:"):
					v := strings.TrimPrefix(s, "ndots:")
					conf.Ndots, _ = strconv.Atoi(v)
				case strings.HasPrefix(s, "timeout:"):
					v := strings.TrimPrefix(s, "timeout:")
					conf.Timeout, _ = strconv.Atoi(v)
				case strings.HasPrefix(s, "attempts:"):
					v := strings.TrimPrefix(s, "attempts:")
					conf.Attempts, _ = strconv.Atoi(v)
				case s == "rotate":
					conf.Rotate = true
				}
			}
		}
	}
	return conf, nil
}

func DnsWriteConfig(conf *DnsConfig, filename string) (err error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	for _, server := range conf.Servers {
		line := "nameserver " + server
		fmt.Fprintln(w, line)
	}
	for _, s := range conf.Search {
		line := "search " + s
		fmt.Fprintln(w, line)
	}
	if conf.Ndots != 0 || conf.Timeout != 0 || conf.Attempts != 0 || conf.Rotate != false {
		line := "options"
		if conf.Ndots != 0 {
			line += " ndots:" + strconv.Itoa(conf.Ndots)
		}
		if conf.Timeout != 0 {
			line += " timeout:" + strconv.Itoa(conf.Timeout)
		}
		if conf.Attempts != 0 {
			line += " attempts:" + strconv.Itoa(conf.Attempts)
		}
		if conf.Rotate == true {
			line += " rotate"
		}
		fmt.Fprintln(w, line)
	}
	w.Flush()

	return
}
