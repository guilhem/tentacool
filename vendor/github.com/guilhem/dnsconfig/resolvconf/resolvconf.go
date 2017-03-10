package resolvconf

import (
  "os"
  "os/exec"

  "github.com/guilhem/dnsconfig"
)

const ResolvPath = "/etc/resolvconf/resolv.conf.d/base"

func IsResolvconf() bool {
  fi, err := os.Lstat(dnsconfig.ResolvPath)
  if err != nil {
    return false
  }

  hasResolvconf := false
  if _, err := exec.LookPath("resolvconf"); err == nil {
    cmd := exec.Command("resolvconf", "--updates-are-enabled")
    err := cmd.Run()
    hasResolvconf = err == nil
  }
  return hasResolvconf && fi.Mode().IsRegular() == false
}
