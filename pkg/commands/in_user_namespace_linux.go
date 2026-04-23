//go:build linux

package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const rhsmSecretsDir = "/usr/share/rhel/secrets"

// RunInUserNamespace executes a command within an externally created user
// namespace (e.g. by unshare or 'buildah unshare'). If loopbackUp is true,
// the loopback interface is brought up before executing the command.
// If disableRHSMHostIntegration is true and /usr/share/rhel/secrets exists,
// a tmpfs is mounted over it to disable RHSM host integration.
// This function does not return on success — it replaces the current process.
func RunInUserNamespace(loopbackUp bool, disableRHSMHostIntegration bool, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	if loopbackUp {
		link, err := netlink.LinkByName("lo")
		if err != nil {
			return fmt.Errorf("getting loopback interface: %w", err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("bringing up loopback interface: %w", err)
		}
	}

	if disableRHSMHostIntegration {
		if _, err := os.Stat(rhsmSecretsDir); err == nil {
			if err := unix.Mount("tmpfs", rhsmSecretsDir, "tmpfs", 0, ""); err != nil {
				return fmt.Errorf("mounting tmpfs over %s: %w", rhsmSecretsDir, err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("checking existence of %s: %w", rhsmSecretsDir, err)
		}
	}

	binary, err := exec.LookPath(args[0])
	if err != nil {
		return err
	}

	return unix.Exec(binary, args, os.Environ())
}
